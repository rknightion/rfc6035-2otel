# rfc6035-2otel

An RFC 6035 SIP voice-quality (`vq-rtcpxr`) collector: it listens for SIP PUBLISH, parses the report
body in both the standard dialect and the measured Poly pre-standard dialect, and exports
OpenTelemetry metrics and logs.

## Task interface

`just check` is the gate: `fmt-check`, `lint`, `vet`, `test`, `tidy-check`, `gen-check`, `build`,
`vuln`, `fuzz`. It is CI's `build-test` job verbatim and the tracker's `definition_of_done`, so every
task inherits it. `just ci` is the sanctioned superset, adding `snapshot` (cross-compilation) and
`image` (a Docker daemon); CI runs those two in separate parallel jobs.

Run `just` with stdin from `/dev/null`. No recipe is marked `[confirm]`.

`dashboards/` and `alerts/` are generated from the builders under `grafana/` and are never
hand-edited, and so is the region between `<!-- BEGIN/END GENERATED SIGNAL CATALOG -->` in
`docs/signals.md`. `spec/signal-catalog.json` is the hand-maintained **input** those builders read:
edit the catalogue or the builder, then `just gen`. `just gen-check` fails on committed drift.

## Tracker

Tasks are `VQR-NNNN` in `backlog/`. Read the **Agent fan-out protocol (canonical)** doc before
designing a wave, and the **Wave operating model** doc for this repo's own rules - its frozen
contracts, its recurring defects, its exclusive resources and its ownership escape hatch. The
**Closed GitHub issues (pre-Backlog history index)** doc maps each pre-migration issue to its
resulting SHA; closed issues were deliberately not re-imported as tasks.

`backlog task view VQR-0001 --plain` prints a task's own contract including its acceptance criteria.

### Rules with no exceptions

**`backlog/` is committed to a public repository**, so no real account identifier or personal data
goes in a task or doc: no phone numbers, SIP URIs, MAC addresses, host or handset names, lab IP
addresses, handset serials, Grafana stack hostnames or ids, tenant or account IDs, credentials, or
capture payloads containing any of those. Write the shape, not the instance: "the second handset",
"the collector host", `<dialect>/<sender>/<report>`. Aggregate counts, timings, metric names, CI run
ids, commit SHAs and structural findings are fine, and so are SIP `Call-ID` values - they are
per-call hashes that identify nothing.

Use the same placeholder vocabulary as `archive/README.md` (`HOST-A`, `PHONE-A`, `PHONE-B`), so the
two agree. The sweep below matches address and hostname *shapes* rather than listing the real names
deliberately: a sweep that spells out the identifiers it hunts for plants them permanently in a
tracked file, which is the leak it exists to prevent. Host and handset names are the one class you
must check by eye.

```bash
grep -rniE '10\.0\.[0-9]|100\.(6[4-9]|[7-9][0-9])\.|grafana\.[a-z0-9-]+\.(com|net)|@gmail|sip:[0-9]{6,}|([0-9a-f]{2}:){5}[0-9a-f]{2}' backlog/ \
  && echo "REVIEW EACH HIT"
```

**Never use `--notes` or `--plan` bare.** They *silently replace* the whole section - another
session's writes vanish with no warning at exit 0. Use `--append-notes` and `--append-plan`. A
`PreToolUse` hook denies the bare form rather than trusting anyone to remember.

**Never hand-edit task, draft, doc, decision or milestone markdown.** Section boundaries are
HTML-comment markers; break one and the section is *silently dropped* at exit 0, with the data still
in the file but invisible until the next write destroys it for real. There is no repair command -
`backlog doctor` only fixes duplicate task IDs. The same hook denies these edits.
`backlog/config.yml` is the one exception and is edited by hand, because list-valued keys cannot be
set through `backlog config set`.

**Never let two agents edit the same task.** v1.50.x fixed the `task edit` funnel, but not reorder,
draft saves, the TUI path, `doc update` or decision updates.

**Finalize in one call**, so an interrupted agent cannot leave finished work looking unfinished:

```bash
backlog task edit VQR-0001 --check-ac 1 --check-ac 2 -s Done
```

The shipped guides check criteria at one step and set status several steps later; a context limit
between the two leaves the task inconsistent.

**Do not build a workflow on `backlog decision`** - half-built upstream, with no `edit`, `view` or
supersede mechanism. Durable reference goes in docs; tasks are the unit.

The `<!-- BACKLOG.MD GUIDELINES -->` block at the foot of this file is written by `backlog init` and
silently returns if deleted. Everything above it is hand-written and survives a re-run.

## Git

Stage explicit pathspecs. Never `git add -A` or `git commit -a` in a checkout carrying changes that
are not yours - parallel lanes share this working tree. `codex/` and `docs/superpowers/` are ignored
run scaffolding and never enter history.

<!-- BACKLOG.MD GUIDELINES START -->
<!-- backlog.md-instructions-version: 1.50.1 -->
<CRITICAL_INSTRUCTION>

## Backlog.md Workflow

This project uses Backlog.md for task and project management.

**For every user request in this project, run `backlog instructions overview` before answering or taking action.**

Use the overview to decide whether to search, read, create, or update Backlog tasks.

Before task lifecycle actions, read the matching detailed guide:
- `backlog instructions task-creation` before creating or splitting tasks
- `backlog instructions task-execution` before planning, changing status or assignee, adding a plan or implementation notes, or implementing task work
- `backlog instructions task-finalization` before checking acceptance criteria, writing final summaries, or moving tasks to terminal statuses

Use `backlog <command> --help` before running unfamiliar commands. Help shows options, fields, and examples.

Do not edit Backlog task, draft, document, decision, or milestone markdown files directly. Use the `backlog` CLI so metadata, relationships, and history stay consistent.

</CRITICAL_INSTRUCTION>
<!-- BACKLOG.MD GUIDELINES END -->
