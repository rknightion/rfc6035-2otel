"""Permanent RFC 6035 dashboard panel identities.

Alert rules refer to panel IDs, so these values are a public compatibility
contract rather than layout implementation detail.  Add entries; do not reuse
or renumber an existing ID.
"""

from __future__ import annotations


def _validated_registry(entries: list[tuple[str, int, str, str]]) -> dict[str, tuple[int, str, str]]:
    registry: dict[str, tuple[int, str, str]] = {}
    ids: set[int] = set()
    for key, panel_id, title, tab in entries:
        if key in registry:
            raise ValueError(f"duplicate panel semantic key: {key}")
        if panel_id in ids:
            raise ValueError(f"duplicate panel ID: {panel_id}")
        registry[key] = (panel_id, title, tab)
        ids.add(panel_id)
    return registry


PANEL_REGISTRY = _validated_registry([
    ("reports-exact", 101, "exact reports in selected range", "Operations"),
    ("reporting-handsets", 102, "reporting handsets", "Operations"),
    ("mos-lq-mean", 103, "mean MOS LQ", "Operations"),
    ("mos-lq-p95", 104, "p95 MOS LQ", "Operations"),
    ("packet-loss-p95", 105, "p95 packet loss", "Operations"),
    ("unknown-sender-reports", 106, "unknown-sender reports", "Operations"),
    ("export-failures", 107, "export failures", "Operations"),
    ("rtt-p95", 108, "p95 RTT", "Operations"),
    ("jitter-p95", 109, "p95 jitter", "Operations"),
    ("handset-quality", 110, "per-handset quality", "Operations"),
    ("mos-lq-trend", 201, "MOS LQ mean and p95", "Call quality"),
    ("mos-cq-trend", 202, "MOS CQ mean and p95", "Call quality"),
    ("packet-loss-trend", 203, "packet loss p95", "Call quality"),
    ("jitter-trend", 204, "jitter p95 by kind", "Call quality"),
    ("rtt-trend", 205, "RTT p95", "Call quality"),
    ("report-volume", 206, "report volume", "Call quality"),
    ("raw-reports", 301, "raw RFC 6035 reports", "Raw Reports"),
    ("r-factor-lq", 401, "R-factor LQ p95", "Diagnostics"),
    ("r-factor-cq", 402, "R-factor CQ p95", "Diagnostics"),
    ("discard-rate", 403, "discard rate p95", "Diagnostics"),
    ("dialect-breakdown", 404, "reports by dialect", "Diagnostics"),
    ("report-type-breakdown", 405, "reports by type", "Diagnostics"),
    ("endpoint-side-breakdown", 406, "quality by endpoint side", "Diagnostics"),
    ("one-way-delay", 407, "one-way delay p95", "Diagnostics"),
    ("build-info", 501, "build info", "Collector"),
    ("datagrams", 502, "datagrams by outcome", "Collector"),
    ("reports", 503, "reports by dialect, type, and sender", "Collector"),
    ("parse-errors", 504, "parse errors by error type", "Collector"),
    ("duplicates", 505, "duplicates by sender", "Collector"),
    ("responses", 506, "SIP responses by status", "Collector"),
    ("dedupe-cache-usage", 507, "dedupe cache usage", "Collector"),
    ("processing-duration", 508, "report processing duration p95", "Collector"),
    ("collector-export-failures", 509, "export failures by signal", "Collector"),
])
