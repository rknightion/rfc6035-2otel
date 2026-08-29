---
id: VQR-0003
title: Migrate the repo task surface to just and retire Makefiles and ad-hoc scripts
status: To Do
assignee: []
created_date: '2026-08-28 19:28'
updated_date: '2026-08-29 10:43'
labels:
  - 'wave:2-fleet'
dependencies: []
priority: medium
type: chore
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
# Migrate rfc6035-2otel task surface to `just`

Fleet-wide `just` migration per the frozen standard. This repo is Go + two Python generator programs
(Grafana dashboard/rule builders) + a Helm chart. It has exactly one `Makefile`, zero tracked shell
scripts, and no existing `justfile`.

## 1. Outcome

`rfc6035-2otel` has one top-level `justfile`, no `Makefile`. `just check` is the complete local PR
gate and is exactly what `.github/workflows/ci.yml`'s `build-test` job runs. `just --list` shows every
recipe grouped and documented. The two Grafana Python generators (`grafana/build_dashboard.py`,
`grafana/build_rules.py`) and the three deploy-time Python scripts under `scripts/` stay as real
programs, reachable via `just gen` / `just gen-check`; the deploy scripts stay invoked directly by
`grafana-sync.yml`, untouched (see §10). `AGENTS.md`, `CONTRIBUTING.md`, `README.md` and
`backlog/config.yml` no longer tell anyone to run `make`.

## 2. The complete justfile

Drop this in as `justfile` at the repo root. Every value below (versions, flags, paths) is read from
this repo's actual `Makefile`, `.golangci.yml`, `.goreleaser.yaml`, `ci.yml` and `helm.yml` — verify
`golangci_lint_version`, `helm_docs_version` and the `govulncheck`/`goreleaser` pins are still current
at the time this task is executed (check `.github/workflows/ci.yml` and `helm.yml` again first; they
may have moved since this task was filed).

