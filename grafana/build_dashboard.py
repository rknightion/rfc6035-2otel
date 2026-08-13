#!/usr/bin/env python3
"""Generate the RFC 6035 operational Dashboard v2 resource."""

from __future__ import annotations

import argparse

from common import ROOT, catalog, dump_json, write_or_check
from panels import PANEL_REGISTRY

OUT = ROOT / "dashboards" / "rfc6035-2otel.json"
SIGNALS_DOC = ROOT / "docs" / "signals.md"
SIGNALS_START = "<!-- BEGIN GENERATED SIGNAL CATALOG -->"
SIGNALS_END = "<!-- END GENERATED SIGNAL CATALOG -->"

PROMETHEUS = "${ds_prometheus}"
LOKI = "${ds_loki}"
QUALITY_LABELS = "service_instance_id, rfc6035_sender_name, rfc6035_report_side"
QUALITY_FILTER = 'service_instance_id=~"$site", rfc6035_sender_name=~"$sender"'
TREND_WINDOW = "24h"
RAW_REPORTS_URL = (
    "/d/rfc6035-2otel?dtab=Raw-Reports&${__url_time_range}"
    "&var-site=${__field.labels.service_instance_id}"
    "&var-sender=${__field.labels.rfc6035_sender_name}"
    "&${ds_prometheus:queryparam}&${ds_loki:queryparam}"
)
RAW_REPORTS_FALLBACK_URL = (
    "/d/rfc6035-2otel?dtab=Raw-Reports&${__url_time_range}"
    "&${site:queryparam}&${sender:queryparam}"
    "&${ds_prometheus:queryparam}&${ds_loki:queryparam}"
)


def metric_by_name(metrics: list[dict]) -> dict[str, dict]:
    return {metric["name"]: metric for metric in metrics}


def prometheus_label(attribute: str) -> str:
    return attribute.replace(".", "_")


def render_signal_catalog() -> str:
    lines = [
        "| OTLP metric | Prometheus name | Kind | Unit | Attributes |",
        "| --- | --- | --- | --- | --- |",
    ]
    for metric in catalog()["metrics"]:
        attributes = ", ".join(f"`{name}`" for name in metric["attributes"]) or "none"
        lines.append(
            f"| `{metric['name']}` | `{metric['prometheus']}` | "
            f"{metric['kind']} | `{metric['unit']}` | {attributes} |"
        )
    return "\n".join(lines)


def render_signals_doc() -> str:
    current = SIGNALS_DOC.read_text(encoding="utf-8")
    before, separator, remainder = current.partition(SIGNALS_START)
    if not separator:
        raise SystemExit(f"missing generated marker: {SIGNALS_START}")
    _, separator, after = remainder.partition(SIGNALS_END)
    if not separator:
        raise SystemExit(f"missing generated marker: {SIGNALS_END}")
    return before + SIGNALS_START + "\n\n" + render_signal_catalog() + "\n\n" + SIGNALS_END + after


def datasource_variable(name: str, label: str, plugin_id: str, default: str) -> dict:
    return {
        "kind": "DatasourceVariable",
        "spec": {
            "name": name, "label": label, "pluginId": plugin_id,
            "current": {"text": default, "value": default}, "options": [],
            "multi": False, "includeAll": False, "allowCustomValue": True,
            "hide": "dontHide", "refresh": "onDashboardLoad", "regex": "", "skipUrlSync": False,
        },
    }


def query_variable(name: str, label: str, query: str, *, hidden: bool = False) -> dict:
    return {
        "kind": "QueryVariable",
        "spec": {
            "name": name, "label": label, "hide": "hideVariable" if hidden else "dontHide",
            "query": {
                "kind": "DataQuery", "version": "v0", "group": "",
                "datasource": {"name": PROMETHEUS},
                "spec": {"query": query, "refId": name},
            },
            "current": {"text": "", "value": ""} if hidden else {"text": "All", "value": "$__all"},
            "options": [], "multi": not hidden, "includeAll": not hidden,
            "allowCustomValue": False if hidden else True,
            "refresh": "onTimeRangeChanged", "regex": "", "skipUrlSync": hidden,
            "sort": "disabled" if hidden else "alphabeticalAsc",
            **({} if hidden else {"allValue": ".*"}),
        },
    }


def data_query(datasource: str, expr: str, *, ref_id: str, instant: bool = False, fmt: str = "time_series") -> dict:
    return {
        "kind": "DataQuery", "version": "v0", "group": "loki" if datasource == LOKI else "",
        "datasource": {"name": datasource},
        "spec": {
            "expr": expr, "refId": ref_id, "instant": instant, "range": not instant,
            "legendFormat": "", "format": fmt,
        },
    }


