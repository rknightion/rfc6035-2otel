---
id: VQR-0002
title: Upgrade Go toolchain to 1.27
status: In Progress
assignee: []
created_date: '2026-08-23 19:06'
updated_date: '2026-08-23 20:21'
labels: []
dependencies: []
ordinal: 2000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Adopt Go 1.27 consistently across the application, nested modules, build images, CI configuration, setup automation, and version-specific contributor documentation.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [x] #1 All active Go module and toolchain pins require Go 1.27
- [x] #2 Build images, CI jobs, setup automation, and current documentation agree with the Go 1.27 requirement
- [x] #3 The repository green-bar validation passes under Go 1.27
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [x] #1 make check
- [x] #2 golangci-lint run ./...
- [ ] #3 gh run list --branch main --limit 1 shows ci-success green at the exact pushed SHA
<!-- DOD:END -->

## Implementation Plan

<!-- SECTION:PLAN:BEGIN -->
1. Inventory every active Go version pin, including nested modules and container or CI toolchains. 2. Update the pins and version-specific documentation to Go 1.27.0 without changing historical records or fixtures. 3. Run the repository-defined validation gate, review the diff, commit to main, push, and confirm hosted CI.
<!-- SECTION:PLAN:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Local Go 1.27.0 evidence: make check and golangci-lint run ./... passed. Final diff check passed. CodeRabbit was skipped because only declarative module and container toolchain pins changed. Hosted exact-head ci-success remains required for DoD 3.

Exact-head CI run 32662554130 exposed stale Linux analysis tools: golangci-lint v2.12.2 and govulncheck v1.1.4 do not support the Go 1.27 syntax they analyze. CI now uses current golangci-lint v2.13.1 and govulncheck v1.3.0. Linux-target lint passed with 0 issues, v1.3.0 reported no vulnerabilities, and actionlint accepted the workflow. The failed run is retained as before-fix evidence.
<!-- SECTION:NOTES:END -->
