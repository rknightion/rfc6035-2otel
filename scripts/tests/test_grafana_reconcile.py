"""Contracts for local Grafana reconciliation projections."""

from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPTS = Path(__file__).resolve().parents[1]


def load_script(name: str):
    path = SCRIPTS / name
    spec = importlib.util.spec_from_file_location(path.stem, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class GrafanaPruneProjectionTests(unittest.TestCase):
    """A bad folder projection must not delete a resource outside the RFC folder."""

    def test_folder_uid_prefers_annotation_and_falls_back_to_label(self) -> None:
        prune = load_script("grafana-prune-rules.py")

        self.assertEqual(
            prune.folder_uid(
                {
                    "metadata": {
                        "annotations": {"grafana.app/folder": "annotated"},
                        "labels": {"grafana.app/folder": "labelled"},
                    }
                }
            ),
            "annotated",
        )
        self.assertEqual(
            prune.folder_uid(
                {"metadata": {"labels": {"grafana.app/folder": "labelled"}}}
            ),
            "labelled",
        )

    def test_stale_names_rejects_an_empty_keep_set(self) -> None:
        prune = load_script("grafana-prune-rules.py")

        with self.assertRaisesRegex(ValueError, "expected rule names must not be empty"):
            prune.stale_resource_names(
                [{"metadata": {"name": "rfc-rule"}}], set(), "rfc-folder"
            )

    def test_expected_rule_resources_parses_kind_qualified_keep_lines(self) -> None:
        prune = load_script("grafana-prune-rules.py")

        with tempfile.TemporaryDirectory() as temporary_directory:
            keep_file = Path(temporary_directory) / "expected-rules.txt"
            keep_file.write_text("AlertRule/shared-name\nRecordingRule/recording-name\n")

            self.assertEqual(
                prune.expected_rule_resources(keep_file),
                {("AlertRule", "shared-name"), ("RecordingRule", "recording-name")},
            )

    def test_expected_rule_resources_refuses_unqualified_or_unknown_kind_lines(self) -> None:
        prune = load_script("grafana-prune-rules.py")

        for line in ("shared-name\n", "Rule/shared-name\n", "AlertRule/\n"):
            with self.subTest(line=line), tempfile.TemporaryDirectory() as temporary_directory:
                keep_file = Path(temporary_directory) / "expected-rules.txt"
                keep_file.write_text(line)

                with self.assertRaisesRegex(ValueError, "expected rule identities"):
                    prune.expected_rule_resources(keep_file)

    def test_stale_names_includes_both_rule_kinds_only_inside_folder(self) -> None:
        prune = load_script("grafana-prune-rules.py")

        resources = [
            {
                "kind": "AlertRule",
                "metadata": {
                    "name": "stale-alert",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
            },
            {
                "kind": "RecordingRule",
                "metadata": {
                    "name": "stale-recording",
                    "labels": {"grafana.app/folder": "rfc-folder"},
                },
            },
            {
                "kind": "AlertRule",
                "metadata": {
                    "name": "other-folder",
                    "annotations": {"grafana.app/folder": "other-folder"},
                },
            },
            {
                "kind": "AlertRule",
                "metadata": {
                    "name": "shared-name",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
            },
            {
                "kind": "RecordingRule",
                "metadata": {
                    "name": "shared-name",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
            },
            {
                "kind": "RecordingRule",
                "metadata": {
                    "name": "recording-name",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
            },
        ]

        self.assertEqual(
            prune.stale_resource_names(
                resources,
                {("AlertRule", "shared-name"), ("RecordingRule", "recording-name")},
                "rfc-folder",
            ),
            [
                ("AlertRule", "stale-alert"),
                ("RecordingRule", "shared-name"),
                ("RecordingRule", "stale-recording"),
            ],
        )


class GrafanaRuleProjectionTests(unittest.TestCase):
    """A live rule change must be visible even when Grafana adds unrelated fields."""

    def test_semantic_projection_ignores_server_fields_but_retains_rule_contract(self) -> None:
        verify = load_script("grafana-verify-rules.py")

        resource = {
            "apiVersion": "rules.alerting.grafana.app/v0alpha1",
            "kind": "AlertRule",
            "metadata": {
                "name": "rfc-rule",
                "annotations": {"grafana.app/folder": "rfc-folder", "runbook_url": "https://runbook"},
                "labels": {"grafana.app/folder": "rfc-folder", "severity": "warning"},
                "uid": "server-uid",
            },
            "spec": {
                "title": "Packet loss",
                "paused": False,
                "for": "5m",
                "noDataState": "Ok",
                "execErrState": "Error",
                "trigger": {"interval": "1m"},
                "labels": {"team": "networking"},
                "annotations": {"summary": "High packet loss"},
                "expressions": [{"refId": "A", "model": {"expr": "up"}}],
                "panelRef": {"uid": "rfc6035-2otel", "id": 12},
                "updated": "server-only",
            },
        }

        self.assertEqual(
            verify.semantic_rule(resource),
            {
                "folder": "rfc-folder",
                "title": "Packet loss",
                "paused": False,
                "for": 300,
                "noDataState": "Ok",
                "execErrState": "Error",
                "trigger": {"interval": "1m"},
                "labels": {"team": "networking"},
                "annotations": {"summary": "High packet loss"},
                "expressions": [{"refId": "A", "model": {"expr": "up"}}],
                "panelRef": {"uid": "rfc6035-2otel", "id": 12},
            },
        )

    def test_semantic_projection_normalizes_grafana_rule_defaults(self) -> None:
        verify = load_script("grafana-verify-rules.py")
        metadata = {"annotations": {"grafana.app/folder": "rfc-folder"}}
        expected = {
            "metadata": metadata,
            "spec": {
                "paused": False,
                "for": "5m",
                "expressions": {
                    "A": {
                        "datasourceUID": "grafanacloud-prom",
                        "model": {
                            "datasource": {"type": "prometheus", "uid": "grafanacloud-prom"},
                            "expr": "up",
                            "refId": "A",
                        },
                        "relativeTimeRange": {"from": "15m", "to": "0s"},
                    },
                    "B": {
                        "datasourceUID": "__expr__",
                        "model": {
                            "datasource": {"type": "__expr__", "uid": "__expr__"},
                            "expression": "A",
                            "refId": "B",
                            "type": "reduce",
                        },
                    },
                },
            },
        }
        live = {
            "metadata": metadata,
            "spec": {
                "for": "5m0s",
                "expressions": {
                    "A": {
                        "datasourceUID": "grafanacloud-prom",
                        "model": {
                            "datasource": {"type": "prometheus", "uid": "grafanacloud-prom"},
                            "expr": "up",
                            "intervalMs": 1000,
                            "maxDataPoints": 43200,
                            "refId": "A",
                        },
                        "relativeTimeRange": {"from": "15m0s", "to": "0s"},
                    },
                    "B": {
                        "model": {
                            "datasource": {"type": "__expr__", "uid": "__expr__"},
                            "expression": "A",
                            "intervalMs": 1000,
                            "maxDataPoints": 43200,
                            "refId": "B",
                            "type": "reduce",
                        },
                    },
                },
            },
        }

        self.assertEqual(verify.semantic_rule(expected), verify.semantic_rule(live))

    def test_semantic_projection_retains_nondefault_model_limits(self) -> None:
        verify = load_script("grafana-verify-rules.py")
        expected = {
            "A": {
                "model": {
                    "datasource": {"uid": "grafanacloud-prom"},
                    "intervalMs": 1000,
                    "maxDataPoints": 43200,
                }
            }
        }
        changed_interval = {
            "A": {
                "model": {
                    "datasource": {"uid": "grafanacloud-prom"},
                    "intervalMs": 5000,
                    "maxDataPoints": 43200,
                }
            }
        }
        changed_points = {
            "A": {
                "model": {
                    "datasource": {"uid": "grafanacloud-prom"},
                    "intervalMs": 1000,
                    "maxDataPoints": 1000,
                }
            }
        }

        self.assertNotEqual(
            verify.canonical_expressions(expected),
            verify.canonical_expressions(changed_interval),
        )
        self.assertNotEqual(
            verify.canonical_expressions(expected),
            verify.canonical_expressions(changed_points),
        )

    def test_compare_rules_reports_semantic_drift_and_stale_extras(self) -> None:
        verify = load_script("grafana-verify-rules.py")

        expected = {
            ("AlertRule", "rfc-rule"): {
                "kind": "AlertRule",
                "metadata": {"annotations": {"grafana.app/folder": "rfc-folder"}},
                "spec": {"title": "Expected", "expressions": [{"ref": "A"}]},
            }
        }
        live = [
            {
                "kind": "AlertRule",
                "metadata": {
                    "name": "rfc-rule",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
                "spec": {"title": "Expected", "expressions": [{"ref": "B"}]},
            },
            {
                "kind": "AlertRule",
                "metadata": {
                    "name": "stale-rule",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
                "spec": {"title": "Stale", "expressions": []},
            },
            {
                "kind": "RecordingRule",
                "metadata": {
                    "name": "rfc-rule",
                    "annotations": {"grafana.app/folder": "rfc-folder"},
                },
                "spec": {"title": "Same name, wrong kind", "expressions": []},
            },
        ]

        self.assertEqual(
            verify.rule_mismatches(expected, live, "rfc-folder"),
            [
                "semantic drift: AlertRule/rfc-rule",
                "stale live rule: AlertRule/stale-rule",
                "stale live rule: RecordingRule/rfc-rule",
            ],
        )


class GitSyncProjectionTests(unittest.TestCase):
    """A different Git blob or manager must fail the dashboard read-back."""

    def test_gitsync_mismatches_requires_annotations_checksum_and_folder(self) -> None:
        gitsync = load_script("verify-gitsync.py")

        resource = {
            "metadata": {
                "annotations": {
                    "grafana.app/managedBy": "repo",
                    "grafana.app/managerId": "manager-123",
                    "grafana.app/sourcePath": "networking/rfc6035-2otel.json",
                    "grafana.app/sourceChecksum": "blob-sha",
                    "grafana.app/folder": "networking",
                }
            }
        }

        self.assertEqual(
            gitsync.gitsync_mismatches(
                resource,
                manager_id="manager-123",
                source_path="networking/rfc6035-2otel.json",
                blob_sha="blob-sha",
                folder_uid="networking",
            ),
            [],
        )
        self.assertEqual(
            gitsync.gitsync_mismatches(
                resource,
                manager_id="manager-999",
                source_path="networking/rfc6035-2otel.json",
                blob_sha="different-blob",
                folder_uid="other-folder",
            ),
            [
                "manager mismatch: expected manager-999, got manager-123",
                "source checksum mismatch: expected different-blob, got blob-sha",
                "folder mismatch: expected other-folder, got networking",
            ],
        )

    def test_repository_mismatches_requires_last_ref_to_reach_pushed_commit(self) -> None:
        gitsync = load_script("verify-gitsync.py")

        repository = {
            "metadata": {"name": "repository-manager-123"},
            "status": {"sync": {"lastRef": "pushed-commit"}},
        }

        self.assertEqual(
            gitsync.repository_mismatches(repository, "repository-manager-123", "pushed-commit"),
            [],
        )
        self.assertEqual(
            gitsync.repository_mismatches(repository, "repository-manager-123", "different-commit"),
            ["repository lastRef mismatch: expected different-commit, got pushed-commit"],
        )


if __name__ == "__main__":
    unittest.main()
