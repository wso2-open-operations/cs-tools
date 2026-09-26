# GitHub integration — deployment and post-deploy test

What to configure, what to point where, and how to prove it works after a
deploy. Replaces ServiceNow's `[GitHub Integration]` flows and the
`GitHubIssueContentProcessor` script include.

## 1. What runs where

```
GitHub  ──webhook (HMAC)──▶  entity-service  POST /webhooks/github   [inbound]
                                   │
                                   ├─ creates / updates change requests
                                   └─ guards: close, labels, scope

Postgres triggers ──▶ github_outbound_queue ──▶ outbound worker ──▶ GitHub  [outbound]
```

Both halves live in **entity-service**. There is no separate deployable.

## 2. Configuration

| Variable | Required | What it is |
|---|---|---|
| `GITHUB_INTEGRATION_ENABLED` | yes | `true` to start the worker and mount the webhook route. |
| `GITHUB_TOKEN` | yes | Fine-grained PAT. **Issues: read & write**, on the mapped repositories only. |
| `GITHUB_WEBHOOK_SECRET` | yes | Any random string (`openssl rand -hex 32`). The same value goes in each repository's webhook settings. |
| `GITHUB_INTEGRATION_LOGIN` | yes | GitHub **login** of the bot account. Events sent by it are dropped — this is the loop guard. |
| `GITHUB_OUTBOUND_INTERVAL` | no | Poll interval, default `2s`. |
| `CSM_PORTAL_BASE_URL` | yes | **Must be the Postgres-backed portal.** Every comment embeds a link built from this; a ServiceNow-backed host produces links that cannot resolve the record they describe. |
| `GITHUB_LABEL_*`, `GITHUB_LABELS_*` | no | Label overrides. Defaults reproduce ServiceNow's values exactly. An unparseable override is a **startup error**, never a silent fallback. |

`DATA_SOURCE=postgres` and the `DB_*` settings are required — the triggers and
the queue live in that database.

## 3. Migrations

Apply `000067`–`000070` in order. They add `case.github_issue_number`,
`account_github_repo`, `github_webhook_delivery`, `github_outbound_queue`, the
CR number sequence, and four triggers.

Verify afterwards that the triggers **exist** — applying cleanly says nothing
about what a migration contains:

```sql
SELECT tgname FROM pg_trigger WHERE NOT tgisinternal AND tgname LIKE '%github%';
-- expect exactly four:
--   change_request_github_outbound        CR state and planned dates
--   work_item_assignment_github_outbound  assignment, CR and case both
--   case_github_outbound                  case closure
--   comment_github_outbound               case comments
```

## 4. Two gates — why a deploy is safe before seeding

Nothing reaches GitHub until **both** are open:

1. the case's account has an **active row** in `account_github_repo`, and
2. the case has a **`github_issue_number`**.

So deploying with an empty `account_github_repo` is inert, however many cases
and change requests exist. Seeding is what turns it on, one account at a time.
The same property makes the test script safe on a populated database.

## 5. Seeding from ServiceNow

Run `scripts/sn-extract-github-config.js` in a background script window on
**prod SN**. It is read-only. It reports:

- `github.dispatch.config` — account → owner/repo/credential, and **emits the
  SQL** to insert them, plus a query naming any account that did not match.
- `git.integration.user-id` → your `GITHUB_INTEGRATION_LOGIN`.
- `git.valid.org.list` — diagnostic. Non-empty means the old inbound
  integration has been **dead** (a `.spit(",")` typo throws and the outer catch
  swallows it), so check real traffic before treating its behaviour as a
  requirement.
- which repositories are actually referenced, and how recently.

**If a credential in that property looks like an inlined token rather than a
name, rotate it at cutover.** `credential_ref` stores a *name* that resolves in
the platform secret store; it is not a place to put a secret.

## 6. Per-repository setup

For each repository in `account_github_repo`:

1. **Settings → Webhooks → Add webhook**
   - Payload URL: `https://<entity-service>/webhooks/github`
   - Content type: `application/json`
   - Secret: the `GITHUB_WEBHOOK_SECRET` value
   - Events: **Issues** and **Issue comments**
2. Grant the PAT **Issues: read & write** on that repository.
3. Confirm the bot account's login matches `GITHUB_INTEGRATION_LOGIN`.

The gateway must forward the body **byte for byte**. The signature covers the
raw bytes, so any re-serialisation invalidates it and every delivery 401s.

## 7. Post-deploy test

```bash
cp scripts/e2e.env.example ~/e2e.env   # outside the repo — it holds secrets
$EDITOR ~/e2e.env
ENV_FILE=~/e2e.env scripts/github-integration-e2e.sh
```

Twenty checks across both halves: all four triggers fire, every row delivers,
comments link to the configured portal, no email address or raw UUID reaches a
public issue, replays are refused, bad signatures 401 without leaking the
expected value, and the loop guard drops our own events.

It opens both gates for one account and one case, and closes them again in a
trap — **including on failure**. Every row it changes is captured as SQL first
and restored afterwards. Point `TEST_ACCOUNT_NAME` at a test account: the run
closes the case and reassigns it before putting it back.

## 8. Rollback

Set `GITHUB_INTEGRATION_ENABLED=false` and redeploy — the worker stops and the
route unmounts. To stop pushes without a deploy:

```sql
UPDATE account_github_repo SET is_active = FALSE;   -- closes gate 1
```

Queued rows stay queued and resume when it is re-enabled. `000067`–`000070`
each have a working `.down.sql`.

## 9. Known gaps

- **Real GitHub delivery of the inbound webhook is unproven.** Outbound dispatch has been driven end to end against a real repository; the inbound half is tested with signed local POSTs, not GitHub's own delivery.

- **Filing an issue is now native, but only where the integration is enabled.**
  `POST /cases/{id}/github-issues` files against GitHub and writes
  `case.github_issue_number`, which is what opens gate 2 for that case. Where
  `GITHUB_INTEGRATION_ENABLED` is unset the route still proxies to ServiceNow,
  so cutover stays per account.

- **ServiceNow's signature oracle is still live.** Its `Authenticator` returns
  the computed HMAC in the 401 body, so anyone who can reach that endpoint can
  sign arbitrary payloads. Two lines to delete; unrelated to this service, but
  exploitable until SN is switched off.
