import json
import re
import unittest
from unittest.mock import patch

import build_dashboard
import build_rules
from panels import PANEL_REGISTRY, _validated_registry


class GeneratedArtefactTests(unittest.TestCase):
    def test_dashboard_is_complete_v2_resource_with_resolved_layout_references(self):
        dashboard = build_dashboard.render()
        self.assertEqual(dashboard["apiVersion"], "dashboard.grafana.app/v2")
        self.assertEqual(dashboard["kind"], "Dashboard")
        self.assertEqual(dashboard["metadata"]["name"], "rfc6035-2otel")
        self.assertNotIn("panels", dashboard)
        spec = dashboard["spec"]
        self.assertEqual(
            [tab["spec"]["title"] for tab in spec["layout"]["spec"]["tabs"]],
            ["Operations", "Call quality", "Raw Reports", "Diagnostics", "Collector"],
        )
        self.assertEqual(len(spec["elements"]), len(set(spec["elements"])))
        references = []

        def visit(value):
            if isinstance(value, dict):
                if value.get("kind") == "ElementReference":
                    references.append(value["name"])
                for child in value.values():
                    visit(child)
            elif isinstance(value, list):
                for child in value:
                    visit(child)

        visit(spec["layout"])
        self.assertTrue(references)
        self.assertTrue(set(references).issubset(spec["elements"]))

    def test_variables_and_query_datasources_use_declared_defaults(self):
        dashboard = build_dashboard.render()
        variables = {variable["spec"]["name"]: variable for variable in dashboard["spec"]["variables"]}
        self.assertEqual(
            set(variables),
            {"ds_prometheus", "ds_loki", "site", "sender", "has_one_way_delay"},
        )
        self.assertEqual(variables["ds_prometheus"]["spec"]["pluginId"], "prometheus")
        self.assertEqual(variables["ds_prometheus"]["spec"]["current"]["value"], "grafanacloud-prom")
        self.assertEqual(variables["ds_loki"]["spec"]["pluginId"], "loki")
        self.assertEqual(variables["ds_loki"]["spec"]["current"]["value"], "grafanacloud-logs")
        for name in ("site", "sender"):
            variable = variables[name]["spec"]
            self.assertEqual(variable["query"]["datasource"]["name"], "${ds_prometheus}")
            self.assertTrue(variable["multi"])
            self.assertTrue(variable["includeAll"])
            self.assertEqual(variable["allValue"], ".*")
        delayed = variables["has_one_way_delay"]["spec"]
        self.assertEqual(delayed["hide"], "hideVariable")
        self.assertEqual(delayed["current"], {"text": "", "value": ""})
        self.assertFalse(delayed["allowCustomValue"])
        self.assertTrue(delayed["skipUrlSync"])
        self.assertEqual(
            delayed["query"]["spec"]["query"],
            "label_values(rfc6035_call_one_way_delay_seconds_count{service_instance_id=~\"$site\",rfc6035_sender_name=~\"$sender\"}, __name__)",
        )

    def test_panel_registry_is_stable_and_quality_queries_keep_link_dimensions(self):
        dashboard = build_dashboard.render()
        ids = [entry[0] for entry in PANEL_REGISTRY.values()]
        self.assertEqual(len(ids), len(set(ids)))
        self.assertEqual(PANEL_REGISTRY["packet-loss-p95"][1:], ("p95 packet loss", "Operations"))
        self.assertEqual(PANEL_REGISTRY["export-failures"][1:], ("export failures", "Operations"))
        self.assertEqual(PANEL_REGISTRY["unknown-sender-reports"][1:], ("unknown-sender reports", "Operations"))
        rendered = json.dumps(dashboard)
        self.assertIn("service_instance_id", rendered)
        self.assertIn("rfc6035_sender_name", rendered)
        self.assertIn("dtab=Raw-Reports", rendered)
        self.assertIn("${site:queryparam}", rendered)
        self.assertIn("${sender:queryparam}", rendered)

    def test_panel_registry_rejects_duplicate_semantic_keys_and_ids(self):
        with self.assertRaisesRegex(ValueError, "semantic key"):
            _validated_registry([("one", 1, "one", "Operations"), ("one", 2, "two", "Operations")])
        with self.assertRaisesRegex(ValueError, "panel ID"):
            _validated_registry([("one", 1, "one", "Operations"), ("two", 1, "two", "Operations")])

    def test_every_catalog_metric_is_covered_and_every_panel_query_is_wired(self):
        dashboard = build_dashboard.render()
        rendered = json.dumps(dashboard)
        for metric in build_dashboard.catalog()["metrics"]:
            self.assertIn(metric["prometheus"], rendered, metric["name"])
        for element in dashboard["spec"]["elements"].values():
            for query in element["spec"].get("data", {}).get("spec", {}).get("queries", []):
                data_query = query["spec"]["query"]
                datasource = data_query["datasource"]["name"]
                self.assertEqual(query["spec"]["datasource"]["uid"], datasource)
                self.assertIn(datasource, {"${ds_prometheus}", "${ds_loki}"})
                if data_query["group"] == "loki":
                    self.assertEqual(datasource, "${ds_loki}")
                else:
                    self.assertEqual(datasource, "${ds_prometheus}")

    def test_collector_tab_exposes_all_nine_self_observability_instruments(self):
        dashboard = build_dashboard.render()
        collector = dashboard["spec"]["layout"]["spec"]["tabs"][4]
        references = [
            item["spec"]["element"]["name"]
            for row in collector["spec"]["layout"]["spec"]["rows"]
            for item in row["spec"]["layout"]["spec"]["items"]
        ]
        self.assertEqual(references, [f"panel-{panel_id}" for panel_id in range(501, 510)])
        self.assertEqual(
            dashboard["spec"]["elements"]["panel-509"]["spec"]["title"],
            "export failures by signal",
        )

    def test_quality_trends_use_a_fixed_sparse_event_window(self):
        dashboard = build_dashboard.render()
        elements = dashboard["spec"]["elements"]
        for panel_id in range(201, 206):
            expressions = [
                query["spec"]["query"]["spec"]["expr"]
                for query in elements[f"panel-{panel_id}"]["spec"]["data"]["spec"]["queries"]
            ]
            for expression in expressions:
                self.assertIn("[24h]", expression)
                self.assertNotIn("[$__range]", expression)
                self.assertRegex(expression, r"_count\{.*\}\[24h\]\)\) > 0")
        report_volume = elements["panel-206"]["spec"]["data"]["spec"]["queries"][0]["spec"]["query"]["spec"]["expr"]
        self.assertIn("[24h]", report_volume)
        self.assertNotIn("[$__range]", report_volume)

        for element in elements.values():
            for query in element["spec"].get("data", {}).get("spec", {}).get("queries", []):
                model = query["spec"]["query"]["spec"]
                if "[$__range]" in model["expr"]:
                    self.assertTrue(model["instant"], element["spec"]["title"])

    def test_diagnostics_and_collector_queries_follow_site_and_sender_selection(self):
        dashboard = build_dashboard.render()
        elements = dashboard["spec"]["elements"]

        for panel_id in [404, *range(501, 510)]:
            rendered = json.dumps(elements[f"panel-{panel_id}"])
            self.assertIn('service_instance_id=~\\"$site\\"', rendered)

        for panel_id in (404, 503, 505):
            rendered = json.dumps(elements[f"panel-{panel_id}"])
            self.assertIn('rfc6035_sender_name=~\\"$sender\\"', rendered)

    def test_rules_use_only_catalog_metric_names(self):
        rendered = build_rules.render()
        known = {metric["prometheus"] for metric in build_rules.catalog()["metrics"]}
        for resource in rendered.values():
            resource = json.loads(resource)
            expression = resource["spec"]["expressions"]["A"]["model"]["expr"]
            self.assertTrue(any(name in expression for name in known), expression)

    def test_one_way_delay_row_and_exact_loki_summaries_follow_contract(self):
        dashboard = build_dashboard.render()
        diagnostics = dashboard["spec"]["layout"]["spec"]["tabs"][3]["spec"]["layout"]["spec"]["rows"]
        delay = next(row for row in diagnostics if row["spec"]["title"] == "One-way delay")
        item = delay["spec"]["conditionalRendering"]["spec"]["items"][0]
        self.assertEqual(delay["kind"], "RowsLayoutRow")
        self.assertEqual(delay["spec"]["conditionalRendering"]["spec"]["visibility"], "show")
        self.assertEqual(item, {"kind": "ConditionalRenderingVariable", "spec": {"variable": "has_one_way_delay", "operator": "matches", "value": ".+"}})
        rendered = json.dumps(dashboard)
        self.assertIn('event_name=`rfc6035.report.received`', rendered)
        raw_query = dashboard["spec"]["elements"]["panel-301"]["spec"]["data"]["spec"]["queries"][0]["spec"]["query"]["spec"]["expr"]
        self.assertIn('event_name="rfc6035.report.received"', raw_query)

    def test_docs_and_catalog_match_bidirectionally(self):
        signals = (build_dashboard.ROOT / "docs" / "signals.md").read_text(encoding="utf-8")
        generated = signals.split(build_dashboard.SIGNALS_START, 1)[1].split(build_dashboard.SIGNALS_END, 1)[0]
        documented = set(re.findall(r"^\| `([^`]+)` \| `([^`]+)` \|", generated, re.MULTILINE))
        catalogued = {
            (metric["name"], metric["prometheus"])
            for metric in build_dashboard.catalog()["metrics"]
        }
        self.assertEqual(documented, catalogued)

    def test_alert_rules_are_stable_app_platform_resources(self):
        rendered = build_rules.render()
        self.assertEqual(
            set(rendered),
            {
                "rfc6035-2otel-high-packet-loss.json",
                "rfc6035-2otel-export-failures.json",
                "rfc6035-2otel-unknown-sender-reports.json",
            },
        )
        expected_panels = {
            "rfc6035-2otel-high-packet-loss.json": ("packet-loss-p95", 105, "quality"),
            "rfc6035-2otel-export-failures.json": ("export-failures", 107, "export"),
            "rfc6035-2otel-unknown-sender-reports.json": ("unknown-sender-reports", 106, "identity"),
        }
        for filename, (panel_key, panel_id, category) in expected_panels.items():
            resource = json.loads(rendered[filename])
            self.assertEqual(resource["apiVersion"], "rules.alerting.grafana.app/v0alpha1")
            self.assertEqual(resource["kind"], "AlertRule")
            self.assertEqual(resource["metadata"]["name"], filename.removesuffix(".json"))
            self.assertEqual(
                resource["metadata"]["annotations"],
                {"grafana.app/folder": "REPLACE_WITH_FOLDER_UID"},
            )
            self.assertEqual(
                resource["metadata"]["labels"],
                {"grafana.app/folder": "REPLACE_WITH_FOLDER_UID"},
            )
            spec = resource["spec"]
            self.assertEqual(spec["trigger"], {"interval": "1m"})
            self.assertEqual(spec["for"], "5m")
            self.assertEqual(spec["noDataState"], "Ok")
            self.assertEqual(spec["execErrState"], "Error")
            self.assertFalse(spec["paused"])
            self.assertEqual(
                spec["labels"],
                {
                    "category": category,
                    "pipeline": "rfc6035-2otel",
                    "service": "rfc6035-2otel",
                    "severity": "warning",
                    "source": "rfc6035-2otel",
                },
            )
            self.assertEqual(spec["panelRef"], {"dashboardUID": "rfc6035-2otel", "panelID": panel_id})
            self.assertEqual(
                spec["annotations"]["__dashboardUid__"],
                spec["panelRef"]["dashboardUID"],
            )
            self.assertEqual(spec["annotations"]["__panelId__"], str(spec["panelRef"]["panelID"]))
            self.assertEqual(spec["annotations"]["panelKey"], panel_key)
            self.assertEqual(
                spec["annotations"]["runbook_url"],
                "https://github.com/rknightion/rfc6035-2otel/blob/main/docs/troubleshooting.md",
            )
            self.assertEqual(set(spec["expressions"]), {"A", "B", "C"})
            self.assertEqual(spec["expressions"]["A"]["model"]["refId"], "A")
            self.assertEqual(spec["expressions"]["B"]["model"]["expression"], "A")
            self.assertEqual(spec["expressions"]["C"]["model"]["expression"], "B")
            self.assertTrue(spec["expressions"]["C"]["source"])

    def test_alert_queries_are_grouped_and_gated_by_their_bounded_dimensions(self):
        expressions = {
            name: json.loads(resource)["spec"]["expressions"]["A"]["model"]["expr"]
            for name, resource in build_rules.render().items()
        }
        self.assertIn("sum by (service_instance_id, rfc6035_sender_name)", expressions["rfc6035-2otel-high-packet-loss.json"])
        self.assertRegex(
            expressions["rfc6035-2otel-high-packet-loss.json"],
            r"sum by \(service_instance_id, rfc6035_sender_name\) \(increase\(rfc6035_2otel_reports_total\[15m\]\)\) > 0",
        )
        self.assertIn("sum by (service_instance_id, rfc6035_2otel_signal)", expressions["rfc6035-2otel-export-failures.json"])
        self.assertIn("[15m]", expressions["rfc6035-2otel-export-failures.json"])
        self.assertIn('rfc6035_sender_name="unknown"', expressions["rfc6035-2otel-unknown-sender-reports.json"])
        self.assertIn("sum by (service_instance_id, rfc6035_sender_name)", expressions["rfc6035-2otel-unknown-sender-reports.json"])
        self.assertIn("[1h]", expressions["rfc6035-2otel-unknown-sender-reports.json"])

    def test_alert_generation_rejects_invalid_panel_registry_bindings(self):
        with patch.object(build_rules, "PANEL_REGISTRY", {}):
            with self.assertRaisesRegex(ValueError, "missing panel key"):
                build_rules.render()
        duplicate = list(build_rules.ALERTS)
        duplicate.append({**duplicate[0], "filename": "duplicate.json"})
        with self.assertRaisesRegex(ValueError, "duplicate panel key"):
            build_rules._validated_alerts(duplicate)
        panel_id, title, tab = PANEL_REGISTRY["packet-loss-p95"]
        altered_registry = dict(build_rules.PANEL_REGISTRY)
        altered_registry["packet-loss-p95"] = (panel_id, title + " changed", tab)
        with patch.object(build_rules, "PANEL_REGISTRY", altered_registry):
            with self.assertRaisesRegex(ValueError, "title"):
                build_rules.render()
        altered_registry = dict(build_rules.PANEL_REGISTRY)
        altered_registry["packet-loss-p95"] = (panel_id, title, "Collector")
        with patch.object(build_rules, "PANEL_REGISTRY", altered_registry):
            with self.assertRaisesRegex(ValueError, "tab"):
                build_rules.render()


if __name__ == "__main__":
    unittest.main()
