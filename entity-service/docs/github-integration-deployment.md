# GitHub integration — deployment and post-deploy test

What to configure, what to point where, and how to prove it works after a
deploy. Replaces ServiceNow's `[GitHub Integration]` flows and the per-repository
`servicenow-config.yml` that hardcoded an account sys_id in each customer repo.

**Status:** code merged (#1827, #1918, #1919). One fix outstanding — see
"Before production". Not yet reachable from GitHub in any deployed environment.

## 1. What runs where

```
GitHub  ──webhook (HMAC)──▶  entity-service  POST /webhooks/github   [inbound]
                                   │
                                   ├─ a validated issue becomes a SERVICE REQUEST
                                   └─ an issue comment becomes a case comment

Postgres triggers ──▶ github_outbound_queue ──▶ worker ──▶ repository_dispatch  [outbound]
                                                              │
                                                              ▼
                                        workflows in the customer's own repository
```

Both halves live in **entity-service**. There is no separate deployable.

Outbound never calls the Issues API directly: it fires `repository_dispatch`,
and the repository's own workflows do the posting. They own the comment wording,
labels and issue state — duplicating that here would diverge the moment either
side changed.

**An issue creates a service request, not a change request.** The template sets
the catalog and carries the class as `sr_type`. A change request is raised later,
by a person, in the portal, with the approval path and planned window an issue
cannot supply.

## 2. Configuration

Four settings are an **all-or-nothing gate** (`Config.HasGithubIntegration`).
Miss one and the routes are never registered — the endpoint returns **404**, not
an error, which is the single most confusing failure mode here.

| Variable | Required | What it is |
|---|---|---|
| `GITHUB_INTEGRATION_ENABLED` | yes | Exactly `true`. |
| `GITHUB_WEBHOOK_SECRET` | yes | `openssl rand -hex 32`. This *is* the endpoint's authentication. The same value goes in every repository's webhook settings. |
| `GITHUB_TOKEN` | yes | Needs **`repo` scope** — outbound calls `POST /repos/{owner}/{repo}/dispatches`. |
| `GITHUB_INTEGRATION_LOGIN` | yes | The login this service raises issues under, from a case. **Not** `github-actions[bot]` — see §3. |
| `CSM_PORTAL_BASE_URL` | yes | **Must be the Postgres-backed portal.** Every outbound comment embeds a link built from this; a host backed by another database cannot resolve the record id. |
| `AUTH_INTERNAL_CLIENT_IDS` | yes* | Must include any client calling `POST /github/service-requests`; that endpoint rejects everything else. |
| `GITHUB_OUTBOUND_INTERVAL` | no | Worker poll interval, default **15s**. |
| `GITHUB_LABEL_*`, `GITHUB_LABELS_CLASS` | no | Label overrides; defaults match `labels.yml`. An override that does not parse, or whose class value is outside *Normal / Standard / Emergency Change*, is a **startup error**, never a silent fallback. |

`DATA_SOURCE=postgres` and the `DB_*` settings are required — the triggers and
the queue live in that database.

> `scripts/e2e.env.example` still shows `GITHUB_INTEGRATION_LOGIN=csm-sync-bot`.
> That value caused a live comment loop during testing. Do not copy it.

## 3. Two identities, and why the difference matters

| Identity | What it does | Guarded against |
|---|---|---|
| `github-actions[bot]` | runs the repository's workflows — labels and comments. **Never opens an issue.** | its *comments* only |
| `GITHUB_INTEGRATION_LOGIN` | what this service raises issues under, from a case | its *issues* and comments |

An `issues` event is judged on the **issue's author**, so an issue we raised from
a case cannot come back and become a service request. An `issue_comment` is
judged on any of our identities, because a comment is the only thing we put back
onto an issue and therefore the only thing that can return as new content.

Treating every integration identity as disqualifying on `issues` breaks creation
outright: the validation workflow applies `validation-passed` **as the bot**, and
that is the delivery the validation gate waits for. Fixed in #2000; a deployment
without it will pass validation and create nothing.

## 4. Migrations

Apply `000067`–`000074` in order. Then verify the objects exist — applying
cleanly says nothing about what a migration contained:

```sql
-- the issue link lives on work_item, not on "case" (000071)
SELECT 1 FROM information_schema.columns
 WHERE table_name='work_item' AND column_name='github_issue_number';

-- one record per issue, per account (000073)
SELECT 1 FROM pg_indexes WHERE indexname='work_item_account_github_issue_uniq';

-- numbering for issue-raised records (000072)
SELECT proname FROM pg_proc WHERE proname LIKE 'next_github_service_request%';
--   next_github_service_request_number     -> SR-GH-000001
--   next_github_service_request_wso2_id    -> WSO2-GH-000001  (required by
--                                             work_item_wso2_id_required_by_type)

SELECT tgname FROM pg_trigger WHERE NOT tgisinternal AND tgname LIKE '%github%';
--   change_request_github_outbound          CR state and planned dates
--   work_item_assignment_github_outbound    assignment
--   case_github_outbound                    closure
--   comment_github_outbound                 comments
--   service_request_github_outbound         SR closure
--   service_request_created_github_notice   the "Created in CSM" announcement
```

`000071` replaces four trigger functions **in the same transaction** that moves
the column they read. That is deliberate: `000069` had already shipped, so
editing it in place would leave every existing database running the old bodies
against a column that no longer exists.

## 5. Two gates — why a deploy is safe before seeding

Nothing reaches GitHub until **both** are open:

1. the account has an **active row** in `account_github_repo`, and
2. the work item carries a **`github_issue_number`**.

So deploying with an empty `account_github_repo` is inert, however many records
exist. Seeding is what turns it on, one account at a time.

Nothing about the repository, account, project or team is hardcoded anywhere in
the service. The account is resolved from the database on **every delivery**,
case-insensitively, honouring `is_active` — so repointing a repository is one
`UPDATE`, with no redeploy and no change in the customer's repository.

## 6. Per-repository setup

1. Insert the mapping:
   ```sql
   INSERT INTO account_github_repo (id, created_on, updated_on, created_by,
                                    updated_by, account_id, owner, repository, is_active)
   SELECT gen_random_uuid(), NOW(), NOW(), 'onboarding', 'onboarding',
          a.id, '<owner>', '<repo>', TRUE
     FROM account a WHERE a.name = '<account name>';
   ```
2. Install in the customer's repository: `csm_validate_issue.yml`,
   `sn_case_updates.yml`, `sn_comment_to_github.yml`, `sn_cr_notifier.yml`,
   `labels.yml`, and the issue templates.
3. Set the repository variable **`CSM_PORTAL_URL`** explicitly. The workflow
   falls back to a hardcoded URL when it is unset, which is the one piece of
   per-repo hardcoding the port did not remove.
4. **Settings → Webhooks → Add webhook**
   - Payload URL: the **public Choreo endpoint** + `/webhooks/github`
   - Content type: `application/json`
   - Secret: the `GITHUB_WEBHOOK_SECRET` value
   - Events: **Issues** and **Issue comments** only
5. Set `account.cre_team_id` if these records should carry an ABT team (§9).

The gateway must forward the body **byte for byte**. The signature covers the raw
bytes, so any re-serialisation invalidates it and every delivery 401s.

## 7. Post-deploy test

```bash
cp scripts/e2e.env.example ~/e2e.env   # outside the repo — it holds secrets
$EDITOR ~/e2e.env
ENV_FILE=~/e2e.env scripts/github-integration-e2e.sh
```

It opens both gates for one account and one case and closes them again in a
trap, **including on failure**; every row it changes is captured as SQL first and
restored afterwards.

Then confirm the live path by hand, in this order — each step rules out the one
before:

1. `POST` to the public URL unsigned → expect **401**. A **404** means the config
   gate (§2) is incomplete; a timeout means the endpoint is still
   Organization-visibility.
2. Open an issue from a template. GitHub's **Recent Deliveries** shows the
   response — a 401 there is a secret mismatch.
3. Watch the log. The expected sequence is *skipped: not validated* on `opened`,
   then creation on the workflow's `labeled` event:
   ```
   event=issues skipped="issue has not passed template validation yet"
   github: service request created from issue number=SR-GH-0000NN
   ```
4. Confirm the record carries its number, `wso2_id`, account and template fields,
   and that the "Created in CSM" comment reached the issue.
5. Comment on the issue → appears in CSM. Comment in CSM → appears on the issue.
   **Exactly one each way.**

## 8. Rollback

Set `GITHUB_INTEGRATION_ENABLED=false` and redeploy — the worker stops and the
routes unmount. To stop pushes without a deploy:

```sql
UPDATE account_github_repo SET is_active = FALSE;   -- closes gate 1
```

Queued rows stay queued and resume when re-enabled. `000067`–`000074` each have
a working `.down.sql`; `000071`'s restores the shipped trigger bodies.

## 9. Known gaps

- **The webhook is not reachable in any deployed environment.**
  `.choreo/component.yaml` declares one endpoint at `Organization` visibility, so
  GitHub cannot reach `/webhooks/github`. Needs a second endpoint at `Public`
  visibility serving only that path. Tracked in
  [#3140](https://github.com/wso2-enterprise/digiops-cs/issues/3140) — **this is
  the blocker for production.**
- **No routing, no notification.** A service request is created with an account
  and nothing else: no assignee, no team stamped, nobody told. A team is derived
  *through* the account (`account.cre_team_id` → `group.name`, the ABT teams), and
  the case API exposes it as `creTeam`, but only a minority of accounts have it
  set and case search has no team filter. Platform gap, tracked in
  [#2756](https://github.com/wso2-enterprise/digiops-cs/issues/2756).
- **One repository, one account.** `UNIQUE (owner, repository)` prevents a shared
  repository serving two accounts.
- **Workflow updates do not propagate.** The workflows live in each customer's
  repository; changing them here reaches nobody.
  [#3141](https://github.com/wso2-enterprise/digiops-cs/issues/3141).
- **Both sides must never run at once.** Disable a repository's four ServiceNow
  flows as the native path is enabled. Duplicate comments on an issue are the
  first symptom. [#3142](https://github.com/wso2-enterprise/digiops-cs/issues/3142).

## 10. Verified

Against a real repository and database, with GitHub delivering over a webhook:

| | |
|---|---|
| issue → service request | 17 template fields, correct account and catalog, `SR-GH-` and `WSO2-GH-` numbering |
| validation gate | `opened` skipped; the workflow's `labeled` event creates |
| creation announced on the issue | number, portal link, catalog/priority/environment |
| GitHub comment → CSM | relayed once, marked `(GitHub Comment)` |
| CSM comment → GitHub | relayed once |
| assignment → issue | comment plus `Status/Assigned` label |
| concurrency | five simultaneous creates for one issue → one record, one `201`, four `200`s |

## Change log

Keep this list current — it is how the next deploy knows what changed.

- **2026-09-25** — Identity rule split by event type (#2000): the validation
  label from `github-actions[bot]` is no longer discarded. Found on a live
  delivery; without it a validated issue never becomes a record.
- **2026-09-23** — Service requests from issues, creation announcement,
  validation gate, `wso2_id` numbering, issue-uniqueness index, endpoint
  authorization (#1918). Assignee email on the case API (#1919).
- **2026-09-21** — Inbound webhook, outbound queue and dispatch worker, and the
  four ServiceNow flow equivalents (#1827).
