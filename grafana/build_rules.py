#!/usr/bin/env python3
"""Generate Grafana alert rules from the signal catalog."""

from __future__ import annotations

import argparse

from common import ROOT, catalog, write_or_check

OUT = ROOT / "alerts" / "rfc6035-2otel.yaml"


def render() -> str:
    names = {metric["name"]: metric["prometheus"] for metric in catalog()["metrics"]}
    reports = names["rfc6035_2otel.reports"]
    loss = names["rfc6035.call.packet_loss"]
    export_failures = names["rfc6035_2otel.export_failures"]
    rules = [
        (
            "rfc6035-2otel-high-packet-loss",
            f"(sum(increase({reports}[5m])) > 0) and histogram_quantile(0.95, sum by (le) (rate({loss}_bucket[5m]))) > 0.05",
            "warning",
            "High packet loss in arriving RFC 6035 reports",
        ),
        (
            "rfc6035-2otel-export-failures",
            f"increase({export_failures}[5m]) > 0",
            "warning",
            "RFC 6035 telemetry export is failing",
        ),
    ]
    lines = ["apiVersion: 1", "groups:", "  - orgId: 1", "    name: rfc6035-2otel", "    folder: rfc6035-2otel", "    interval: 1m", "    rules:"]
    for uid, expr, severity, summary in rules:
        lines.extend([
            f"      - uid: {uid}", f"        title: {summary}", "        condition: A", "        for: 5m",
            "        noDataState: OK", "        execErrState: Error", "        labels:", "          pipeline: rfc6035-2otel",
            f"          severity: {severity}", "          source: rfc6035-2otel", "        annotations:",
            f"          summary: {summary}", "        data:", "          - refId: A", "            relativeTimeRange: {from: 300, to: 0}",
            "            datasourceUid: grafanacloud-prom", "            model:",
            "              datasource: {type: prometheus, uid: grafanacloud-prom}", "              editorMode: code",
            f"              expr: '{expr}'", "              instant: true", "              refId: A",
        ])
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    write_or_check(OUT, render(), args.check)
    print("alert rules: 2 catalog-resolved expressions")


if __name__ == "__main__":
    main()
