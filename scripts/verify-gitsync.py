#!/usr/bin/env python3
"""Verify Grafana has reconciled a GitSync dashboard revision."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from typing import Any


MANAGED_BY_KEY = "grafana.app/managedBy"
MANAGER_KEY = "grafana.app/managerId"
SOURCE_PATH_KEY = "grafana.app/sourcePath"
SOURCE_CHECKSUM_KEY = "grafana.app/sourceChecksum"
FOLDER_KEY = "grafana.app/folder"


def annotations(resource: dict[str, Any]) -> dict[str, Any]:
    return resource.get("metadata", {}).get("annotations", {})


def gitsync_mismatches(
    resource: dict[str, Any], *, manager_id: str, source_path: str, blob_sha: str,
    folder_uid: str,
) -> list[str]:
    """Return all read-back discrepancies for the immutable GitSync identity."""
    actual = annotations(resource)
    expected = (
        (MANAGED_BY_KEY, "repo", "managed-by"),
        (MANAGER_KEY, manager_id, "manager"),
        (SOURCE_PATH_KEY, source_path, "source path"),
        (SOURCE_CHECKSUM_KEY, blob_sha, "source checksum"),
        (FOLDER_KEY, folder_uid, "folder"),
    )
    return [
        f"{label} mismatch: expected {wanted}, got {actual.get(key, 'missing')}"
        for key, wanted, label in expected
        if actual.get(key) != wanted
    ]


def repository_mismatches(
    repository: dict[str, Any], manager_id: str, commit_sha: str
) -> list[str]:
    """Require the named GitSync repository to have pulled the pushed commit."""
    name = repository.get("metadata", {}).get("name")
    last_ref = repository.get("status", {}).get("sync", {}).get("lastRef")
    mismatches: list[str] = []
    if name != manager_id:
        mismatches.append(f"repository manager mismatch: expected {manager_id}, got {name or 'missing'}")
    if last_ref != commit_sha:
        mismatches.append(
            f"repository lastRef mismatch: expected {commit_sha}, got {last_ref or 'missing'}"
        )
    return mismatches


def gcx_command(args: argparse.Namespace, *parts: str) -> list[str]:
    return [args.gcx, *parts, "--context", args.context]


def resource_items(document: Any) -> list[dict[str, Any]]:
    if isinstance(document, dict) and isinstance(document.get("items"), list):
        return [item for item in document["items"] if isinstance(item, dict)]
    raise ValueError("gcx repository output did not contain a resource list")


def gcx_json(args: argparse.Namespace, *parts: str) -> Any:
    completed = subprocess.run(
        gcx_command(args, *parts, "-o", "json"),
        check=True,
        text=True,
        stdout=subprocess.PIPE,
    )
    return json.loads(completed.stdout)


def live_dashboard(args: argparse.Namespace) -> dict[str, Any]:
    resources = resource_items(
        gcx_json(args, "resources", "get", f"dashboards/{args.resource_name}")
    )
    if len(resources) != 1:
        raise ValueError(f"expected one dashboard resource, got {len(resources)}")
    return resources[0]


def live_repository(args: argparse.Namespace) -> dict[str, Any]:
    repositories = resource_items(
        gcx_json(args, "resources", "get", "repositories", "--limit", "0")
    )
    matched = [
        repository
        for repository in repositories
        if repository.get("metadata", {}).get("name") == args.manager_id
    ]
    if len(matched) != 1:
        raise ValueError(f"expected one GitSync repository {args.manager_id}, got {len(matched)}")
    return matched[0]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--resource-name", default="rfc6035-2otel")
    parser.add_argument("--manager-id", required=True)
    parser.add_argument("--source-path", required=True)
    parser.add_argument("--blob-sha", required=True)
    parser.add_argument("--commit-sha", required=True)
    parser.add_argument("--folder-uid", required=True)
    parser.add_argument("--context", required=True)
    parser.add_argument("--gcx", default="gcx")
    parser.add_argument("--retries", type=int, default=12)
    parser.add_argument("--retry-seconds", type=float, default=5.0)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.retries < 1 or args.retry_seconds < 0:
        print("retries must be positive and retry-seconds must not be negative", file=sys.stderr)
        return 1

    mismatches: list[str] = []
    try:
        for attempt in range(args.retries):
            mismatches = repository_mismatches(
                live_repository(args), args.manager_id, args.commit_sha
            )
            if not mismatches:
                mismatches = gitsync_mismatches(
                    live_dashboard(args),
                    manager_id=args.manager_id,
                    source_path=args.source_path,
                    blob_sha=args.blob_sha,
                    folder_uid=args.folder_uid,
                )
            if not mismatches:
                return 0
            if attempt + 1 < args.retries:
                time.sleep(args.retry_seconds)
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError, ValueError) as error:
        print(f"GitSync verification failed: {error}", file=sys.stderr)
        return 1

    print("GitSync verification failed:", file=sys.stderr)
    print(*mismatches, sep="\n", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
