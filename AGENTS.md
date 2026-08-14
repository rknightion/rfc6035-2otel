# rfc6035-2otel — contributor and agent instructions

Claude Code and Codex both read this file; `CLAUDE.md` is a one-line import of it, so the two cannot
drift apart. Everything below the tool-managed marker block at the end is hand-written and survives
`backlog init` re-runs.

An RFC 6035 SIP voice-quality (`vq-rtcpxr`) collector: it listens for SIP PUBLISH, parses the report
body in both the standard and the measured Poly pre-standard dialect, and exports OpenTelemetry
metrics and logs.

## The gate

```bash
make check                # fmt-check vet test tidy-check grafana-check build
golangci-lint run ./...   # CI runs this and `make check` does NOT — a green make is not a green CI
```

CI additionally runs `govulncheck ./...`, a 10s parser fuzz smoke, a GoReleaser snapshot build and a
Docker image build. `ci-success` is the required check. Both gate commands are the tracker's
`definition_of_done`, so every task inherits them.

Generated artefacts are never hand-edited: `dashboards/`, `alerts/` and `spec/signal-catalog.json`
come from the builders under `grafana/`. Edit the builder, then `make dashboard` / `make rules`.
`make grafana-check` fails if the committed output does not match.

## Task tracking

Open work lives in Backlog.md, in `backlog/`, committed to git. The queue is a query, not a file:

```bash
backlog task list --plain          # what is left
backlog doc list --plain           # the durable docs
backlog task view VQR-0001 --plain # a task's own contract, including its acceptance criteria
```

Read the **fan-out protocol** doc before designing a wave, and the **Wave operating model** doc for
this project's own rules — its frozen contracts, its recurring defects, its exclusive resources and
its ownership escape hatch. The **closed work index** doc maps every pre-migration GitHub issue
(#2–#22) to its resulting SHA; closed issues were deliberately not re-imported as tasks.

### Rules with no exceptions

**`backlog/` is committed to git, so tasks and docs must never contain real account identifiers or
personal data** — no phone numbers, SIP URIs, MAC addresses, host or handset names, lab IP addresses,
handset serials, Grafana stack hostnames or ids, tenant or account IDs, credentials, or capture
payloads containing them. Write the shape, not the instance: "the second handset", "the collector
host", `<dialect>/<sender>/<report>`. Aggregate counts, timings, metric names, CI run ids, commit
SHAs and structural findings are fine, and so are SIP `Call-ID` values — they are per-call hashes
that identify nothing.

The same placeholder vocabulary as `archive/README.md` applies, so the two agree: `HOST-A`,
`PHONE-A`, `PHONE-B` — that table says what each denotes without naming it, which is why the pattern
below matches address and hostname *shapes* rather than listing the real names. Deliberately so:
a sweep that spells out the identifiers it is hunting for plants them permanently in a tracked file,
which is the leak it exists to prevent. The host and handset names are the one class you must check
by eye.

```bash
grep -rniE '10\.0\.[0-9]|100\.(6[4-9]|[7-9][0-9])\.|grafana\.[a-z0-9-]+\.(com|net)|@gmail|sip:[0-9]{6,}|([0-9a-f]{2}:){5}[0-9a-f]{2}' backlog/ \
  && echo "REVIEW EACH HIT"
```

**Never use `--notes` or `--plan` bare.** They *silently replace* the whole section — another
session's writes vanish with no warning at exit 0. Use `--append-notes` and `--append-plan`. This is
an open upstream bug, not a misunderstanding, and a global `PreToolUse` hook in the agent config denies the bare
form rather than trusting anyone to remember.

**Never hand-edit task, draft, doc, decision or milestone markdown.** Section boundaries are
HTML-comment markers; break one and the section is *silently dropped* at exit 0, with the data still
in the file but invisible until the next write destroys it for real. There is no repair command —
`backlog doctor` only fixes duplicate task IDs. The same hook denies these edits.
`backlog/config.yml` is the one exception and is edited by hand, because list-valued keys cannot be
set through `backlog config set`.

**Never let two agents edit the same task.** v1.50.x fixed the edit funnel, but not reorder, draft
saves, the TUI path, `doc update` or decision updates.

**Finalize in one call**, so an interrupted agent cannot leave finished work looking unfinished:

```bash
backlog task edit VQR-0001 --check-ac 1 --check-ac 2 -s Done
```

The shipped guides check criteria at one step and set status several steps later; a context limit
between the two leaves the task inconsistent.

**Do not build a workflow on `decisions`** — half-built upstream, with no `edit`, `view` or supersede
mechanism. Durable reference goes in docs; tasks are the unit.

### Git

This is `rknightion/rfc6035-2otel`, not a fork: completed work is committed and pushed straight to
`main`. Never `git add -A` or `git commit -a` in a checkout carrying changes that are not yours —
stage explicit pathspecs. `codex/` and `docs/superpowers/` are gitignored run scaffolding and never
enter history.

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
