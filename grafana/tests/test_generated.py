import json
import re
import unittest

import build_dashboard
import build_rules


class GeneratedArtefactTests(unittest.TestCase):
    def test_every_panel_query_metric_exists_in_catalog(self):
        known = {metric["prometheus"] for metric in build_dashboard.catalog()["metrics"]}
        for panel in build_dashboard.render()["panels"]:
            for target in panel.get("targets", []):
                expression = target["expr"]
                self.assertTrue(any(name in expression for name in known), expression)

    def test_every_catalog_metric_is_queried(self):
        rendered = json.dumps(build_dashboard.render())
        for metric in build_dashboard.catalog()["metrics"]:
            self.assertIn(metric["prometheus"], rendered, metric["name"])

    def test_rules_use_only_catalog_metric_names(self):
        rendered = build_rules.render()
        known = {metric["prometheus"] for metric in build_rules.catalog()["metrics"]}
        for line in rendered.splitlines():
            if "expr: '" not in line:
                continue
            expression = line.split("expr: '", 1)[1].rsplit("'", 1)[0]
            self.assertTrue(any(name in expression for name in known), expression)

    def test_dashboard_has_required_sections_and_preserves_health_dimensions(self):
        dashboard = build_dashboard.render()
        rows = [panel["title"] for panel in dashboard["panels"] if panel["type"] == "row"]
        self.assertEqual(rows[:2], ["Call quality", "Collector health"])
        self.assertTrue(set(rows[2:]).issubset({"Catalog additions"}))
        rendered = json.dumps(dashboard)
        for label in (
            "rfc6035_2otel_datagram_outcome",
            "rfc6035_report_dialect",
            "rfc6035_report_type",
            "rfc6035_sender_name",
            "error_type",
            "rfc6035_response_status_code",
            "rfc6035_2otel_signal",
        ):
            self.assertIn(label, rendered)

    def test_docs_and_catalog_match_bidirectionally(self):
        signals = (build_dashboard.ROOT / "docs" / "signals.md").read_text(encoding="utf-8")
        generated = signals.split(build_dashboard.SIGNALS_START, 1)[1].split(build_dashboard.SIGNALS_END, 1)[0]
        documented = set(re.findall(r"^\| `([^`]+)` \| `([^`]+)` \|", generated, re.MULTILINE))
        catalogued = {
            (metric["name"], metric["prometheus"])
            for metric in build_dashboard.catalog()["metrics"]
        }
        self.assertEqual(documented, catalogued)

    def test_alerts_treat_no_data_as_ok(self):
        rendered = build_rules.render()
        self.assertNotIn("noDataState: NoData", rendered)
        self.assertEqual(rendered.count("noDataState: OK"), 2)
        reports = next(
            metric["prometheus"]
            for metric in build_rules.catalog()["metrics"]
            if metric["name"] == "rfc6035_2otel.reports"
        )
        self.assertIn(f"sum(increase({reports}[5m])) > 0", rendered)


if __name__ == "__main__":
    unittest.main()
