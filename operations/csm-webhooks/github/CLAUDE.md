# CSM GitHub Webhook — working notes

Lives in `operations/csm-webhooks/github/`: one folder per webhook source under
`operations/csm-webhooks/` (see that folder's README). This one is GitHub's.

The public endpoint GitHub posts issue and issue-comment webhooks to. It
verifies the HMAC signature and forwards the delivery to entity-service.
Nothing else.

```
GitHub ──POST /webhooks/github──▶ this component (Public)
                                      │  verify X-Hub-Signature-256
                                      ▼
                            Choreo gateway  (adds x-jwt-assertion)
                                      │
                                      ▼
              entity-service  POST /github/deliveries  (Organization)
                                      │
                                      ▼
                        GithubSyncService ──▶ Postgres
```

## Why it exists

entity-service is **Organization**-visible in Choreo and must stay that way.
GitHub has to reach the webhook from the internet, and making one route public
would publish every other route with it. So the endpoint lives here and the
data stays there.

## Hard rules

- **This component holds no database credentials and makes no decisions.** If
  something here starts needing to know what a delivery *means*, it belongs in
  entity-service instead.
- **The body is forwarded byte for byte.** The signature is computed over the
  raw bytes, so re-encoding the JSON — even only reordering keys — makes it
  unverifiable. `Delivery.Payload` is `json.RawMessage` for that reason, and
  because a parsed struct here would be a second model of GitHub's payload to
  keep in step with entity-service's own. `internal/webhook` has a test
  asserting a re-encoded equivalent of the same JSON fails to verify.
- **One directory under `cmd/`.** A second `package main` anywhere in this
  module breaks the Choreo Go buildpack: it cannot choose which to build,
  falls back to the module root, finds no `.go` files and fails with
  `no Go files in /workspace`. That failure names neither directory.
- **Every signature rejection is identical.** A caller must not learn which
  check failed, and must never be told the expected signature. ServiceNow's
  equivalent returned the computed HMAC in its 401 body, which made the secret
  irrelevant.

## Configuration

| variable | meaning |
|---|---|
| `GITHUB_WEBHOOK_SECRET` | the HMAC key, **the same value** as the repository webhook's secret. This is the only authentication on `/webhooks/github` — GitHub cannot present a bearer token. Empty makes the endpoint refuse everything rather than accept everything. |
| `ENTITY_BASE_URL` | entity-service, over the internal network |
| `ENTITY_TOKEN_URL` / `ENTITY_CLIENT_ID` / `ENTITY_CLIENT_SECRET` / `ENTITY_SCOPES` | the OAuth2 client-credentials identity it forwards as |
| `PORT` | defaults to 8080 |

**`ENTITY_CLIENT_ID` must appear in entity-service's `M2M_CLIENT_IDS`**,
or every forward is refused 401 and no delivery is ever applied.

## The one deployment detail that will bite

entity-service reads the caller's client id from **`x-jwt-assertion`**, not
from `Authorization`. In Choreo the gateway synthesises that header after
validating the bearer token, so this component only has to send
`Authorization` — which the OAuth2 transport does on its own.

But if the connection is wired so it bypasses the gateway, entity-service sees
no identity and answers **401 with `callerId=-`** on every forward. That log
line is the signature of this mistake.

Locally there is no gateway, so `scripts/csm-compose/gateway-shim` stands in:

```bash
PORT=18595 UPSTREAM_URL=http://localhost:18590 gateway-shim    # the shim
ENTITY_BASE_URL=http://localhost:18595 ... ./cmd/server        # this component
```

## Responses

| status | when |
|---|---|
| 200 | applied, deliberately skipped, or a duplicate delivery — all three, so GitHub stops retrying |
| 400 | unreadable body, or a missing delivery/event header |
| 401 | the signature did not verify |
| 500 | forwarding failed. GitHub retries, and that is the only recovery: nothing else re-reads its event stream |

entity-service answers **409** for a delivery it has already applied; this
component maps that to the 200 GitHub needs. The distinct internal status lets
the caller tell "already applied" from "applied just now" without parsing a
message.

## Setting up a repository

1. The repository needs an `account_github_repo` row mapping `owner`/`repository`
   to an account. **An unmapped repository is ignored silently** — not an
   error. Without the row, nothing happens and nothing says why.
2. The repository's issue templates must apply the labels the sync classifies
   on: `Type/ServiceRequest`, or a `[CR]:`/`[ECR]:` title with a `CR/*Change`
   class label. A template referencing a label the repository does not have
   produces issues that can never be classified.
3. Webhook: content type `application/json`, the same secret as
   `GITHUB_WEBHOOK_SECRET`, events **Issues** and **Issue comments**.

## A note on `GITHUB_INTEGRATION_LOGIN`

That setting lives on entity-service, not here, but it decides whether your own
activity is visible to the sync: events authored by that login are dropped as
the integration's own writes, **by identity**. Point it at a personal account
and the integration becomes blind to that person. It wants a bot account.
