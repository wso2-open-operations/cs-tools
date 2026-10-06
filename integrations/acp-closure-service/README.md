# ACP Closure Service

Go implementation of Phase 1 of the Account Closure Process (ACP) migration
from ServiceNow — subscription end-date closure only. Invoice- and
compliance-based closure logic (Phase 2) is out of scope; see
`docs/legacy-servicenow-reference/` at the repo root for the original
ServiceNow Script Include source this is ported from.

Unlike every other Go component in this repo, this is not an HTTP server: it
is a run-to-completion CLI. `main()` performs one full sweep over open
projects and exits — a Choreo Task component's cron owns the schedule, not
this process.

## Quick Start

```bash
# from integrations/acp-closure-service
cp .env.example .env   # fill in real values
go run ./cmd/acp-closure
```

`DRY_RUN` defaults to `true` — a run will fetch, decide and resolve
recipients without writing to `csm-integration-service`. Sending is a
separate switch, `IS_EMAIL_SEND_ENABLED` (see CLAUDE.md, "Real email
sending"). Set `DRY_RUN=false` only for a deliberate,
reviewed cutover.

## Overview

- Runtime: Go `1.26.5+`
- Entry point: `cmd/acp-closure/main.go`
- Calls `csm-integration-service`, not entity-service directly — see
  `internal/entity`'s package doc for why
- Authentication: OAuth2 client-credentials (M2M), scoped via
  `entity.RequiredScopes`

## Prerequisites

- Go `1.26.5+` — [install](https://go.dev/doc/install)

## Testing

```bash
go test ./...
go test -race ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

Or use `make`:

```bash
make test    # vet + race-detector tests
make build   # vet + test + compile
```

**Run `make test` before every push.** Unlike other Go modules in this repo,
this one is not currently wired into the shared `.githooks/pre-push` hook —
that hook only checks `apps/csm-portal/backend` today, and extending it to
cover other modules is a separate, deliberate change for whoever owns that
shared file, not bundled into this component's initial PR. Run tests
manually before pushing until that's addressed.

## Security Scanning

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt=text ./...
```

## Configuration

Copy `.env.example` to `.env` and fill in the values.

### csm-integration-service

| Variable | Description |
|---|---|
| `CSM_INTEGRATION_BASE_URL` | Base URL of csm-integration-service |
| `CSM_INTEGRATION_TOKEN_URL` | OAuth2 token endpoint |
| `CSM_INTEGRATION_CLIENT_ID` | OAuth2 client ID |
| `CSM_INTEGRATION_CLIENT_SECRET` | OAuth2 client secret |
| `CSM_INTEGRATION_SCOPES` | Required, space-separated. Kept out of code (rather than hardcoded) so the requested grant can be adjusted without a redeploy. Asgardeo enforces the actual grant regardless, so this only controls what's requested. See `entity.RequiredScopes` for the scope set csm-integration-service's token endpoint currently requires. |

### Run behavior

| Variable | Default | Description |
|---|---|---|
| `DRY_RUN` | `true` | Fails safe toward `true` on anything except an explicit, successfully-parsed `false` — unset, empty, or malformed all stay in dry-run. |
| `TEST_PROJECT_ID` | unset | When set, scopes the entire run to exactly this one project (fetched via `GetProject`) instead of paginating every `"Open"` project in the environment. Safe to combine with `DRY_RUN=false` for an end-to-end test against a single dedicated project. |

## Project Structure

```text
acp-closure-service/
├── cmd/acp-closure/main.go        # Entry point — config, wiring, one sweep, exit
├── internal/
│   ├── apierror/                  # Typed upstream error (4xx/5xx passthrough)
│   ├── closure/                   # Pure decision logic: notice windows, day-0 ordering
│   ├── entity/                    # HTTP client for csm-integration-service
│   ├── notify/                    # Notice shape + logging notifier (real sending: not yet built)
│   ├── recipients/                # Pure customer-contact fallback + AM-email resolution
│   ├── suspensionstate/           # suspensionProcessState blob <-> closure.NoticeWindow translation
│   └── sweep/                     # Orchestration: fetch -> decide -> notify -> write back
├── .env.example
└── Makefile
```

## Known discrepancies between documented/coded behavior and live behavior

Confirmed via direct testing against staging, not assumption — worth
knowing before trusting a sibling service's source or docs at face value:

- entity-service's own `maxLimit` constant states `100` for search
  pagination; the real, live limit is **50** (`limit: 51` returns a 400).
  `internal/sweep/run.go`'s `pageSize` is set accordingly, with the
  discrepancy documented inline.
- `csm-integration-service`'s `CLAUDE.md` (as of this writing) states that
  `PATCH /projects/{id}` always 401s under M2M-only auth. Confirmed via
  direct testing that this is not true in practice — the endpoint accepts
  M2M-only writes successfully.
- `suspensionProcessState`'s real shape is the rich, per-dimension legacy
  blob (`based_on_subscription_end_date` / `based_on_due_invoices` /
  `based_on_compliance`, each with `event_type` + action results) — see
  `internal/suspensionstate`'s package doc.

## Open dependencies

None of the original open dependencies remain:

- **Business-contact role**: `BUSINESS_CONTACT`, returned in a project
  contact's `roles` by csm-integration-service v1.1 (CSM Postgres). v1.0
  (ServiceNow) doesn't return project roles, so on v1.0 customer notices go
  to Primary Contacts. See CLAUDE.md, "Business Contacts come only from the
  CSM database (v1.1)".
- **Real email sending**: implemented (`notify.EmailNotifier`, WSO2's email
  service), behind `IS_EMAIL_SEND_ENABLED`.
