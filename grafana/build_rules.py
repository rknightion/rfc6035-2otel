#!/usr/bin/env python3
"""Generate Grafana-managed alert resources from the signal catalog."""

from __future__ import annotations

import argparse

import build_dashboard
from common import ROOT, catalog, dump_json, write_or_check
from panels import PANEL_REGISTRY

OUT = ROOT / "alerts" / "grafana-managed"
FOLDER_UID = "REPLACE_WITH_FOLDER_UID"
DASHBOARD_UID = "rfc6035-2otel"
PROMETHEUS_UID = "grafanacloud-prom"
RUNBOOK_URL = "https://github.com/rknightion/rfc6035-2otel/blob/main/docs/troubleshooting.md"

ALERTS = (
    {
        "filename": "rfc6035-2otel-high-packet-loss.json",
        "title": "High packet loss in arriving RFC 6035 reports",
        "summary": "High packet loss in arriving RFC 6035 reports",
        "description": "The p95 packet loss for a reporting site and sender is above 5% over 15m.",
        "panel_key": "packet-loss-p95",
        "category": "quality",
        "lookback": "15m",
        "threshold": 0.05,
    },
    {
        "filename": "rfc6035-2otel-export-failures.json",
        "title": "RFC 6035 telemetry export is failing",
        "summary": "RFC 6035 telemetry export is failing",
        "description": "Metrics or logs export failures were recorded for a site and signal over 15m.",
        "panel_key": "export-failures",
        "category": "export",
        "lookback": "15m",
        "threshold": 0,
    },
    {
        "filename": "rfc6035-2otel-unknown-sender-reports.json",
        "title": "RFC 6035 reports from an unknown sender",
        "summary": "RFC 6035 reports arrived from an unknown sender",
        "description": "A site received RFC 6035 reports from the bounded unknown sender over 1h.",
        "panel_key": "unknown-sender-reports",
        "category": "identity",
        "lookback": "1h",
        "threshold": 0,
    },
)


def _validated_alerts(alerts: tuple[dict, ...] | list[dict]) -> tuple[dict, ...]:
    seen_keys: set[str] = set()
    seen_filenames: set[str] = set()
    for alert in alerts:
        panel_key = alert["panel_key"]
        filename = alert["filename"]
        if panel_key in seen_keys:
            raise ValueError(f"duplicate panel key: {panel_key}")
        if filename in seen_filenames:
            raise ValueError(f"duplicate alert filename: {filename}")
        seen_keys.add(panel_key)
        seen_filenames.add(filename)
    return tuple(alerts)


def _contains_element_reference(value: object, name: str) -> bool:
    if isinstance(value, dict):
        return value.get("kind") == "ElementReference" and value.get("name") == name or any(
            _contains_element_reference(child, name) for child in value.values()
        )
    if isinstance(value, list):
        return any(_contains_element_reference(child, name) for child in value)
    return False


def _validate_panel_bindings(alerts: tuple[dict, ...]) -> None:
    for alert in alerts:
        key = alert["panel_key"]
        if key not in PANEL_REGISTRY:
            raise ValueError(f"missing panel key: {key}")
    dashboard = build_dashboard.render()
    if dashboard["metadata"]["name"] != DASHBOARD_UID:
        raise ValueError(f"dashboard UID is not {DASHBOARD_UID}")
    elements = dashboard["spec"]["elements"]
    tabs = dashboard["spec"]["layout"]["spec"]["tabs"]
    for alert in alerts:
        key = alert["panel_key"]
        panel_id, title, tab = PANEL_REGISTRY[key]
        element_name = f"panel-{panel_id}"
        element = elements.get(element_name)
        if element is None:
            raise ValueError(f"missing dashboard panel: {element_name}")
        if element["spec"]["title"] != title:
            raise ValueError(f"panel title mismatch for {key}")
        matched_tab = next(
            (candidate["spec"]["title"] for candidate in tabs if _contains_element_reference(candidate, element_name)),
            None,
        )
        if matched_tab != tab:
            raise ValueError(f"panel tab mismatch for {key}")


