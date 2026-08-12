"""Deterministic Grafana artefact helpers using only the standard library."""

from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / "spec" / "signal-catalog.json"


def catalog() -> dict:
    return json.loads(CATALOG.read_text(encoding="utf-8"))


def dump_json(value: dict) -> str:
    return json.dumps(value, indent=2, sort_keys=True) + "\n"


def write_or_check(path: Path, content: str, check: bool) -> None:
    if check:
        if not path.exists() or path.read_text(encoding="utf-8") != content:
            raise SystemExit(f"generated artefact drift: {path.relative_to(ROOT)}")
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
