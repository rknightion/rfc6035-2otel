#!/usr/bin/env python3
"""Compare generated Grafana rule resources with their live semantic state."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any, Iterable


COLLECTIONS = (
    ("alertrules.v0alpha1.rules.alerting.grafana.app", "AlertRule"),
    ("recordingrules.v0alpha1.rules.alerting.grafana.app", "RecordingRule"),
)
FOLDER_KEY = "grafana.app/folder"


def folder_uid(resource: dict[str, Any]) -> str | None:
    metadata = resource.get("metadata", {})
    return metadata.get("annotations", {}).get(FOLDER_KEY) or metadata.get("labels", {}).get(FOLDER_KEY)


def semantic_rule(resource: dict[str, Any]) -> dict[str, Any]:
    """Project fields Grafana must preserve; exclude server-managed metadata."""
    spec = resource.get("spec", {})
    return {
        "folder": folder_uid(resource),
        "title": spec.get("title"),
        "paused": spec.get("paused"),
        "for": spec.get("for"),
        "noDataState": spec.get("noDataState"),
        "execErrState": spec.get("execErrState"),
        "trigger": spec.get("trigger"),
        "labels": spec.get("labels"),
        "annotations": spec.get("annotations"),
        "expressions": spec.get("expressions"),
        "panelRef": spec.get("panelRef"),
    }


def rule_mismatches(
    expected: dict[tuple[str, str], dict[str, Any]],
    live_resources: Iterable[dict[str, Any]],
    target_folder_uid: str,
) -> list[str]:
    """Return missing, drifted and stale rule errors for one folder."""
    if not expected:
        raise ValueError("expected rule resources must not be empty")
    live: dict[tuple[str, str], dict[str, Any]] = {}
    for resource in live_resources:
        kind = resource.get("kind")
        name = resource.get("metadata", {}).get("name")
        if (
            kind in {"AlertRule", "RecordingRule"}
            and isinstance(name, str)
            and folder_uid(resource) == target_folder_uid
        ):
            live[(kind, name)] = resource
    mismatches: list[str] = []
    for kind, name in sorted(expected):
        actual = live.pop((kind, name), None)
        if actual is None:
            mismatches.append(f"missing live rule: {kind}/{name}")
        elif semantic_rule(expected[(kind, name)]) != semantic_rule(actual):
            mismatches.append(f"semantic drift: {kind}/{name}")
    mismatches.extend(f"stale live rule: {kind}/{name}" for kind, name in sorted(live))
    return mismatches


def resource_items(document: Any) -> list[dict[str, Any]]:
    if isinstance(document, list):
        return [item for item in document if isinstance(item, dict)]
    if isinstance(document, dict) and isinstance(document.get("items"), list):
        return [item for item in document["items"] if isinstance(item, dict)]
    raise ValueError("gcx list output did not contain a resource list")


def expected_resources(path: Path, folder_uid_value: str) -> dict[tuple[str, str], dict[str, Any]]:
    expected: dict[tuple[str, str], dict[str, Any]] = {}
    for manifest in sorted(path.glob("*.json")):
        resource = json.loads(manifest.read_text())
        if not isinstance(resource, dict):
            raise ValueError(f"expected object in {manifest}")
        name = resource.get("metadata", {}).get("name")
        kind = resource.get("kind")
        if kind not in {"AlertRule", "RecordingRule"} or not isinstance(name, str) or not name:
            raise ValueError(f"missing AlertRule/RecordingRule identity in {manifest}")
        identity = (kind, name)
        if identity in expected:
            raise ValueError(f"duplicate expected resource identity: {kind}/{name}")
        metadata = resource.setdefault("metadata", {})
        annotations = metadata.setdefault("annotations", {})
        labels = metadata.setdefault("labels", {})
        annotations[FOLDER_KEY] = folder_uid_value
        labels[FOLDER_KEY] = folder_uid_value
        expected[identity] = resource
    if not expected:
        raise ValueError("expected rule resources must not be empty")
    return expected


def gcx_command(args: argparse.Namespace, *parts: str) -> list[str]:
    return [args.gcx, *parts, "--context", args.context]


def gcx_json(args: argparse.Namespace, *parts: str) -> Any:
    completed = subprocess.run(
        gcx_command(args, *parts, "-o", "json"),
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    return json.loads(completed.stdout)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--expected-dir", type=Path, required=True)
    parser.add_argument("--folder-uid", required=True)
    parser.add_argument("--context", required=True)
    parser.add_argument("--gcx", default="gcx")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        expected = expected_resources(args.expected_dir, args.folder_uid)
        selectors = [collection for collection, _ in COLLECTIONS]
        live = resource_items(gcx_json(args, "resources", "get", *selectors, "--limit", "0"))
        mismatches = rule_mismatches(expected, live, args.folder_uid)
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError, ValueError) as error:
        print(f"grafana rule verification failed: {error}", file=sys.stderr)
        return 1
    if mismatches:
        print("grafana rule verification failed:", file=sys.stderr)
        print(*mismatches, sep="\n", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
