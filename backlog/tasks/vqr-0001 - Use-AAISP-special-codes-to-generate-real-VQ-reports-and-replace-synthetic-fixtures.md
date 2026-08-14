---
id: VQR-0001
title: >-
  Use AAISP special codes to generate real VQ reports and replace synthetic
  fixtures
status: To Do
assignee: []
created_date: '2026-08-14 16:34'
updated_date: '2026-08-14 16:34'
labels:
  - fixtures
  - parser
  - live-calls
dependencies: []
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Replace and extend the currently-synthetic parser fixtures with reports captured from real calls, using AAISP's special dialling codes as an on-demand generator. Imported from GitHub issue #20, which stays open as the long-form record (`gh issue view 20`).

WHY: the samples this project ships are built from the RFC 6035 / RFC 3611 examples plus measured Poly output, not from a wire capture. A parser validated only against the spec's own examples is validated against the spec, not against what a device actually emits. The pre-standard grammar in particular was built from measurement rather than from any specification.

THE CODES (they terminate on AAISP's platform, so a call can be placed and held at zero cost without involving another person): *103 plays hold music - the best sample generator, long continuous known audio; *100 is a 1 kHz tone, the cleanest possible control signal; *105 reads the time, a short call; *104 / 17070 read back the calling number; *400-*699 return a specific SIP error code, which makes failure paths reproducible on demand.

SCOPE: a documented, repeatable procedure (not necessarily automated) - which code, how long to hold, what to expect. Diff a captured real report against the existing synthetic fixtures and record every difference; any divergence is a parser bug or a fixture bug and both matter. Extend the fixtures with real captures while keeping the synthetic ones as spec-conformance cases, labelled which is which - that distinction is the entire point. Cover the short-call and failed-call cases.

CORRECTION CARRIED OVER, do not re-derive: an earlier version of issue #20 claimed the 'extra' handset had never delivered a report and blamed a shadowed firewall rule. BOTH HALVES WERE WRONG, corrected 2026-08-13. Both dialects arrive and parse correctly - the live collector log shows source=10.0.0.139:5060 dialect=standard and source=10.0.50.175:5060 dialect=prestandard repeatedly. The 'zero reports' finding came from querying the RETIRED vq-collector's syslog stream in Loki, which this project does not write to, so the empty result was guaranteed and meant nothing. Check docker logs rfc6035-2otel or the OTLP data. The firewall rule was genuinely mis-sequenced but the traffic was already permitted by a higher-priority rule whose port alias contains 5060, so it was shadowed with no consequence. The issue's original first acceptance criterion ('firewall rule corrected to sequence below 336') contradicted its own correction and has been dropped here.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 At least one real report captured per dialect via *103, with the capture procedure recorded as it is run
- [ ] #2 Real captures diffed against the existing synthetic fixtures and every difference documented, each classified as a parser bug or a fixture bug
- [ ] #3 Fixtures extended with real captures, labelled real vs synthetic, with the synthetic ones retained as spec-conformance cases
- [ ] #4 Short-call and failed-call behaviour covered using *105 and the *400-*699 range
- [ ] #5 Procedure written up under docs/ so it can be repeated without rediscovering the codes or the hold times
<!-- AC:END -->

## Definition of Done
<!-- DOD:BEGIN -->
- [ ] #1 make check
- [ ] #2 golangci-lint run ./...
- [ ] #3 gh run list --branch main --limit 1 shows ci-success green at the exact pushed SHA
<!-- DOD:END -->

## Implementation Notes

<!-- SECTION:NOTES:BEGIN -->
Requires two exclusive resources per the Wave operating model doc: the live handsets/Camden deployment, and a SEPARATE explicit grant of call-origination authority. A lane without that grant in its brief reads only. Wave 2's P2-L8 parked on exactly this.
<!-- SECTION:NOTES:END -->
