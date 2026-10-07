<!--
Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
-->

# Customer Portal E2E (Playwright, local)

Runs locally against `pnpm run dev` (:3000) or a deployed environment, authenticated by a
**captured browser session** so no login page or 2FA is driven. See
[`auth/README.md`](./auth/README.md) to capture one. To run against the **local compose
stack** instead, see "Local stack" below.

There is no mock backend here: specs hit the same backend the dev server is configured
against (`public/config.js`), so anything a spec creates is a **real record** in that
environment. Tag created data so it stays identifiable.

## Run

Configuration lives in **`.env.e2e`** (at the webapp root, committed), so no env
vars need to be typed on the command line:

```bash
pnpm run test:e2e                          # all specs
node_modules/.bin/playwright test --ui     # author/debug interactively
node_modules/.bin/playwright show-report   # open the last HTML report
```

> Note: `pnpm exec playwright …` fails in this repo ("packages field missing");
> call the binary directly via `node_modules/.bin/playwright …`.

### Configuration

`playwright.config.ts` loads the env files itself (via node's built-in
`process.loadEnvFile`, no dotenv dependency). Precedence, highest first:

1. **real environment / CLI** — `E2E_BASE_URL=… pnpm run test:e2e`
2. **`.env.e2e.local`** — your personal overrides, git-ignored (`.env.*.local`)
3. **`.env.e2e`** — committed team defaults

| Var | Effect |
|---|---|
| `E2E_BASE_URL` | Environment under test. Default in `.env.e2e` is staging; falls back to `http://localhost:3000` if unset everywhere |
| `E2E_NO_WEBSERVER=1` | Don't boot the local dev server — required when `E2E_BASE_URL` points at a running deployment |
| `CI` | `forbidOnly`, and never reuse an already-running dev server |

No secrets belong in these files. Login is by replaying
`storageState/session.json` (git-ignored), not by credentials in env vars.

### Base URL must match the captured session

**The captured bundle decides which environment you can run against** — it only
restores into the origin it was captured from (see `fixtures/test.ts`).
`.env.e2e` ships pointing at staging because that is where the current
`session.json` was captured.

To run against the local dev server instead: recapture `session.json` while
signed in at `http://localhost:3000`, then create `.env.e2e.local` with

```bash
E2E_BASE_URL=http://localhost:3000
# Must be set empty, not omitted: keys absent from .env.e2e.local still come
# from .env.e2e, and an empty value reads as falsy so Playwright boots
# `pnpm run dev` itself.
E2E_NO_WEBSERVER=
```

`withSession()` skips (rather than fails) any test whose session bundle is
missing or captured against a different origin than the run targets, and the
skip message names the mismatch.

## Local stack (docker-compose + the mock identity provider)

Everything above targets a deployed environment and signs in as a staging account. To run
against the **local compose stack** instead (`docker-compose up -d`; customer webapp
`http://localhost:3000`, customer backend `:8090`, mock OIDC provider `http://localhost:9100`),
sign in as one of the customers the local seed registers on its projects
(`scripts/csm-compose/seed-entity-service.sql`; "Local seed personas" in
`entity-service/CLAUDE.md`):

| Persona (`E2E_LOCAL_PERSONA`) | Email | Registered contact of |
|---|---|---|
| `dave` | `dave.mendis@example.com` | "Example Corp Production" (project `00000000-0000-0000-0000-000000000401`) |
| `erin` | `erin.jayawardena@example.com` | same project |
| `mira` | `mira.santos@lumenworks.example` | "Lumen Works Platform" (a generated project: found by name, its id is random per database) |
| `noel` | `noel.prasad@lumenworks.example` | same project |

The mock provider signs in **any** email with no credential check, so nothing here needs a
password or TOTP seed. A session is minted by driving the app's own sign-in once and is then
replayed like any hand-captured bundle:

```bash
# from apps/customer-portal/webapp, with the stack up and seeded
for p in dave erin mira noel; do
  E2E_LOCAL_PERSONA=$p pnpm run test:e2e:local-auth      # writes tests/e2e/storageState/local-$p.json
done
node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium
```