def _metric_names() -> dict[str, str]:
    return {metric["name"]: metric["prometheus"] for metric in catalog()["metrics"]}


def _prometheus_expression(expr: str, lookback: str) -> dict:
    return {
        "datasourceUID": PROMETHEUS_UID,
        "queryType": "instant",
        "relativeTimeRange": {"from": lookback, "to": "0s"},
        "model": {
            "refId": "A",
            "expr": expr,
            "instant": True,
            "range": False,
            "datasource": {"type": "prometheus", "uid": PROMETHEUS_UID},
        },
    }


def _condition_expressions(query: str, lookback: str, threshold: float) -> dict:
    return {
        "A": _prometheus_expression(query, lookback),
        "B": {
            "datasourceUID": "__expr__",
            "model": {
                "refId": "B",
                "type": "reduce",
                "expression": "A",
                "reducer": "last",
                "datasource": {"type": "__expr__", "uid": "__expr__"},
            },
        },
        "C": {
            "source": True,
            "datasourceUID": "__expr__",
            "model": {
                "refId": "C",
                "type": "threshold",
                "expression": "B",
                "datasource": {"type": "__expr__", "uid": "__expr__"},
                "conditions": [{"evaluator": {"type": "gt", "params": [threshold]}}],
            },
        },
    }


def _query_for(alert: dict, names: dict[str, str]) -> str:
    reports = names["rfc6035_2otel.reports"]
    if alert["panel_key"] == "packet-loss-p95":
        loss = names["rfc6035.call.packet_loss"]
        labels = "service_instance_id, rfc6035_sender_name"
        return (
            f"histogram_quantile(0.95, sum by (le, {labels}) (increase({loss}_bucket[15m]))) "
            f"and on ({labels}) sum by ({labels}) (increase({reports}[15m])) > 0"
        )
    if alert["panel_key"] == "export-failures":
        failures = names["rfc6035_2otel.export_failures"]
        return f"sum by (service_instance_id, rfc6035_2otel_signal) (increase({failures}[15m]))"
    if alert["panel_key"] == "unknown-sender-reports":
        return (
            f'sum by (service_instance_id, rfc6035_sender_name) '
            f'(increase({reports}{{rfc6035_sender_name="unknown"}}[1h]))'
        )
    raise ValueError(f"no query for panel key: {alert['panel_key']}")


def _resource(alert: dict, query: str) -> dict:
    panel_id, _, _ = PANEL_REGISTRY[alert["panel_key"]]
    uid = alert["filename"].removesuffix(".json")
    return {
        "apiVersion": "rules.alerting.grafana.app/v0alpha1",
        "kind": "AlertRule",
        "metadata": {
            "name": uid,
            "annotations": {"grafana.app/folder": FOLDER_UID},
            "labels": {"grafana.app/folder": FOLDER_UID},
        },
        "spec": {
            "title": alert["title"],
            "trigger": {"interval": "1m"},
            "for": "5m",
            "paused": False,
            "noDataState": "Ok",
            "execErrState": "Error",
            "labels": {
                "category": alert["category"],
                "pipeline": "rfc6035-2otel",
                "service": "rfc6035-2otel",
                "severity": "warning",
                "source": "rfc6035-2otel",
            },
            "annotations": {
                "__dashboardUid__": DASHBOARD_UID,
                "__panelId__": str(panel_id),
                "panelKey": alert["panel_key"],
                "summary": alert["summary"],
                "description": alert["description"],
                "runbook_url": RUNBOOK_URL,
            },
            "panelRef": {"dashboardUID": DASHBOARD_UID, "panelID": panel_id},
            "expressions": _condition_expressions(query, alert["lookback"], alert["threshold"]),
        },
    }


def render() -> dict[str, str]:
    alerts = _validated_alerts(ALERTS)
    _validate_panel_bindings(alerts)
    names = _metric_names()
    return {alert["filename"]: dump_json(_resource(alert, _query_for(alert, names))) for alert in alerts}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    for filename, resource in render().items():
        write_or_check(OUT / filename, resource, args.check)
    print("alert rules: 3 App Platform resources")


if __name__ == "__main__":
    main()
