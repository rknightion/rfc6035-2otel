# GitHub issue archive

`issues.json` is the complete, **redacted** archive of every GitHub issue this repository ever had —
19 issues (#2–#22, the gaps being pull requests) and all 26 of their comments, captured on 2026-08-14
immediately before the issues were deleted from GitHub. The issues no longer exist upstream, so this
file is the record, not a pointer to one.

Open work moved to the Backlog.md board in `backlog/` at the same time. `backlog/docs/doc-0003`
indexes every closed issue against the commit that resolved it.

## Reading it

```bash
jq -r '.[] | select(.number==20) | .title, "", .body' archive/issues.json   # one issue
jq -r '.[] | "\(.number)\t\(.state)\t\(.title)"' archive/issues.json        # the whole list
jq -r '.[] | select(.number==11) | .comments[] | .body' archive/issues.json # its comments
```

Fields captured per issue: `number`, `title`, `body`, `comments`, `labels`, `state`, `stateReason`,
`author`, `createdAt`, `updatedAt`, `closedAt`, `url`, `milestone`, `assignees`. The `url` values
still point at the deleted issues and will 404 — they are kept because commit messages and the wave
reports cite issue numbers, and the URL makes the number unambiguous.

**Comment completeness was verified, not assumed.** `gh --json comments` paginates. The per-issue
counts from the REST API (`repos/OWNER/REPO/issues?state=all&per_page=100`) were compared against the
dump issue by issue: 19 issues matched exactly, 26 comments against 26.

## Redaction

This archive is committed to a public repository, and it is committed at the moment the deletable
copy is being destroyed — so lab identifiers were replaced before it entered git history rather than
after. Each real value maps to exactly one stable token everywhere, so correlation across issues and
comments still works: the same host in issue #2 and issue #21 is the same token.

| Token | What it denotes |
|---|---|
| `HOST-A` | The host running the collector and the packet captures |
| `PHONE-A` | The handset configured for the standard RFC 6035 dialect |
| `PHONE-B` | The handset configured for the Poly pre-standard dialect |
| `IP-HOST-A` | `HOST-A`'s LAN address |
| `IP-PHONE-A` / `IP-PHONE-B` | The two handsets' LAN addresses |
| `IP-TAILNET-HOST-A` | `HOST-A`'s tailnet address |
| `GRAFANA-STACK-HOST` | The Grafana Cloud stack hostname telemetry was queried against |
| `GRAFANA-STACK-ID` | That stack's numeric id |

79 substitutions across 387 decoded string fields. The sweep that certified the result ran over the
**decoded fields**, never over the serialized JSON: in `json.dumps` output an escape such as `\n`
leaves a literal `n` immediately before the following word, which breaks a `\b` word boundary, so a
blob sweep can certify a file clean while it still leaks. The verifying script is deliberately **not**
committed — it contains the real values on the left-hand side of the mapping, so shipping it would
undo the redaction it performed.

Not redacted, deliberately: vendor names (Poly, Lens, OPNsense, Grafana, Tailscale), the public
`m7kni.io` docs-hub domain that the repository's own tracked files already carry, metric and log field
names, commit SHAs, CI run ids, and the SIP `Call-ID` values from lab calls — they are per-call
hashes that identify nothing and authenticate to nothing, and they are the evidence in several
issues. No credentials, tokens, email addresses or MAC addresses were present in the source at all;
that was checked, not assumed.