* **Groups stay empty.** The mock provider's sign-in form pre-fills its Groups box with
  `cs_engineer`, a CSM *staff* group, and copies it into the token verbatim. The mint step clears
  it and asserts the signed-in user holds only the `customer` role. What a customer may see comes
  from their user record (role `customer`, a registered portal contact of a project), not from
  the token.
* **Tokens last one hour** and the provider issues no refresh token. Re-run the mint line to
  refresh a bundle (it overwrites). A spec whose bundle is missing, expired, minted for another
  origin than `E2E_BASE_URL`, or whose stack does not answer, is **skipped** with the reason, never
  failed.
* **Other ports.** The webapp origin is part of a bundle; mint and run against the same one:
  `E2E_BASE_URL=http://localhost:13000 E2E_LOCAL_PERSONA=dave pnpm run test:e2e:local-auth`, then
  `E2E_BASE_URL=http://localhost:13000 node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium`.
  (`.env.e2e` already sets `E2E_BASE_URL=http://localhost:3000` and `E2E_NO_WEBSERVER=1`.)
* **The auth setup project** of the main config (`auth/auth.setup.ts`, the staging sign-in) skips
  itself when no staging credentials are set, so these specs run without any.
* **Fixtures move.** The seeded `CHG-FIXED-*` change requests are driven forward by whoever
  approves or rejects them; `docker-compose up -d migrate` re-runs the (self-healing) seed and puts
  them back (wait for the `migrate` container to exit before running a spec, e.g.
  `docker-compose up -d migrate && docker wait <project>-migrate-1`). The smoke spec changes
  nothing; `customer-change-request-approval.spec.ts` answers `CHG-FIXED-007` and `-008`, and skips
  (never fails) a fixture an earlier run already answered, naming the seed re-run that resets it. The
  state-changing specs below re-seed the stack's Postgres themselves, before every test and once more
  when their file is done, so the specs that sort after them still find the fixtures waiting.
* **entity-service runs as a plain role.** The compose stack connects entity-service as `csm_app`
  (no superuser, no `BYPASSRLS`; created by `migrate-and-seed.sh`), so the project-membership
  row-level security applies as it does in production: a customer asking for another project's
  change request gets a 404, not its contents. As the `postgres` superuser every such read answered
  200.
* **Operations needs the seed.** The menu appears only when the project's type grants change request
  / service request read access; the seed sets that on the local "Subscription" type. A database
  seeded before that existed shows no Operations menu until `migrate` is re-run.

