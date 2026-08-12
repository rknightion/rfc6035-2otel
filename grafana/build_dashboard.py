#!/usr/bin/env python3
"""Generate the RFC 6035 quality and collector-health dashboard."""

from __future__ import annotations

import argparse

from common import ROOT, catalog, dump_json, write_or_check

OUT = ROOT / "dashboards" / "rfc6035-2otel.json"
SIGNALS_DOC = ROOT / "docs" / "signals.md"
SIGNALS_START = "<!-- BEGIN GENERATED SIGNAL CATALOG -->"
SIGNALS_END = "<!-- END GENERATED SIGNAL CATALOG -->"


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
    return (
        before
        + SIGNALS_START
        + "\n\n"
        + render_signal_catalog()
        + "\n\n"
        + SIGNALS_END
        + after
    )


def histogram_quantile(metric: dict, attributes: list[str], quantile: float = 0.95) -> str:
    labels = ["le", *(prometheus_label(attribute) for attribute in attributes)]
    return (
        f"histogram_quantile({quantile}, "
        f"sum by ({', '.join(labels)}) (rate({metric['prometheus']}_bucket[5m])))"
    )


def rate(metric: dict) -> str:
    labels = [prometheus_label(attribute) for attribute in metric["attributes"]]
    grouping = f" by ({', '.join(labels)})" if labels else ""
    return f"sum{grouping} (rate({metric['prometheus']}[5m]))"


def legend(attributes: list[str]) -> str:
    return " ".join(f"{{{{{prometheus_label(attribute)}}}}}" for attribute in attributes)


def row(panel_id: int, title: str, y: int) -> dict:
    return {
        "collapsed": False,
        "gridPos": {"h": 1, "w": 24, "x": 0, "y": y},
        "id": panel_id,
        "panels": [],
        "title": title,
        "type": "row",
    }


def panel(
    panel_id: int,
    title: str,
    expr: str,
    unit: str,
    attributes: list[str],
    x: int,
    y: int,
    description: str = "",
) -> dict:
    return {
        "description": description,
        "fieldConfig": {"defaults": {"unit": unit}, "overrides": []},
        "gridPos": {"h": 8, "w": 12, "x": x, "y": y},
        "id": panel_id,
        "targets": [{"expr": expr, "legendFormat": legend(attributes), "refId": "A"}],
        "title": title,
        "type": "timeseries",
    }


def render() -> dict:
    metrics = metric_by_name(catalog()["metrics"])
    panels: list[dict] = []
    panel_id = 1

    panels.append(row(panel_id, "Call quality", 0))
    panel_id += 1
    quality = [
        ("MOS LQ", "rfc6035.call.mos_lq", "none", ""),
        ("MOS CQ", "rfc6035.call.mos_cq", "none", ""),
        ("R-factor LQ", "rfc6035.call.r_factor_lq", "none", ""),
        ("R-factor CQ", "rfc6035.call.r_factor_cq", "none", ""),
        ("Packet loss", "rfc6035.call.packet_loss", "percentunit", ""),
        ("Discard rate", "rfc6035.call.discard_rate", "percentunit", ""),
        ("Jitter", "rfc6035.call.jitter", "s", ""),
        ("Round-trip delay", "rfc6035.call.round_trip_delay", "s", ""),
        (
            "One-way delay",
            "rfc6035.call.one_way_delay",
            "s",
            "No data is normal: the captured Poly reports omit ESD and SOWD.",
        ),
    ]
    for index, (title, name, unit, description) in enumerate(quality):
        metric = metrics[name]
        panels.append(
            panel(
                panel_id,
                title,
                histogram_quantile(metric, metric["attributes"]),
                unit,
                metric["attributes"],
                (index % 2) * 12,
                1 + (index // 2) * 8,
                description,
            )
        )
        panel_id += 1

    health_y = 1 + ((len(quality) + 1) // 2) * 8
    panels.append(row(panel_id, "Collector health", health_y))
    panel_id += 1
    health = [
        ("Build info", "rfc6035_2otel.build_info", "short"),
        ("Datagram outcomes", "rfc6035_2otel.datagrams", "ops"),
        ("Reports by dialect, type, and sender", "rfc6035_2otel.reports", "ops"),
        ("Parse errors", "rfc6035_2otel.parse_errors", "ops"),
        ("Duplicates / dedupe hits", "rfc6035_2otel.duplicates", "ops"),
        ("SIP responses", "rfc6035_2otel.responses", "ops"),
        ("Export failures", "rfc6035_2otel.export_failures", "ops"),
        ("Dedupe cache usage", "rfc6035_2otel.dedupe_cache.usage", "short"),
        ("Report processing duration", "rfc6035_2otel.report.process.duration", "s"),
    ]
    build_attributes = [
        "service.version",
        "vcs.ref.head.revision",
        "rfc6035_2otel.build.date",
        "rfc6035_2otel.build.go_version",
    ]
    for index, (title, name, unit) in enumerate(health):
        metric = metrics[name]
        attributes = metric["attributes"]
        if metric["kind"] == "histogram":
            expr = histogram_quantile(metric, attributes)
        elif metric["kind"] == "counter":
            expr = rate(metric)
        else:
            expr = metric["prometheus"]
        if name == "rfc6035_2otel.build_info":
            attributes = build_attributes
        panels.append(
            panel(
                panel_id,
                title,
                expr,
                unit,
                attributes,
                (index % 2) * 12,
                health_y + 1 + (index // 2) * 8,
            )
        )
        panel_id += 1

    covered = {name for _, name, _, _ in quality} | {name for _, name, _ in health}
    additions = [metric for metric in catalog()["metrics"] if metric["name"] not in covered]
    if additions:
        additions_y = health_y + 1 + ((len(health) + 1) // 2) * 8
        panels.append(row(panel_id, "Catalog additions", additions_y))
        panel_id += 1
        for index, metric in enumerate(additions):
            expr = (
                histogram_quantile(metric, metric["attributes"])
                if metric["kind"] == "histogram"
                else rate(metric)
                if metric["kind"] == "counter"
                else metric["prometheus"]
            )
            panels.append(
                panel(
                    panel_id,
                    f"Catalog signal: {metric['name']}",
                    expr,
                    metric["unit"],
                    metric["attributes"],
                    (index % 2) * 12,
                    additions_y + 1 + (index // 2) * 8,
                )
            )
            panel_id += 1

    return {
        "panels": panels,
        "refresh": "1m",
        "schemaVersion": 41,
        "tags": ["rfc6035", "generated"],
        "time": {"from": "now-6h", "to": "now"},
        "timezone": "browser",
        "title": "RFC 6035 voice quality",
        "uid": "rfc6035-2otel",
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
