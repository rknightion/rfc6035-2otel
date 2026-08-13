#!/usr/bin/env python3
"""Prune stale Grafana alert and recording rules from one resolved folder."""

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
    """Return the Grafana folder, preferring its canonical annotation."""
    metadata = resource.get("metadata", {})
    annotations = metadata.get("annotations", {})
    labels = metadata.get("labels", {})
    return annotations.get(FOLDER_KEY) or labels.get(FOLDER_KEY)


def stale_resource_names(
    resources: Iterable[dict[str, Any]],
    expected_resources: set[tuple[str, str]],
    target_folder_uid: str,
) -> list[tuple[str, str]]:
    """Select only stale AlertRule/RecordingRule identities in the target folder."""
    if not expected_resources:
        raise ValueError("expected rule names must not be empty")

    stale: list[tuple[str, str]] = []
    for resource in resources:
        kind = resource.get("kind")
        name = resource.get("metadata", {}).get("name")
        if (
            kind in {"AlertRule", "RecordingRule"}
            and isinstance(name, str)
            and folder_uid(resource) == target_folder_uid
            and (kind, name) not in expected_resources
        ):
            stale.append((kind, name))
    return sorted(stale)


def resource_items(document: Any) -> list[dict[str, Any]]:
    """Accept gcx list output as either an item list or a collection object."""
    if isinstance(document, list):
        return [item for item in document if isinstance(item, dict)]
    if isinstance(document, dict) and isinstance(document.get("items"), list):
        return [item for item in document["items"] if isinstance(item, dict)]
    raise ValueError("gcx list output did not contain a resource list")


def expected_rule_resources(path: Path) -> set[tuple[str, str]]:
    """Parse non-empty ``AlertRule/name`` or ``RecordingRule/name`` keep lines."""
    expected: set[tuple[str, str]] = set()
    allowed_kinds = {kind for _, kind in COLLECTIONS}
    for line_number, raw_line in enumerate(path.read_text().splitlines(), start=1):
        line = raw_line.strip()
        if not line:
            continue
        kind, separator, name = line.partition("/")
        if not separator or kind not in allowed_kinds or not name or "/" in name:
            raise ValueError(
                f"expected rule identities must use AlertRule/name or RecordingRule/name (line {line_number})"
            )
        expected.add((kind, name))
    if not expected:
        raise ValueError("expected rule identities must not be empty")
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
    parser.add_argument("--expected-names", type=Path, required=True)
    parser.add_argument("--folder-uid", required=True)
    parser.add_argument("--context", required=True)
    parser.add_argument("--gcx", default="gcx")
    parser.add_argument("--dry-run", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        expected = expected_rule_resources(args.expected_names)
        selectors = [collection for collection, _ in COLLECTIONS]
        resources = resource_items(
            gcx_json(args, "resources", "get", *selectors, "--limit", "0")
        )
        stale = stale_resource_names(resources, expected, args.folder_uid)
        for kind, name in stale:
            collection = next(collection for collection, candidate in COLLECTIONS if candidate == kind)
            target = f"{collection}/{name}"
            if args.dry_run:
                print(f"would delete {kind}/{name}")
            else:
                subprocess.run(
                    gcx_command(args, "resources", "delete", target), check=True
                )
                print(f"deleted {kind}/{name}")
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError, ValueError) as error:
        print(f"grafana rule pruning failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