| File | Purpose |
|---|---|
| `auth/local-session.setup.ts` | Mints `storageState/local-<persona>.json` (run through `playwright.local-auth.config.ts`, a separate config so a regression run never mints as a side effect) |
| `auth/localSessions.ts` | `LOCAL_PERSONAS`, `withLocalSession(test, "dave")` (replays the bundle; skips on missing / expired / wrong-origin / unreachable), `sessionMinutesLeft` |
| `specs/local/customer-change-requests.spec.ts` | Smoke: dave lists `CHG-FIXED-007` under Operations > Change requests (Customer Approval on a fresh seed), and every change request the list API returns for the project belongs to it |
| `specs/local/customer-change-request-approval.spec.ts` | Writes, and needs nothing but the stack and the sessions (no database access; it reaches the change requests through the list and skips a fixture an earlier run answered): dave approves `CHG-FIXED-007` (Customer Approval -> Scheduled, banner kept, buttons gone, list and erin's view agree) and confirms `CHG-FIXED-008` as Successful (Customer Review -> Closed), through the real chain; re-run the seed to reset |
| `specs/local/customer-change-request-answer.spec.ts` | **State-changing.** A customer answers: dave approves `CHG-FIXED-007` (success banner, Scheduled in the page and the list, erin no longer offered Approve) and a stale tab of erin's gets the plain-words 409 (the backend's `change_request_approval_not_pending` code, not its wording, decides it) with focus on the heading; dave rejects (confirmation first, "Go back" changes nothing, then Canceled); at Customer Review (`CHG-FIXED-008`) Successful closes it, Unsuccessful asks first and sends it to Rollback. Each test reads the answer back from the API and from Postgres |
| `specs/local/customer-change-request-propose.spec.ts` | **State-changing.** Propose New Time: the dialog asks for a start AND an end and an empty / past / inverted window shows inline errors and sends nothing; the whole loop for a Normal change (dave proposes, the change goes to Authorize and the buttons go for dave and erin, alice approves as CAB through the CSM portal's backend, both are asked again with fresh requests, erin approves the new time; proposing the window already on the change is refused in the dialog); the loop repeated (the start drags the end along, a second CAB round decided by bob, the whole approver history kept); a Standard change stays in Customer Approval and asks both again at once; a change on hold has Propose New Time switched off with the reason beside it (a hold placed after the dialog was opened is still refused with the reason, the dialog stays open and focus stays on its Submit button; the reject confirmation then says the change is on hold instead of pointing at Propose New Time), and still takes an answer; a page opened before the change was re-scheduled cannot approve the new window (the schedule-changed message, nothing recorded, focus on the heading); after a proposal the page keeps saying WSO2 is reviewing it, and focus is on the heading; Enter in the dialog submits |
| `specs/local/customer-change-request-access.spec.ts` | **State-changing in the small.** Who may not, and who always may: a change request is visible to a customer only once it was designated to them (asked of them at Customer Approval / Customer Review, in any status of their own request) and then in every later state. mira (another project) gets a 404 on `CHG-FIXED-007` by id, in its approvals, on the decision and on every PATCH, sees it in no list, and gets the not-found page at either address; `CHG-FIXED-005` (New) and `CHG-FIXED-006` (Review, nobody asked) are a 404 for dave in every call, not listed under any state filter, the not-found page by address, and still not there when one is moved on to a customer state (the state alone shows nothing); `CHG-FIXED-007` stays visible to dave, listed and openable, in Authorize, Scheduled, Implement, Review, Customer Review, Rollback, Closed and Canceled, with nothing to answer; a contact whose own request was cancelled still sees the change (no buttons, her answer refused) at Customer Approval and Customer Review; a direct PATCH from dave's token with anything but an answer or a proposed time is 403 and moves nothing |
| `specs/local/customer-change-request-contrast.spec.ts` | Needs `E2E_POSTGRES_CONTAINER` (it re-seeds; nothing is answered). In light and in dark mode, the text of Approve, Reject, Propose New Time, Successful, Unsuccessful and the reject confirmation's button reaches WCAG AA (4.5 : 1) against the colour actually painted behind it, sampled from a screenshot because the page background is a gradient (`utils/contrast.ts`) |
| `specs/local/customer-change-request-visibility.spec.ts` | **State-changing.** WSO2 staff RAISE change requests on Lumen Works Platform through the CSM portal's backend (jane raises, alice and bob approve) and walk them through the real approval flow, while mira and noel (and dave, of another project) are watched in the API and in the UI at every step, on the numbers: the list, the stat cards and the dashboard count (against what the project said before), the detail, the approvals and a refused write. Both boxes ticked: invisible in New, Assess and Authorize; visible to BOTH contacts at Customer Approval; mira proposes a new time and it STAYS visible to mira and noel in Authorize (list and detail, with the Authorize label); CAB again, mira approves, Implement, Review, Customer Review (noel answers), Closed: visible all the way; dave never. A change request that never needs the customer (Normal, Standard, or with no project) is never visible in any state; one that needs only the review is invisible until Customer Review; a contact registered after the stage was provisioned sees nothing of it; a contact whose registration ended sees nothing (and everything again once re-registered), and a change request the sync moved to another project is nobody's there; the approvals a customer reads hold the customer rows by name and every internal stage as a label and a status only. A hidden change request is also absent from the Operations hub, a search by its number and the global search, is not commentable through the case route and reads, on every case-like route that takes any id, exactly as an id that exists nowhere (no oracle). Also straight at entity-service (`E2E_ENTITY_SERVICE_URL`): a hidden change request is a 404 to a read, the approvals, an answer and a field no customer may set |
| `specs/local/customer-change-request-legacy.spec.ts` | **State-changing.** Change requests MIGRATED from ServiceNow (`fixtures/legacy-change-requests.sql`: ServiceNow-style ids and numbers, created in 1999, i.e. before the local cutover `CR_STRICT_VISIBILITY_FROM=2000-01-01T00:00:00Z`, no customer-stage rows, flags false, some stages with no label): customers see them exactly as today (every state but New, Assess and Authorize; the row created one second before the instant is shown, the one at the instant is not; the cards count the list), a change request raised after the cutover in the very same state is not; a legacy one in Customer Approval / Customer Review with no live stage (the "Demo Test 1" shape) is read without writing anything and answered by the first contact (stage provisioned once, both contacts asked, the answer recorded, still visible, a second answer a 409); a legacy one a customer proposes a new time on stays visible in Authorize to both contacts while an untouched legacy Authorize row never is, and once the CAB approves the new time both contacts are asked again (a proposal is a Re-schedule, which writes the customer's approval as required of the row) and the second contact's approval schedules it (WSO2 can still tick the box in Authorize: add-only, idempotent after a proposal); a sync-mirrored stage with no label keeps its pending approver pending across state moves, is decided by that approver (an Emergency change in Authorize), and is shown to a customer as a label and a status only |
| `specs/local/customer-change-request-dates.spec.ts` | **State-changing.** The planned time is validated by the SERVER: through the customer backend and straight at entity-service, a customer's proposal with `tomorrow`, `now`, `infinity`, a date with no time, a past start, an end equal to or before the start, a year outside 2000 to 2100, garbage or an empty string is a 400 in the service's words and changes nothing (state, window, approver rows, `updated_on`); a good window right after still works, the whole loop (an RFC 3339 offset stored as the UTC instant, the CAB approves, erin approves); an end-only proposal cannot keep a start that has passed; the same strings are refused on staff PATCH and on create, and a refused create leaves no row |
| `utils/localStack.ts` | What those specs need beyond a browser session: resetting the fixtures (the seed, run in the stack's Postgres), reading the raw rows, the customer's own API calls (any email, with the stat cards and the dashboard count), staff raising and walking a change request through the CSM portal's backend (`raiseChange`, `staffApi`, `staffDecides`, `staffMoves`), the legacy rows, a contact registered late, calls straight at entity-service, screenshots (`E2E_SHOT_DIR`) and wall-clock helpers for the Propose dialog |
| `pages/ChangeRequestDetailsPage.ts` | The change request detail page, for every spec above: Approve / Reject / Propose New Time / Successful / Unsuccessful, the banners, the confirmations, the Propose dialog, the lifecycle panel's current stage (the copy lives in `CHANGE_REQUEST_DETAILS` in `utils/selectors.ts`) |

### State-changing local specs (answer, propose, access, visibility, legacy, dates)

The `customer-change-request-{answer,propose,access,visibility,legacy,dates}.spec.ts` specs (and the contrast spec, which re-seeds) approve, reject and re-schedule
the seeded fixtures for real, so they need a little more than a session. They **skip with the reason**
(never fail, never guess) unless the stack under test is named:

| Variable | Needed by | Meaning |
|---|---|---|
| `E2E_POSTGRES_CONTAINER` | all of them | The Docker container of the stack's Postgres, e.g. `csm-platform-postgres-1` for the stock compose project or `csmenv-postgres-1` for a second stack. The specs re-run `scripts/csm-compose/seed-entity-service.sql` in it (`docker exec … psql`) before every test and read raw rows back. There is **no default**: a name that was guessed could reset somebody else's stack. After each re-seed the spec reads the fixtures back *through the app under test* and fails with that explanation if they do not show up, which is what a container of the wrong stack looks like |
| `E2E_CSM_BFF_URL` | `…-propose`, `…-visibility`, `…-legacy`, `…-dates` | The CSM portal's backend as the browser reaches it (`http://localhost:8082` stock). The loop has WSO2 staff (alice, the CAB) approve the new time through its `POST /change-requests/{id}/approvals/decision`, the same call as the CSM portal's Approvals tab |
| `E2E_ENTITY_SERVICE_URL` | `…-visibility`, `…-dates` (optional) | entity-service's own port (`http://localhost:8081` stock, `http://localhost:18081` for a second stack). With it, those specs also call the service straight, as the customer backend would (its machine token plus the customer's `x-user-id-token`), to prove a rule is the service's and not only the backend's; without it those calls are skipped |
| `E2E_CR_STRICT_VISIBILITY_FROM` | optional | The cutover instant the stack's entity-service runs with (`CR_STRICT_VISIBILITY_FROM`; default `2000-01-01T00:00:00Z`, the local compose value). Only `…-legacy` reads it: it puts its two boundary rows one second before and exactly at that instant. It must be after 1999-12-20, the age of the legacy rows |
| `E2E_SHOT_DIR` | optional | A directory: the specs save screenshots of their key screens there (`utils/localStack.ts` `shot`) |
| `E2E_POSTGRES_USER`, `E2E_POSTGRES_DB` | optional | Default `postgres`, `csm_platform` |

The customer backend and the mock identity provider are not configured: the specs read them from the
webapp's own `/config.js`, so an API call can never go to another stack than the browser does.

```bash
# from apps/customer-portal/webapp, the stack up and seeded, sessions minted for dave, erin and mira
export E2E_BASE_URL=http://localhost:13000            # the customer webapp of the stack under test
export E2E_POSTGRES_CONTAINER=csmenv-postgres-1
export E2E_CSM_BFF_URL=http://localhost:18082
for p in dave erin mira noel; do E2E_LOCAL_PERSONA=$p node_modules/.bin/playwright test --config=playwright.local-auth.config.ts; done
node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium --reporter=list
```

The Propose specs pin the browser's time zone to `America/New_York` and read the zone the dialog says it
uses ("Times are in your time zone: …"), then assert that the wall time typed there is what Postgres
holds in UTC, and that the dialog shows the very same wall time again once the change is back in
Customer Approval.

The sessions last one hour: re-mint before a long or repeated run. The specs leave the fixtures re-seeded,
so the read-only smoke spec that sorts after them still finds `CHG-FIXED-007` in Customer Approval.

## Layout

| Path | Purpose |
|---|---|
| `auth/README.md` | How to capture a session bundle (localStorage + sessionStorage) |
| `fixtures/test.ts` | `withSession(test)` replays `storageState/session.json`, skipping each test that uses it when the bundle is absent or captured against a different origin (it skips from `beforeEach`, so tests are reported individually as skipped rather than the file being skipped as a unit); `openContextAs(browser, name)` opens a second authenticated context |
| `pages/` | Page objects — one per screen, no assertions inside |
| `specs/` | The specs, grouped in subfolders by feature area |
| `utils/` | Shared selectors / data-tagging helpers |
| `storageState/` | Captured session bundles — **git-ignored, real tokens** |

## Roles

One session (`session.json`) is captured today, so specs run as whatever that
account is. For role-gated coverage, capture additional bundles as
`storageState/<name>.json` and pass the name to `withSession(test, name)` or
`openContextAs(browser, name)`.

Each bundle should be captured from an account holding one role on the project
under test, so a spec can assert what that role can and cannot do:

- **admin** — manages users and registry service tokens in Settings.
- **lead** — a portal user who can also escalate a case past EL3.
- **portal** — the baseline: signs in, creates and manages cases.
- **security** — receives security advisories and raises security reports.

Project-level feature visibility (Operations, Security Center, Updates,
Engagements, Usage & Metrics …) is independent of all of these — it comes from
`GET /projects/{id}/features`. Pick the project a spec runs against
accordingly.
