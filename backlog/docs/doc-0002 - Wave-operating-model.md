---
id: doc-0002
title: Wave operating model
type: guide
created_date: '2026-08-14 16:31'
updated_date: '2026-08-14 16:32'
---
This project's own campaign rules. The model itself — run contract, routing, lane briefs, goal-file
template, pre-flight checklist — is the fan-out protocol doc, and nothing here restates it. If a
section below could be pasted into another repo unchanged, it is in the wrong document.

## Rules this project added, each with the failure that caused it

**The public signal contract is frozen and a release cannot rewrite it.** Wave 2 shipped v0.1.0, then
live translation exposed that an explicit OTel gauge unit of `1` on build-info made the SDK emit
`rfc6035_2otel_build_info_ratio` rather than `rfc6035_2otel_build_info`. v0.1.0 was **not** rewritten;
the fix went out as an additive v0.1.1 (#18). A lane that finds a wrong metric name, unit, attribute
or log field in a published release proposes the next patch, never a retag.

**A metric or log field is not proven by a unit test. It is proven by querying the live stack.** The
same wave's unit tests passed on the build-info gauge that was wrong in Grafana. Any lane touching
`internal/otelexport`, `spec/signal-catalog.json` or the dashboards closes with a real query against
the configured Grafana Cloud stack — the positive and the negative — and pastes the rows.

**Do not divide by 100, do not invent a unit.** `NLR` and `JDR` arrive as percent and are exported as
UCUM `%` exactly as received. RFC 3611's `127` sentinel is exposed as unavailable/nil on the typed
field while the literal `127` survives in the lossless log fields. Both were deliberate Wave 1 calls;
a lane that "normalises" either is reversing a decision, not fixing a bug.

**`internal/vqreport` owns the dialect enum, alone.** The exporter passes the parser-owned value
through. This exists so adding a dialect is one file, not N+1 — it was already broken once and fixed
in `8763e9c`.

**Fixtures are labelled real or synthetic, and the label is load-bearing.** Everything shipped so far
derives from the RFC 6035 / RFC 3611 examples plus measured Poly output, not from a wire capture of an
arbitrary sender. A lane must not quietly promote a synthetic fixture to evidence of what a device
emits. This is the entire subject of VQR-0001.

**Wrong-stream evidence is the recurring way this project fools itself.** A 2026-08-13 investigation
concluded a whole dialect was dead because it queried `{service_name="syslog"} | appname="vqcollector"`
in Loki — the *retired* `vq-collector`'s stream. `rfc6035-2otel` exports OTLP directly and writes
nothing there, so the empty result was guaranteed and meant nothing. Before a lane reports a negative
from telemetry, it states which stream it queried and why that stream would carry the signal if it
existed. Check `docker logs rfc6035-2otel` or the OTLP data.

**Deployment inputs that live only on the host are a standing hazard.** The live soak failed once
because Camden's `senders:` mapping block, required by the cardinality contract, was simply absent
from the live config — two reports landed as `unknown`. There is still no durable source of truth for
that mapping. A lane that depends on it verifies it is present before drawing conclusions.

## Recurring defects in this codebase, with instances

- **golangci-lint is CI-only and `make check` does not run it.** Five genuine first-run findings
  failed CI four separate times in Wave 2 (`31633961591`, `31633987259`, `31634513277`,
  `31634636711`) before `fda5313` cleared them. Run `golangci-lint run ./...` locally before pushing;
  a green `make check` is not a green CI.
- **The `make` target a workflow calls may not exist.** Helm failed three times (`31633961557`,
  `31634509492`, `31634636761`) purely because `make helm-docs` had not been written yet. When adding
  a workflow step that shells into `make`, add the target in the same change.
- **Test-side timing races, not production races.** The SIP observer race surfaced in release-PR CI
  was fixed by giving the *assertions* a bounded wait (#17). Production code was not serialised.
  Reach for the bounded wait before restructuring the observer.
- **Transient `curl 56` / HTTP 503 on tool downloads** (actionlint `31633962098`, cosign `31637220064`
  and `31640533952`) is infrastructure, not a finding. Rerun once, and say in the task notes that you
  did — do not "fix" a lint config in response to a truncated gzip.
- **Release Please proposed 1.0.0 against a `0.0.0` manifest.** Resolved with `initial-version: 0.1.0`
  in config; the manifest was not touched. Releases stay pre-1.0 until both frozen conditions hold: a
  non-Poly RFC 6035 sender observed, and the pre-standard grammar seen from a second Poly model or
  firmware. Neither has happened.

## Lane conventions and exclusive resources

Ownership is per-package, and these are the seams: `internal/sip` (listener), `internal/vqreport`
(parser and the dialect enum), `internal/otelexport` (signal contract), `internal/selfobs`,
`internal/config`, `cmd/rfc6035-2otel` (wiring, single owner), `grafana/` + `dashboards/` + `alerts/`
(generated — never hand-edit the outputs, edit the builders and run `make dashboard` / `make rules`),
`charts/` and `deploy/`, `.github/workflows/`, `docs/` + `spec/`.

**Three exclusive resources. One lane at a time, named in the goal, and never two in a wave:**

1. **The two live handsets and the Camden deployment.** Physical; a second lane placing calls
   invalidates the first lane's capture window.
2. **The Grafana Cloud stack.** Writes to dashboards, rules or folders are mutations of a shared live
   system. Reads are unrestricted.
3. **Placing real calls.** Call origination is a *separately granted* authority, not implied by having
   the handsets in scope — Wave 2's P2-L8 parked on exactly this and resumed only after Rob granted
   it explicitly on 2026-08-13. A lane without that grant in its brief reads only.

## Ownership and the escape hatch

A lane owns its packages exclusively and does not edit another lane's files, including to fix an
obvious one-line break. **The escape hatch:** when the fix is genuinely in another lane's file, the
lane records it in its task notes with the exact `file:line` and the proposed change, and continues on
its own work. The wiring pass applies it. A boundary with no escape hatch is a stop condition wearing a
safety label.

`cmd/rfc6035-2otel` and any cross-cutting rename are the wiring pass's, never a parallel lane's.

**A second agent must never attach to a running lane.** One did in Wave 2, resumed the Grafana lane,
and was interrupted; sole ownership had to reconcile the whole diff by hand. It committed nothing, so
the recovery was cheap — the next one may not be.

## Run-end against this tracker

Landed work is `Done` with the landing SHA in its final summary, and the CI run id that was green at
that exact SHA — this project has twice shipped something whose CI was green at a *different* head.
Parked work is `Parked` with the concrete resume boundary, in the shape Wave 2 proved works: what was
done, what the next actor must be granted, and what is already true so it resumes rather than restarts.
Discovered work is a new task labelled `needs-triage`. Nothing durable may live only in the closing
terminal message.

Closed GitHub issues #2–#22 are indexed in the closed-work index doc; they are the record of Waves 1–3
and are not re-imported as tasks.