def panel_query(datasource: str, expr: str, *, ref_id: str = "A", instant: bool = False, fmt: str = "time_series") -> dict:
    return {
        "kind": "PanelQuery",
        "spec": {
            "refId": ref_id,
            "hidden": False,
            "datasource": {"uid": datasource},
            "query": data_query(datasource, expr, ref_id=ref_id, instant=instant, fmt=fmt),
        },
    }


def raw_link() -> dict:
    return {"title": "Open raw reports", "url": RAW_REPORTS_FALLBACK_URL, "targetBlank": False}


def panel(key: str, queries: list[dict], *, viz: str = "timeseries", unit: str = "short", description: str = "", field_link: bool = False) -> tuple[str, dict]:
    panel_id, title, _ = PANEL_REGISTRY[key]
    defaults: dict = {"unit": unit}
    if field_link:
        defaults["links"] = [{"title": "Open matching raw reports", "url": RAW_REPORTS_URL, "targetBlank": False}]
    return (
        f"panel-{panel_id}",
        {
            "kind": "Panel",
            "spec": {
                "id": panel_id, "title": title, "description": description, "links": [raw_link()] if field_link else [],
                "data": {"kind": "QueryGroup", "spec": {"queries": queries, "queryOptions": {}, "transformations": []}},
                "vizConfig": {
                    "kind": "VizConfig", "group": viz, "version": "12.1.0",
                    "spec": {"options": {}, "fieldConfig": {"defaults": defaults, "overrides": []}},
                },
            },
        },
    )


def quality_p95(metric: str, extra_labels: str = "", *, window: str = "$__range") -> str:
    labels = f"{QUALITY_LABELS}{', ' + extra_labels if extra_labels else ''}"
    return (
        f"histogram_quantile(0.95, sum by (le, {labels}) "
        f"(increase({metric}_bucket{{{QUALITY_FILTER}}}[{window}]))) "
        f"and on ({labels}) sum by ({labels}) (increase({metric}_count{{{QUALITY_FILTER}}}[{window}])) > 0"
    )


def quality_mean(metric: str, extra_labels: str = "", *, window: str = "$__range") -> str:
    labels = f"{QUALITY_LABELS}{', ' + extra_labels if extra_labels else ''}"
    return (
        f"(sum by ({labels}) (increase({metric}_sum{{{QUALITY_FILTER}}}[{window}])) / "
        f"sum by ({labels}) (increase({metric}_count{{{QUALITY_FILTER}}}[{window}]))) "
        f"and on ({labels}) sum by ({labels}) (increase({metric}_count{{{QUALITY_FILTER}}}[{window}])) > 0"
    )


def quality_trend_p95(metric: str, extra_labels: str = "") -> str:
    return quality_p95(metric, extra_labels, window=TREND_WINDOW)


def quality_trend_mean(metric: str, extra_labels: str = "") -> str:
    return quality_mean(metric, extra_labels, window=TREND_WINDOW)


def selection_filter(metric: dict) -> str:
    filters = ['service_instance_id=~"$site"']
    if "rfc6035.sender.name" in metric["attributes"]:
        filters.append('rfc6035_sender_name=~"$sender"')
    return ", ".join(filters)


def metric_expr(metric: dict) -> str:
    name = metric["prometheus"]
    labels = ", ".join(prometheus_label(attribute) for attribute in metric["attributes"])
    selector = selection_filter(metric)
    if metric["kind"] == "histogram":
        return f"histogram_quantile(0.95, sum by (le{', ' if labels else ''}{labels}) (rate({name}_bucket{{{selector}}}[5m])))"
    if metric["kind"] == "counter":
        return f"sum by ({labels}) (rate({name}{{{selector}}}[5m]))" if labels else f"sum(rate({name}{{{selector}}}[5m]))"
    return f"{name}{{{selector}}}"


def element_reference(name: str) -> dict:
    return {"kind": "ElementReference", "name": name}