```just
set shell := ["bash", "-euo", "pipefail", "-c"]

version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
commit := `git rev-parse HEAD 2>/dev/null || echo unknown`
build_date := `date -u +%Y-%m-%dT%H:%M:%SZ`
ldflags := "-s -w -X main.version=" + version + " -X main.commit=" + commit + " -X main.buildDate=" + build_date
tools_dir := justfile_directory() + "/.tools"
chart_dir := "charts/rfc6035-2otel"
golangci_lint_version := "v2.13.2"
helm_docs_version := "v1.14.2"
govulncheck_version := "v1.3.0"
goreleaser_version := "v2.16.0"

# show the task surface
default:
    @just --list

# install repo-local tooling (idempotent, network allowed)
setup:
    go mod download
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{golangci_lint_version}}
    mkdir -p {{tools_dir}}
    test -x {{tools_dir}}/helm-docs || GOBIN={{tools_dir}} go install github.com/norwoodj/helm-docs/cmd/helm-docs@{{helm_docs_version}}

# format Go sources in place
[group('dev')]
fmt:
    gofmt -w $(find . -name '*.go' -not -path './vendor/*')

# verify Go formatting and justfile formatting; never mutates
[group('check')]
[no-exit-message]
fmt-check:
    files="$(gofmt -l $(find . -name '*.go' -not -path './vendor/*'))"; test -z "$files" || { echo "Go files require formatting:"; echo "$files"; exit 1; }
    just --fmt --check

# static analysis via golangci-lint (same config CI enforces)
[group('check')]
[no-exit-message]
lint:
    golangci-lint run ./...

# go vet
[group('check')]
[no-exit-message]
vet:
    go vet ./...

# run the Go test suite with the race detector; filter="Name" narrows via -run
[group('check')]
[no-exit-message]
test filter="":
    if [ -n "{{filter}}" ]; then go test -race -run '{{filter}}' ./...; else go test -race ./...; fi

# verify go.mod/go.sum need no changes
[group('check')]
[no-exit-message]
tidy-check:
    go mod tidy -diff

# apply go mod tidy
[group('dev')]
tidy:
    go mod tidy

# regenerate the Grafana dashboard and alert-rule resources from the signal catalog
[group('gen')]
gen:
    cd grafana && python3 build_dashboard.py
    cd grafana && python3 build_rules.py

# verify generated Grafana resources match their builders, and run the generator unit tests (drift gate)
[group('check')]
[no-exit-message]
gen-check:
    cd grafana && python3 build_dashboard.py --check
    cd grafana && python3 build_rules.py --check
    cd grafana && python3 -m unittest discover -s tests -t . -q
    python3 -m unittest discover -s scripts/tests -v

# compile the binary into bin/
[group('build')]
build:
    go build -trimpath -ldflags "{{ldflags}}" -o bin/rfc6035-2otel ./cmd/rfc6035-2otel

# THE GATE: everything a PR must pass, exactly what ci.yml's build-test job runs
[group('check')]
check: fmt-check lint vet test tidy-check gen-check build

# scan dependencies for known vulnerabilities
[group('check')]
[no-exit-message]
audit:
    go run golang.org/x/vuln/cmd/govulncheck@{{govulncheck_version}} ./...

# run the RFC 6035 report parser fuzz smoke test (10s)
[group('check')]
[no-exit-message]
fuzz:
    go test ./internal/vqreport -run='^$' -fuzz='^FuzzParse$' -fuzztime=10s

# build a snapshot release with goreleaser (no publish/sign/sbom/docker) — local parity with CI
[group('build')]
[no-exit-message]
build-snapshot:
    go run github.com/goreleaser/goreleaser/v2@{{goreleaser_version}} release --snapshot --clean --skip=publish,sign,sbom,docker

# build a local docker image for smoke-testing (never pushes)
[group('build')]
image tag="rfc6035-2otel:dev":
    docker build --build-arg VERSION={{version}} --build-arg COMMIT={{commit}} --build-arg BUILD_DATE={{build_date}} -t {{tag}} .

# CI-only superset of check: adds the vuln scan, fuzz smoke, snapshot release build and local image build
[group('check')]
ci: check audit fuzz build-snapshot image

# lint and render the Helm chart
[group('check')]
[no-exit-message]
helm-lint:
    helm lint {{chart_dir}}
    helm template rfc6035-2otel {{chart_dir}} > /dev/null

# regenerate the Helm chart README from its chart metadata
[group('gen')]
helm-docs: setup
    {{tools_dir}}/helm-docs --chart-search-root charts

# verify the Helm chart README has no drift from its generated content
[group('check')]
[no-exit-message]
helm-docs-check: helm-docs
    git diff --exit-code {{chart_dir}}/README.md

# remove reproducible build output and downloaded tools
[group('build')]
clean:
    rm -rf bin {{tools_dir}}
```

## 3. Makefile disposition

`git rm Makefile` once every item below is verified working through `just`.

