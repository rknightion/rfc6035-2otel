---
id: doc-0003
title: 'Closed work index (GitHub issues #2-#22)'
type: other
created_date: '2026-08-14 16:33'
updated_date: '2026-08-14 16:59'
---
Every GitHub issue closed before the tracker migration on 2026-08-14 — Waves 1 through 3 and the
release-repair issues between them. **17 closed issues, verified with `gh issue list --state closed
--limit 1000`** (the default limit is 30; the count here is the one actually returned).

These are **not** re-imported as backlog tasks, deliberately. Backlog IDs follow creation order, so a
`VQR-00NN` could never be made to match the `#NN` already cited in commit messages, wave reports and
issue cross-references — importing them would create a second ID space over the same history, and 17
`Done` rows would compete with the board's only real signal, which is what is left.

**The issues themselves were deleted from GitHub on 2026-08-14.** `gh issue view <N>` no longer
resolves and the `#NN` URLs 404. The bodies and all 26 comments live in `archive/issues.json`,
committed and pushed before the deletion, with lab identifiers replaced by stable placeholders —
`archive/README.md` carries the mapping and the verification:

```bash
jq -r '.[] | select(.number==11) | .title, "", .body' archive/issues.json
```

The full narrative — including every non-green CI run and its disposition — is in the gitignored
`codex/report-20260812-wave1.md` and `codex/report-20260812-wave2.md`.

Numbers absent from this table (#1, #14, #19, #23) are pull requests, not issues and not archived
here. #16 was Renovate's Dependency Dashboard, bot-owned and never a task; it was deleted with the
rest and Renovate will recreate it at a new number on its next run. #20 was the only open issue and
is now **VQR-0001** on the board — it was deleted too, so the task is the live record and the archive
is the original.

Resulting SHAs are given where they are derivable. Commit messages in this repo do **not** cite issue
numbers, so the mapping below is by content — file-addition history and the wave reports — not by an
explicit `Closes #N`. Where a lane's work was squashed into the Phase 1 integration commit, that is
said rather than guessed at.

| # | Title | Closed | Resulting SHA(s) |
|---|---|---|---|
| 2 | Build and deploy first working RFC 6035 to OTLP release | 2026-08-12 | `25f9b4d`, `8763e9c`, `ce49568`, `206c6f1` (all of Wave 1) |
| 3 | Wave 2: sibling parity, public signal contract, and v0.1.0 | 2026-08-13 | umbrella; released at `bd0168b` (v0.1.0) |
| 4 | P1-L1: Correct the public metric and log contract | 2026-08-12 | `d05f2d7` |
| 5 | P1-L2: Add bounded collector self-observability | 2026-08-12 | `4d264c6` |
| 6 | P1-L3: Add release-please and GoReleaser plumbing | 2026-08-12 | `3bcb027`, `973853c` (`initial-version: 0.1.0`) |
| 7 | P1-L4: Port and harden the complete workflow set | 2026-08-12 | `efb5247` |
| 8 | P1-L5: Write operator documentation and community files | 2026-08-12 | `fda5313` (docs, `CONTRIBUTING.md`, `SECURITY.md` first appear there) |
| 9 | P1-L6: Generate the signal catalog, dashboard, and alerts | 2026-08-12 | `fda5313` (`spec/signal-catalog.json`, `dashboards/`, `alerts/` first appear there) |
| 10 | P1-L7: Add the community Helm chart | 2026-08-12 | `fceae20` |
| 11 | P2-L8: Prove both dialects and deduplication in a live soak | 2026-08-13 | `4df6e82` |
| 12 | P2-L9: Prove end-to-end docs hub publishing | 2026-08-12 | `a9521cb` |
| 13 | P2-L10: Fuzz the parser from real and RFC seeds | 2026-08-12 | `c252715` |
| 15 | GATE integration: reconcile Phase 1 and first-run CI | 2026-08-12 | `fda5313` |
| 17 | test: synchronize SIP observer assertions | 2026-08-12 | `8921f62` |
| 18 | fix: preserve the frozen build-info Prometheus series name | 2026-08-12 | `1259341`, released `f64160d` (v0.1.1) |
| 21 | Wave 3: operational Dashboard v2 and Grafana reconciliation | 2026-08-13 | `951cc2d`, `ccd0d60`, `fae5290`, `66935b3`, `23731cb`; released `c0e08eb` (v0.2.0) |
| 22 | Design: assess binary RFC 3611 RTCP-XR receiver support | 2026-08-13 | design assessment, no implementing commit |

## The two facts from this history a future session will otherwise re-derive

**The build-info defect (#18) is why the signal contract is treated as frozen.** An explicit OTel
gauge unit of `1` made the SDK emit `rfc6035_2otel_build_info_ratio`. v0.1.0 was not retagged; the fix
shipped additively as v0.1.1.

**1.0.0 is blocked on two observations, neither of which has happened:** a non-Poly RFC 6035 sender
ingested, and the pre-standard grammar seen from a second Poly model or firmware. Recorded at the
close of Wave 2. Do not plan a 1.0.0 wave until both are observable.