def grid_row(title: str, element_names: list[str], *, conditional: dict | None = None) -> dict:
    items = []
    width = 24 // min(4, len(element_names))
    for index, name in enumerate(element_names):
        items.append({"kind": "GridLayoutItem", "spec": {"x": (index % 4) * width, "y": (index // 4) * 7, "width": width, "height": 7, "element": element_reference(name)}})
    spec: dict = {"title": title, "collapse": False, "layout": {"kind": "GridLayout", "spec": {"items": items}}}
    if conditional:
        spec["conditionalRendering"] = conditional
    return {"kind": "RowsLayoutRow", "spec": spec}


def tab(title: str, rows: list[dict]) -> dict:
    return {"kind": "TabsLayoutTab", "spec": {"title": title, "layout": {"kind": "RowsLayout", "spec": {"rows": rows}}}}


def render() -> dict:
    metrics = metric_by_name(catalog()["metrics"])
    elements = dict([
        panel("reports-exact", [panel_query(LOKI, 'sum(count_over_time({service_name="rfc6035-2otel",service_instance_id=~"$site"} | event_name=`rfc6035.report.received` | rfc6035_sender_name=~"$sender" [$__range]))', instant=True)], viz="stat", unit="short"),
        panel("reporting-handsets", [panel_query(LOKI, 'count(sum by (rfc6035_sender_name) (count_over_time({service_name="rfc6035-2otel",service_instance_id=~"$site"} | event_name=`rfc6035.report.received` | rfc6035_sender_name=~"$sender" [$__range])))', instant=True)], viz="stat", unit="short"),
        panel("mos-lq-mean", [panel_query(PROMETHEUS, quality_mean(metrics["rfc6035.call.mos_lq"]["prometheus"]), instant=True)], viz="stat", unit="none", field_link=True),
        panel("mos-lq-p95", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.mos_lq"]["prometheus"]), instant=True)], viz="stat", unit="none", field_link=True),
        panel("packet-loss-p95", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.packet_loss"]["prometheus"]), instant=True)], viz="stat", unit="percentunit", field_link=True),
        panel("unknown-sender-reports", [panel_query(PROMETHEUS, f'sum(increase({metrics["rfc6035_2otel.reports"]["prometheus"]}{{rfc6035_sender_name="unknown",service_instance_id=~"$site"}}[$__range]))', instant=True)], viz="stat", unit="short"),
        panel("export-failures", [panel_query(PROMETHEUS, f'sum(increase({metrics["rfc6035_2otel.export_failures"]["prometheus"]}{{service_instance_id=~"$site"}}[$__range]))', instant=True)], viz="stat", unit="short"),
        panel("rtt-p95", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.round_trip_delay"]["prometheus"]), instant=True)], viz="stat", unit="s", field_link=True),
        panel("jitter-p95", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.jitter"]["prometheus"], "rfc6035_jitter_kind"), instant=True)], viz="stat", unit="s", field_link=True),
        panel("handset-quality", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.mos_lq"]["prometheus"]), instant=True)], viz="bargauge", unit="none", field_link=True),
        panel("mos-lq-trend", [panel_query(PROMETHEUS, quality_trend_mean(metrics["rfc6035.call.mos_lq"]["prometheus"])), panel_query(PROMETHEUS, quality_trend_p95(metrics["rfc6035.call.mos_lq"]["prometheus"]), ref_id="B")], unit="none", field_link=True),
        panel("mos-cq-trend", [panel_query(PROMETHEUS, quality_trend_mean(metrics["rfc6035.call.mos_cq"]["prometheus"])), panel_query(PROMETHEUS, quality_trend_p95(metrics["rfc6035.call.mos_cq"]["prometheus"]), ref_id="B")], unit="none", field_link=True),
        panel("packet-loss-trend", [panel_query(PROMETHEUS, quality_trend_p95(metrics["rfc6035.call.packet_loss"]["prometheus"]))], unit="percentunit", field_link=True),
        panel("jitter-trend", [panel_query(PROMETHEUS, quality_trend_p95(metrics["rfc6035.call.jitter"]["prometheus"], "rfc6035_jitter_kind"))], unit="s", field_link=True),
        panel("rtt-trend", [panel_query(PROMETHEUS, quality_trend_p95(metrics["rfc6035.call.round_trip_delay"]["prometheus"]))], unit="s", field_link=True),
        panel("report-volume", [panel_query(PROMETHEUS, f'sum by (service_instance_id, rfc6035_sender_name) (increase({metrics["rfc6035_2otel.reports"]["prometheus"]}{{{QUALITY_FILTER}}}[{TREND_WINDOW}]))')], unit="short", field_link=True),
        panel("raw-reports", [panel_query(LOKI, '{service_name="rfc6035-2otel",service_instance_id=~"$site"} | event_name="rfc6035.report.received" | rfc6035_sender_name=~"$sender"', fmt="logs")], viz="logs", unit="short"),
        panel("r-factor-lq", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.r_factor_lq"]["prometheus"]), instant=True)], viz="bargauge", unit="none", field_link=True),
        panel("r-factor-cq", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.r_factor_cq"]["prometheus"]), instant=True)], viz="bargauge", unit="none", field_link=True),
        panel("discard-rate", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.discard_rate"]["prometheus"]), instant=True)], viz="bargauge", unit="percentunit", field_link=True),
        panel("dialect-breakdown", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.reports"]))], unit="short"),
        panel("report-type-breakdown", [panel_query(PROMETHEUS, f'sum by (rfc6035_report_type) (increase({metrics["rfc6035_2otel.reports"]["prometheus"]}{{{QUALITY_FILTER}}}[$__range]))', instant=True)], viz="bargauge", unit="short"),
        panel("endpoint-side-breakdown", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.jitter"]["prometheus"], "rfc6035_jitter_kind"), instant=True)], viz="bargauge", unit="s", field_link=True),
        panel("one-way-delay", [panel_query(PROMETHEUS, quality_p95(metrics["rfc6035.call.one_way_delay"]["prometheus"]), instant=True)], viz="bargauge", unit="s", field_link=True),
        panel("build-info", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.build_info"]), instant=True, fmt="table")], viz="table"),
        panel("datagrams", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.datagrams"]))], unit="ops"),
        panel("reports", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.reports"]))], unit="ops"),
        panel("parse-errors", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.parse_errors"]))], unit="ops"),
        panel("duplicates", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.duplicates"]))], unit="ops"),
        panel("responses", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.responses"]))], unit="ops"),
        panel("dedupe-cache-usage", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.dedupe_cache.usage"]))]),
        panel("processing-duration", [panel_query(PROMETHEUS, metric_expr(metrics["rfc6035_2otel.report.process.duration"]))], unit="s"),
        panel("collector-export-failures", [panel_query(PROMETHEUS, f'sum by (service_instance_id, rfc6035_2otel_signal) (increase({metrics["rfc6035_2otel.export_failures"]["prometheus"]}{{service_instance_id=~"$site"}}[$__range]))', instant=True)], viz="bargauge", unit="short"),
    ])
    one_way_condition = {"kind": "ConditionalRenderingGroup", "spec": {"visibility": "show", "condition": "and", "items": [{"kind": "ConditionalRenderingVariable", "spec": {"variable": "has_one_way_delay", "operator": "matches", "value": ".+"}}]}}
    return {
        "apiVersion": "dashboard.grafana.app/v2", "kind": "Dashboard", "metadata": {"name": "rfc6035-2otel"},
        "spec": {
            "title": "RFC 6035 voice quality", "description": "Operational quality and collector health for RFC 6035 reports. Generated by grafana/build_dashboard.py.",
            "tags": ["rfc6035", "generated"], "annotations": [], "links": [], "preload": False, "cursorSync": "Crosshair",
            "timeSettings": {"from": "now-6h", "to": "now", "autoRefresh": "1m", "autoRefreshIntervals": ["10s", "30s", "1m", "5m", "15m", "1h"], "timezone": "browser", "hideTimepicker": False, "fiscalYearStartMonth": 0},
            "variables": [
                datasource_variable("ds_prometheus", "Prometheus", "prometheus", "grafanacloud-prom"),
                datasource_variable("ds_loki", "Loki", "loki", "grafanacloud-logs"),
                query_variable("site", "Site", f'label_values({metrics["rfc6035_2otel.reports"]["prometheus"]}, service_instance_id)'),
                query_variable("sender", "Sender", f'label_values({metrics["rfc6035_2otel.reports"]["prometheus"]}{{service_instance_id=~"$site"}}, rfc6035_sender_name)'),
                query_variable("has_one_way_delay", "has_one_way_delay", 'label_values(rfc6035_call_one_way_delay_seconds_count{service_instance_id=~"$site",rfc6035_sender_name=~"$sender"}, __name__)', hidden=True),
            ],
            "elements": elements,
            "layout": {"kind": "TabsLayout", "spec": {"tabs": [
                tab("Operations", [grid_row("Operational summary", [f"panel-{panel_id}" for panel_id in range(101, 111)])]),
                tab("Call quality", [grid_row("Quality by site and handset", [f"panel-{panel_id}" for panel_id in range(201, 207)])]),
                tab("Raw Reports", [grid_row("Received RFC 6035 reports", ["panel-301"])]),
                tab("Diagnostics", [grid_row("Quality diagnostics", [f"panel-{panel_id}" for panel_id in range(401, 407)]), grid_row("One-way delay", ["panel-407"], conditional=one_way_condition)]),
                tab("Collector", [grid_row("Collector self-observability", [f"panel-{panel_id}" for panel_id in range(501, 510)])]),
            ]}},
        },
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    write_or_check(OUT, dump_json(render()), args.check)
    write_or_check(SIGNALS_DOC, render_signals_doc(), args.check)
    print(f"dashboard coverage: {len(catalog()['metrics'])} catalog metrics")


if __name__ == "__main__":
    main()