| Make target | Replacement | Notes |
|---|---|---|
| `build` | `just build` | Same ldflags, same output path. `BINARY`/`VERSION`/`COMMIT`/`BUILD_DATE` become `just` variables computed once per invocation via backtick assignment. |
| `test` | `just test` | Adds an optional `filter=""` param per the mandatory-vocabulary contract; `go test -race ./...` unchanged when no filter given. |
| `vet` | `just vet` | Unchanged. |
| `fmt` | `just fmt` | Unchanged (`gofmt -w` only — this repo's Makefile never ran goimports on fmt, only in the golangci-lint formatters list; don't add goimports to `fmt`/`fmt-check`, it isn't currently enforced and adding it now is a scope change this task does not authorize). |
| `fmt-check` | `just fmt-check` | Same `gofmt -l` check, plus `just --fmt --check` per §5.10 of the standard (not optional). |
| `tidy` | `just tidy` | Unchanged. |
| `tidy-check` | `just tidy-check` | Unchanged. |
| `dashboard` | `just gen` (partial) | `gen` now runs both `build_dashboard.py` and `build_rules.py` together — there was never a reason to regenerate one without the other since `build_rules.py` imports `build_dashboard` (see `grafana/build_rules.py:9`). If a future need arises to regenerate just one, add a private helper recipe then — don't split `gen` speculatively now. |
| `rules` | `just gen` (partial) | Same as above. |
| `grafana-check` | `just gen-check` | Identical body: both builders' `--check` flags plus both unittest `discover` calls. |
| `helm-docs` | `just helm-docs` | Same `.tools/helm-docs` install-once pattern, now via `just setup` as a recipe dependency instead of an inline guard. |
| `docker` | `just image` | Renamed to the mandatory-vocabulary name `image`; gains a `tag=` default param (`rfc6035-2otel:dev`, matching the Makefile's implicit `$(BINARY):dev`). |
| `check` | `just check` | Now ALSO runs `lint` (`golangci-lint run ./...`) — the old `make check` deliberately excluded it (see `AGENTS.md:15`, quoted below) and CI ran it as a separate parallel job. Folding it into `just check` is required by the fleet standard's completeness rule (§1: "If CI runs a check that `check` does not, the contract is broken"). This is the one intentional behavior change in this migration — call it out in the PR/commit description. `govulncheck`, the fuzz smoke, the goreleaser snapshot build and the docker image build stay OUT of `check` and live in the new `ci` superset recipe instead, because they are either network/registry-dependent, slow, or already run through a dedicated GitHub Action in CI (see §5) — pulling them into the fast local gate would slow down every `just check` run for marginal benefit. |
| n/a (`.PHONY` line) | delete | Meaningless in `just`; every recipe already always runs. |

Quoted for context, `AGENTS.md:14-15` currently reads:
```
make check                # fmt-check vet test tidy-check grafana-check build
golangci-lint run ./...   # CI runs this and `make check` does NOT — a green make is not a green CI
```
This is exactly the gap `just check` closes. Update this section per §6 below.

**After the justfile is proven locally and CI is switched (§8 order of work): `git rm Makefile`.**

## 4. Script disposition

No tracked `*.sh`/`*.bash`/`*.zsh`/`*.ps1` files exist in this repo (`git ls-files | grep -E
'\.(sh|bash|zsh|ps1)$'` returns nothing). The only task-shaped scripts are Python, under `grafana/`
and `scripts/`.

| Script | Classification | Disposition |
|---|---|---|
| `grafana/build_dashboard.py` (283 lines) | KEEP | Real program: full dashboard-JSON builder with a panel registry, signal catalog parsing, and a `--check` drift mode. Wrapped by `just gen` / `just gen-check`. |
| `grafana/build_rules.py` (226 lines) | KEEP | Real program: alert/recording-rule builder, imports `build_dashboard` as a library. Same wrapping. |
| `grafana/common.py`, `grafana/panels.py`, `grafana/__init__.py` | KEEP | Library modules imported by the two builders above, not independently invoked — no recipe needed for these individually. |
| `grafana/tests/test_generated.py` | KEEP | Unit test suite for the generators (shell test-suite equivalent, §6 of the standard). Run via `just gen-check`, never absorbed. |
| `scripts/grafana-prune-rules.py` (130 lines) | KEEP | Real program invoked only by `.github/workflows/grafana-sync.yml` (a deploy workflow against live Grafana state, gated by OpenBao-minted credentials). Out of scope for this migration — see §10. Not reachable via any `just` recipe; it is not a developer/CI-check task, it is a production deploy step. |
| `scripts/grafana-verify-rules.py` (187 lines) | KEEP | Same as above — invoked only by `grafana-sync.yml`. |
| `scripts/verify-gitsync.py` (153 lines) | KEEP | Same as above — invoked only by `grafana-sync.yml`. |
| `scripts/tests/test_grafana_reconcile.py` | KEEP | Unit test suite covering the three deploy scripts above. Run via `just gen-check` (already was, under `make grafana-check`'s `python3 -m unittest discover -s scripts/tests`) even though the scripts it tests are themselves out of scope — the tests are cheap, fast, and were already part of the local/CI gate. |

No ABSORB candidates: there is nothing here that is a thin wrapper around 1-2 commands with no
control flow. Every script here is either a substantial generator program or a deploy-time tool with
real logic (subprocess calls to `gcx`, JSON diffing, retry-shaped verification). None gets deleted.

## 5. CI changes

### `.github/workflows/ci.yml`

Add a `setup-just` step to every job that will call a `just` recipe (`build-test`, `govulncheck`,
`fuzz`). Do **not** add it to `lint`, `goreleaser-snapshot`, or `docker-build` — those three stay on
their existing GitHub Actions (`golangci-lint-action`, `goreleaser-action`, `docker/build-push-action`)
unchanged; converting an Action-based step into `run: just …` would lose its caching/annotation
behavior for no benefit, and the standard's "never convert a `uses:` into `run: just`" rule extends to
these dedicated tool Actions, not just reusable `workflow_call`s.

```yaml
      - uses: extractions/setup-just@<pin-this-SHA> # v4
        with:
          just-version: '1.58.0'
```
Resolve `<pin-this-SHA>` to a real commit SHA of `extractions/setup-just` at execution time — every
other action in this repo's workflows is SHA-pinned with a trailing `# vN` comment (see the
`actions/checkout` pin repeated throughout `ci.yml`/`helm.yml`); match that convention exactly, do not
leave a floating tag.

Per-job edits:

- **`build-test`**: insert `setup-go` (unchanged) then the `setup-just` step above, then change
  `- run: make check` to `- run: just check`.
- **`lint`**: **unchanged.** Still uses `golangci-lint/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9` with `version: v2.13.2`. This pin must stay equal to `golangci_lint_version` in the justfile — if one is bumped by Renovate/Dependabot without the other, `just lint` (local) and the CI `lint` job silently diverge. Flag this coupling in a comment above the justfile's `golangci_lint_version` line pointing at `ci.yml`'s `lint` job.
- **`govulncheck`**: insert `setup-just`, then replace both `run:` lines (`go install
  golang.org/x/vuln/cmd/govulncheck@v1.3.0` and `govulncheck ./...`) with a single `- run: just audit`.
  Keep `govulncheck_version` in the justfile equal to the `v1.3.0` that was here — same coupling
  concern as above, note it the same way.
- **`fuzz`**: insert `setup-just`, then replace
  `- run: go test ./internal/vqreport -run='^$' -fuzz='^FuzzParse$' -fuzztime=10s` with
  `- run: just fuzz`.
- **`goreleaser-snapshot`**: unchanged. Stays on `goreleaser/goreleaser-action@f06c13b6…`.
- **`docker-build`**: unchanged. Stays on `docker/setup-buildx-action` + `docker/build-push-action`.
- **`ci-success`**: unchanged — `needs: [build-test, lint, govulncheck, fuzz, goreleaser-snapshot,
  docker-build]` stays exactly as-is; this is the branch-ruleset-gated check name, do not touch its
  job list or its `if: always()` / failure-detection logic.
- Do not touch `permissions:`, `concurrency:`, or any `actions/checkout`/`actions/setup-go` pin.

### `.github/workflows/helm.yml`

The `lint-template` job currently has no Go/just setup at all (it only uses `azure/setup-helm`). Add
`extractions/setup-just` there too (helm itself doesn't need Go, but `just` does need installing).

Replace:
```yaml
      - run: helm lint "$CHART_DIR"
      - run: helm template rfc6035-2otel "$CHART_DIR" > /dev/null
      - run: |
          make helm-docs
          git diff --exit-code "$CHART_DIR/README.md"
```
with:
```yaml
      - run: just helm-lint
      - run: just helm-docs-check
```
`helm-docs-check` needs Go on this runner (it calls `just setup`, which does `go install
helm-docs`), so also add `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0` with
`go-version-file: go.mod` — copy the exact same step used in `ci.yml`. This is a genuine new
dependency this job didn't have before; note it plainly in the commit message so it isn't mistaken for
scope creep.

`helm-success` job (the `needs: [lint-template]` gate) is unchanged.

### Workflows deliberately not touched

`actionlint.yml`, `arm-automerge.yml`, `auto-rc.yml`, `codeql.yml`, `dependency-review.yml`,
`docker-security.yml`, `ghcr-cleanup.yml`, `publish.yml`, `release-assets.yml`, `release-please.yml`,
`scorecard.yml`, `trigger-docs-sync.yml`, `zizmor.yml`, `grafana-sync.yml` — none of these has a
`run:` block containing build/test/lint/format/generate/validate logic that belongs in `just`. See
§10.

## 6. Docs and agent-contract changes

- **`AGENTS.md:12-24`** ("The gate" section) — replace:
  ```
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
  ```
  with:
  ```
  ## Task interface

  This repo's task surface is a `justfile`. Discover it, don't guess it:

      just --list                        # human-readable
      just --dump --dump-format json     # machine-readable
      just --show <recipe>               # what a recipe actually runs

  - `just check` is the full local gate and is exactly what CI's `build-test` job enforces
    (`fmt-check`, `lint`, `vet`, `test`, `tidy-check`, `gen-check`, `build`). It must pass before you
    commit. It is the tracker's `definition_of_done`, so every task inherits it.
  - `just ci` is a superset CI also enforces via separate parallel jobs: `audit` (govulncheck), `fuzz`
    (10s parser fuzz smoke), `build-snapshot` (GoReleaser), `image` (Docker build). You don't need to
    run these locally for every change, but they exist for parity when you do.
  - Prefer `just <recipe>` over the underlying tool. If you are typing `golangci-lint` or `pytest`,
    you want `just lint` or `just gen-check`.
  - Run `just` with stdin from /dev/null. No recipe here is marked `[confirm]`, but if one is added
    later, stop and ask before running it rather than passing `--yes`.
  - If a task you need does not exist, add a recipe with a `#` doc comment and a `[group(...)]`
    rather than running a bare command.

  Generated artefacts are never hand-edited: `dashboards/`, `alerts/` and `spec/signal-catalog.json`
  come from the builders under `grafana/`. Edit the builder, then `just gen`. `just gen-check` fails
  if the committed output does not match.
  ```
  Everything else in `AGENTS.md` (task tracking, PII rules, backlog CLI rules, git policy) is
  untouched — this migration only replaces the tool-invocation section.

- **`CLAUDE.md`** — no change. It only `@`-imports `AGENTS.md`.

- **`CONTRIBUTING.md:9-16`** — replace:
  ```
  Requires the Go version declared by `go.mod`. The local acceptance command is:

  ```sh
  make check
  ```

  It runs formatting, vet, race-enabled tests, module-tidiness verification, and a binary
  build. `make build` writes `bin/rfc6035-2otel`; `make test` runs the race detector.
  ```
  with:
  ```
  Requires the Go version declared by `go.mod` and `just` (https://just.systems). The local
  acceptance command is:

  ```sh
  just check
  ```

  It runs formatting, linting, vet, race-enabled tests, generated-artefact drift checks, and a binary
  build. `just build` writes `bin/rfc6035-2otel`; `just test` runs the race detector (pass
  `filter="TestName"` to narrow it). Run `just --list` for the full task surface.
  ```

- **`README.md:12-15`** — replace:
  ```
  ## Quick start

  ```sh
  make build
  ./bin/rfc6035-2otel -version
  ```
  ```
  with:
  ```
  ## Quick start

  ```sh
  just build
  ./bin/rfc6035-2otel -version
  ```
  ```

## 7. `backlog/config.yml`

Current line:
```
definition_of_done: ["make check", "golangci-lint run ./...", "gh run list --branch main --limit 1 shows ci-success green at the exact pushed SHA"]
```
New line (this file is the one Backlog.md file edited by hand — see `AGENTS.md`'s note that
`backlog/config.yml` is the sole exception to "never hand-edit backlog markdown"; this is YAML, and
list-valued keys can't be set through `backlog config set`):
```
definition_of_done: ["just check", "gh run list --branch main --limit 1 shows ci-success green at the exact pushed SHA"]
```
`golangci-lint run ./...` is dropped as a separate entry because `just check` now runs it
(`just check` → `lint` → `golangci-lint run ./...`) — listing it twice would be redundant and, worse,
would drift the moment `just lint`'s invocation changes without a matching edit here.

## 8. Order of work

1. Add the `justfile` (§2) at the repo root. Do not touch the `Makefile` yet — both exist side by
   side.
2. Run `just --fmt --check`; if it fails, run `just --fmt` once and re-verify (`--fmt` without
   `--check` mutates the file in place — expected here, this is the one time to run it unchecked).
3. Run `just check` locally end to end. Fix any drift between what the justfile assumes and what the
   toolchain actually does (tool versions especially — reread `.github/workflows/ci.yml` and
   `.golangci.yml` for the current `golangci-lint`/`govulncheck`/`goreleaser` pins before trusting the
   ones written into §2, they may have moved since this task was filed).
4. Run `just ci` locally at least once (needs Docker running for `just image`, and `git` history for
   `just build-snapshot`'s version detection).
5. Diff `just check`'s behavior against `make check` + the CI `lint` job run on the same commit —
   confirm they agree except for the deliberate addition of `lint` into `check` (§3).
6. Edit `.github/workflows/ci.yml` and `.github/workflows/helm.yml` per §5. Push to a branch (or
   direct to `main` per this repo's own git policy — it pushes straight to `main`, see `AGENTS.md`'s
   Git section) and watch `ci-success` and `helm-success` go green calling `just`, not `make` or raw
   tool invocations, for every job that changed.
7. Update `AGENTS.md`, `CONTRIBUTING.md`, `README.md` per §6.
8. Update `backlog/config.yml`'s `definition_of_done` per §7.
9. Grep the whole tree once more for any remaining `make ` / `./scripts/` / `Makefile` reference this
   task missed (`grep -rn 'make check\|make build\|make helm-docs\|make dashboard\|make rules\|make grafana-check' --include='*.md' --include='*.yml' .`, excluding `backlog/` history/archive files which are historical record and not part of the live contract).
10. Only once steps 1-9 are green and nothing references it: `git rm Makefile`.

Steps 1-5 keep the repo green throughout (the Makefile and CI both still work). Step 6 is the only
point where CI itself changes — verify it goes green before touching docs or deleting anything. The
deletion in step 10 is strictly last.

## 9. Traps specific to this repo

- **`grafana/build_rules.py` imports `grafana/build_dashboard.py` as a Python module** (`import
  build_dashboard` at `grafana/build_rules.py:9`), which only resolves when the interpreter's CWD is
  `grafana/`. The `gen`/`gen-check` recipes MUST `cd grafana &&` before invoking either script — do
  not "simplify" this to `python3 grafana/build_dashboard.py` run from the repo root, it will
  `ModuleNotFoundError` on the rules builder.
- **Each recipe line is its own shell** (§10 of the standard). `gen`/`gen-check`'s four lines each
  independently `cd grafana` (or don't, for the `scripts/tests` line) — this is deliberate, not an
  oversight; don't collapse them into one line with `&&` chains across the `cd` boundary, and don't
  extract a shared `cd grafana` onto its own line expecting it to persist.
- **`helm-docs` needs Go on the runner to install itself** (`go install
  github.com/norwoodj/helm-docs/...`) — the existing `helm.yml` `lint-template` job never had Go
  before (it only runs `azure/setup-helm`). Adding `just helm-docs-check` there is a genuine new
  dependency (an `actions/setup-go` step), not a no-op wiring change. Don't skip it silently.
- **Two independent CI gates, not one.** `ci-success` (from `ci.yml`) and `helm-success` (from
  `helm.yml`) are separate required checks with separate path triggers. `just check` matches
  `ci-success`'s `build-test` job; it intentionally does NOT include `helm-lint`/`helm-docs-check` —
  those only need to run when `charts/**` changes, which is exactly what `helm.yml`'s path filter
  already does. Don't fold Helm recipes into the main `check`/`ci` chain.
- **`golangci_lint_version` and `govulncheck_version` in the justfile duplicate pins that also live in
  `.github/workflows/ci.yml`** (the `lint` job's `version: v2.13.2` input, and the removed `go install
  golang.org/x/vuln/cmd/govulncheck@v1.3.0` line folded into `just audit`). These are two independent
  places Renovate/Dependabot can bump one without the other. There is no `just` mechanism to share a
  version constant with a YAML workflow file across process boundaries in this fleet's no-remote-import
  model (§7 of the standard) — accept the duplication, but leave the coupling comments from §5 in place
  so a future bump PR touches both.
- **`go run pkg@version` (used for `audit` and `build-snapshot`) hits the network and Go module proxy
  on every invocation** unless the module is already in the local module cache — this is a deliberate
  tradeoff to avoid a stateful install step in `setup`; if this becomes a problem (offline dev, flaky
  proxy), the fallback is `go install pkg@version` into `.tools` inside `setup`, mirroring the
  `helm-docs` pattern — do not silently switch to that without updating both `setup` and the two
  recipes' bodies together.
- **`just --fmt` is destructive** (rewrites the justfile's whitespace/formatting) — only run it bare
  once during initial authoring (§8 step 2); `fmt-check`'s `just --fmt --check` is the one that
  belongs in `check` and never mutates.
- **The `test` recipe's `filter` param uses single-quote interpolation** (`-run '{{filter}}'`) per
  §10's quoting trap — do not remove the quotes, an unquoted `{{filter}}` containing a space would
  split into two arguments.

## 10. Out of scope — do not touch

- **`scripts/grafana-prune-rules.py`, `scripts/grafana-verify-rules.py`, `scripts/verify-gitsync.py`**
  — deploy-time tools invoked only by `.github/workflows/grafana-sync.yml` against live Grafana state
  behind OpenBao-broker-minted, per-run credentials. Not a developer/CI-check task surface; §6 of the
  standard explicitly excludes "scripts invoked by something other than a developer or CI" (this is
  invoked by a push-triggered deploy workflow acting as an operator, not a PR gate) — leave these
  scripts, and every step of `grafana-sync.yml`, exactly as they are.
- **`.github/workflows/grafana-sync.yml`** in full — do not add `setup-just`, do not touch its
  `python3 scripts/...` invocations, its `gcx` install/context steps, or its GitSync push/verify logic.
- **`.github/workflows/publish.yml`** — calls the reusable `uses:
  rknightion/.github/.github/workflows/container-publish.yml@…` workflow. Never convert a `uses:`
  reusable-workflow call into `run: just` (§8 of the standard, and §13's anti-pattern list).
- **`.github/workflows/release-please.yml`, `auto-rc.yml`, `release-assets.yml`** — release-please and
  its satellite workflows are GitHub-native and out of scope by name (§8 of the standard). Do not fold
  release logic into `just`, and do not touch their auth (per this account's standing rule, they mint
  per-run tokens from the OpenBao broker — never provision a durable `RELEASE_PLEASE_TOKEN`).
- **`.github/workflows/codeql.yml`, `zizmor.yml`, `actionlint.yml`, `scorecard.yml`,
  `dependency-review.yml`, `docker-security.yml`, `ghcr-cleanup.yml`, `arm-automerge.yml`,
  `trigger-docs-sync.yml`** — all GitHub-native or fleet-standard security/automation workflows, none
  with build/test/lint logic in a `run:` block. Leave untouched.
- **The `lint` job's `golangci-lint-action`, the `goreleaser-snapshot` job's `goreleaser-action`, and
  the `docker-build` job's `docker/build-push-action` steps in `ci.yml`** — dedicated Actions with
  caching/annotation behavior a raw `run: just …` would lose. Leave the `uses:` steps as they are; only
  the `run:` steps in those jobs (there are none left to touch besides what §5 already lists) change.
- **`archive/`, `codex/`, `docs/superpowers/`** — historical/scratch material, not part of the live
  task surface, not referenced by any recipe.
- **`spec/protocol.md`, `spec/signal-catalog.json`, `dashboards/rfc6035-2otel.json`,
  `alerts/grafana-managed/**`** — generated/spec artifacts, already covered by `just gen` /
  `just gen-check`; do not hand-edit them as part of this migration.
- **`backlog/` markdown files** — never hand-edited; drive all task/doc changes through the `backlog`
  CLI. `backlog/config.yml` is the sole named exception (§7).
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 A top-level justfile exists at the repo root defining all seven mandatory recipes (default, setup, fmt, fmt-check, lint, test, check) plus gen, gen-check, audit, fuzz, build-snapshot, image, ci, helm-lint, helm-docs, helm-docs-check, clean
- [ ] #2 just check runs fmt-check, lint, vet, test, tidy-check, gen-check and build, and is exactly what .github/workflows/ci.yml's build-test job runs
- [ ] #3 just --fmt --check passes with no diff
- [ ] #4 just --list shows a doc comment and a [group(...)] for every public recipe
- [ ] #5 Makefile is deleted (git rm), with no remaining reference to make in AGENTS.md, CONTRIBUTING.md, README.md, or any workflow
- [ ] #6 grafana/build_dashboard.py, grafana/build_rules.py, scripts/grafana-prune-rules.py, scripts/grafana-verify-rules.py, and scripts/verify-gitsync.py all still exist as files; the first two are reachable via just gen/just gen-check, the last three remain invoked only by grafana-sync.yml and by no just recipe
- [ ] #7 .github/workflows/ci.yml's build-test, govulncheck and fuzz jobs call just check / just audit / just fuzz respectively via a setup-just step, while its lint, goreleaser-snapshot and docker-build jobs keep their existing Actions unchanged, and ci-success still gates on the same needs list
- [ ] #8 .github/workflows/helm.yml's lint-template job calls just helm-lint and just helm-docs-check via added setup-go and setup-just steps, and helm-success still gates on lint-template
- [ ] #9 AGENTS.md's gate section is replaced with the Task interface contract naming just check as the definition_of_done, and CONTRIBUTING.md/README.md no longer mention make
- [ ] #10 backlog/config.yml's definition_of_done lists "just check" instead of "make check" and "golangci-lint run ./..."
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 make check
- [ ] #2 golangci-lint run ./...
- [ ] #3 gh run list --branch main --limit 1 shows ci-success green at the exact pushed SHA
<!-- DOD:END -->

## Comments

<!-- COMMENTS:BEGIN -->
author: campaign-ordering
created: 2026-08-29 09:18
---
## Fleet ordering — WAVE 2. Starts after the Wave 0 pilot (`sf2loki` / SFL-0073) and the Wave 1 hubs land.

Within Wave 2 the order is free — these repos do not depend on each other. Batching by language is worthwhile so one lane reuses its Makefile-to-recipe mapping across similar repos.

Do not start before the pilot reports. The standard may be amended off the back of it, and picking this up early risks coding against a superseded seam.

**Provisioning `just` in CI.** Which mechanism depends on the runner, and the two must not be mixed:

| Runner | Mechanism |
| --- | --- |
| `arc-arm64` (m7kni self-hosted) | `just` is **baked into the runner image** by `m7kni/ci-tools` (`runner-image/Dockerfile`, `ARG JUST_VERSION`). Do **not** add `extractions/setup-just`, and delete the step if this repo already has one — it installs a second `just` earlier on `PATH` and turns the image pin into a lie. |
| GitHub-hosted (all `rknightion` repos) | `extractions/setup-just`, SHA-pinned, with an explicit `just-version:`. |

Both sides currently sit on **1.58.0** and are Renovate-managed. `ci-tools`' `Tool version drift` workflow fails if the Dockerfile `ARG` and the published image ever disagree, and lists any repo still carrying a second pin.

**While you are in the workflow files, check the hub pin.** On 2026-08-29 Renovate was unfrozen for `rknightion/.github` in `m7kni/renovate-config` — it had been `enabled: false` on the mistaken belief that callers tracked `@main`, which froze the fleet across 19 different hub SHAs (v1.3.1 June → v1.9.7 August) so that no hub fix ever propagated. Bumps now arrive as one grouped, CI-gated, automerged PR per repo. **A `uses:` whose comment is not a real `# vX.Y.Z` still cannot be bumped** (it resolves to a digest-only update, which the fleet rules disable) — if you find one, repair the comment as part of this task.
---

author: campaign-ordering
created: 2026-08-29 10:43
---
## Standard amendment — `ci` is the sanctioned superset of `check` (RATIFIED)

This supersedes the frozen wording *"`check` is the complete local gate and reproduces every CI job that can run off a GitHub runner"*, which several lanes could not honour without making the pre-commit gate depend on a Docker daemon.

**The definitions now are:**

- **`check`** — everything that runs with **only the language toolchain installed**. This is the pre-commit gate. A leg that runs on a bare toolchain belongs here *however long it takes*.
- **`ci`** — `check` plus the legs CI gates that need a **Docker daemon, a service container, or cross-compilation**, and nothing else. Written as `ci: check <heavy legs>`.

**Every leg you put in `ci` must carry a comment naming which of those three it needs.** That comment is the guard: without it `ci` becomes the bin for anything slow or awkward, `check` quietly stops meaning much, and the fleet is back to a per-repo gate.

Eleven of the 42 lanes arrived at this shape independently before it was ratified, which is why it won.

**If this repo has no such legs, it has no `ci` recipe at all** and `check` is the whole gate. Do not add an empty one.
---
<!-- COMMENTS:END -->
