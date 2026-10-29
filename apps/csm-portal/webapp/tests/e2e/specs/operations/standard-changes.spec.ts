// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// Standard changes, end to end on the real local stack, through the UI:
//
//   1. Operations > Change requests has a two-entry strip, "Change requests"
//      (the list, no `view`) | "Standard changes" (`?view=standard`). Standard
//      changes opens on the catalogue; the proposals are its secondary view
//      (the "Templates or proposals" toggle, `&show=proposals`, with `scope` and
//      `state` in the URL; the scope select is named "Show", a name the toggle
//      does not share). The proposals open on all proposals (no `scope`). Both
//      are headed "Standard changes" with the same line, which says nothing
//      about service requests, incidents, problems or runbooks; every other
//      Operations tab (each one checked once the sidebar has rendered it) keeps
//      the Operations heading. Entering Standard changes keeps the list's `cr*`
//      filters in the URL, unread; the "Change requests" tab drops only
//      Standard changes' parameters, so the list comes back with its filter
//      applied. Every old link (the separate tab, in both spellings and both
//      forms, each with `view=catalogue` / `view=proposals` and without, and the
//      previous `?view=catalogue` / `?view=proposals` sub-views, with a scope,
//      a state or the list's `cr*` filters) lands on the right place with its
//      filters kept, and a rewrite of an old sub-view adds no history entry.
//      The catalogue lists the seed's five templates
//      (scripts/csm-compose/seed-standard-changes.sh) with their states and
//      versions, the suspended one's page shows the seed's reason, the
//      proposals list the seed's rejected, in-CAB-review and draft proposals
//      and its revision, the breadcrumbs read "Change requests › Standard
//      changes › [Proposals ›] <number>" (the "Proposals" crumb opens all the
//      proposals), Back returns to the view a record was opened from, "Pin
//      this page" names each place ("Standard changes" however the list was
//      filtered, "Standard change proposals: 1 filter", a template by its
//      number) and the sidebar's "Change requests" is the one active entry
//      throughout.
//   2. The "needs your attention" strip above the catalogue: hidden for jane,
//      who has nothing pending; for carol (a CAB member asked about the seed's
//      proposal in CAB review) and bob (who has the seed's draft) it reads the
//      counts Postgres holds, in words, each part a link to the proposals
//      filtered to it, which lists that seeded proposal. Hidden on the
//      proposals view, which for jane, who proposed nothing, opens on all the
//      proposals: bob's seeded ones.
//   3. bob proposes a standard change from a CLOSED change request (its detail
//      page's "Propose as standard change"), templates it (name, category,
//      impact Medium -- not the create form's default, Low -- description,
//      justification, risk and impact analysis, plan visible to customers, a
//      text and a select parameter, {{placeholders}} in the subject and the
//      implementation, backout and test plans, typed and inserted with "Insert
//      {{…}}"). The live preview shows every placeholder highlighted and open
//      ({{host}} / {{env}}) before sample values are typed, open and filled
//      side by side (styled differently: the filled one with a dotted
//      underline) once only the host is typed, and filled with them all
//      after. He saves and submits it to the CAB; carol finds it through her
//      attention strip and approves it; the template's page shows the current
//      version with its placeholders highlighted, and it is in the catalogue as
//      active version 1; alice raises a change request from it on the create
//      form (type Standard, the template, the change specifics, the read-only
//      preview). The change request holds the template's content rendered with
//      her values (subject, the three plans, impact, risk, category, rollback
//      duration on the page; description, justification, risk and impact
//      analysis, plan visibility, impact and risk from the API and Postgres,
//      compared with the template's own version), shows the "Raised from
//      standard change template" banner, its lifecycle bar never passes through
//      Assess or Authorize, and in Edit every field the template specifies is
//      disabled AND shows the template's value while the others (priority,
//      assignment, affected services) stay editable. The backend holds the same
//      line: a PATCH through the BFF that changes a field the template
//      specifies is refused with 400 change_request_standard_template_field,
//      while resending the stored value with a work note is accepted. She
//      schedules it (Request Approval reads "Schedule"), implements, reviews
//      and rolls it back; the template then reads "Review needed", linking to
//      that change request.
//
// It WRITES to the stack: a closed change request raised and walked through the
// CSM backend's API (the evidence), the proposal, the template and the change
// request raised from it, all named with TAG (and a work note on that change
// request). Before and after, the clean-up deletes from Postgres everything
// carrying TAG (and every change request raised from a TAG template), with the
// event-outbox rows of the records it deletes, and nothing else. So it runs only
// when the stack is named:
//
//   E2E_NO_WEBSERVER=1 E2E_POSTGRES_CONTAINER=csmcr-postgres-1 \
//   E2E_CSM_BFF_URL=http://localhost:8082 E2E_OIDC_URL=http://localhost:9100 \
//   E2E_ENTITY_SERVICE_URL=http://localhost:8081 \
//     npx playwright test tests/e2e/specs/operations/standard-changes.spec.ts
//
// The seed must have been run first (scripts/csm-compose/seed-standard-changes.sh).
// The personas sign in through the local mock identity provider (no password):
// bob proposes, carol decides as a member of the "CAB Approval" group (the
// proposer cannot decide his own), alice raises and moves the change request,
// jane (in no approval group, with no proposals) has nothing pending.

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { ChangeRequestDetailPage } from "../../pages/ChangeRequestDetailPage";
import { EXAMPLE_CORP_ABT_GROUP_ID, realStackNamed, staff, STAFF, walkTo } from "../../utils/realStackApi";

const POSTGRES_CONTAINER = process.env.E2E_POSTGRES_CONTAINER?.trim();
const BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:3001";

/** Every record this spec writes starts with it; the clean-up deletes by it. */
const TAG = "QA-STD-E2E";
const RUN = Date.now().toString(36);
const SOURCE_SUBJECT = `${TAG} Restart the cache service on cache-01 (${RUN})`;
const TEMPLATE_NAME = `${TAG} Restart the cache service on a node (${RUN})`;
const TEMPLATE_SUBJECT = `${TAG} Restart the cache service on {{host}} ({{env}})`;
const SOURCE_BACKOUT = "Start the previous cache build again and confirm it serves reads.";
const SOURCE_TEST = "Read a known key and compare it with the primary.";

/**
 * What the template specifies, as bob writes it, and how each field reads once the
 * {{host}} / {{env}} placeholders are filled. The implementation plan ends with a
 * {{host}} and the test plan with an {{env}} inserted through "Insert {{…}}".
 */
const TEMPLATE = {
  category: "Software",
  /** Not the create form's default (Low), so the change request's impact can only have come from the template. */
  impact: "Medium",
  risk: "Low",
  description: "Restart the cache service on one node to clear a stuck eviction loop.",
  justification: "A stuck eviction loop slows every read on the node; a restart clears it.",
  riskImpactAnalysis: "One node is out of the pool for about a minute; the others serve its reads.",
  /** Not the create form's default (off), as above. */
  planVisibleToCustomers: true,
  rollbackDuration: "15 mins",
  subject: (host: string, env: string) => `${TAG} Restart the cache service on ${host} (${env})`,
  implementationTyped: "Drain {{host}} in {{env}}, restart the cache service, put it back in the pool on ",
  implementation: (host: string, env: string) =>
    `Drain ${host} in ${env}, restart the cache service, put it back in the pool on ${host}`,
  backoutTyped: "Start the previous cache build on {{host}} again and confirm it serves reads.",
  backout: (host: string) => `Start the previous cache build on ${host} again and confirm it serves reads.`,
  testTyped: "Read a known key and compare it with the primary in ",
  test: (env: string) => `Read a known key and compare it with the primary in ${env}`,
} as const;

/** The personas (the mock identity provider signs any of them in). jane has nothing pending in standard changes. */
const USERS = STAFF;

/** Where Standard changes lives (`utils/standardChanges.ts`). */
const STANDARD_CHANGES = "/operations/change-requests?view=standard";
/** The URL's path and query, with the query exactly `search` (no `?` for none). */
function exactly(path: string, search = ""): RegExp {
  return new RegExp(`${escapeRegExp(path)}${search ? `\\?${escapeRegExp(search)}` : ""}$`);
}

/** The Operations heading, which every Operations tab but Standard changes keeps. */
const OPERATIONS_HEADING = {
  title: "Operations",
  subtitle: "Service requests, change requests, incidents, problems, and runbooks across customers.",
};
/** Standard changes' heading, the same for the catalogue and the proposals. */
const STANDARD_CHANGES_HEADING = {
  title: "Standard changes",
  subtitle: "Pre-authorised change requests, raised from templates the CAB has approved.",
};
/** What the user complained about: the standard change views headed with the entities they have nothing to do with. */
const OTHER_OPERATIONS_ENTITIES = /service requests?|incidents?|problems?|runbooks?/i;
/** The Operations tabs of the sidebar, as the local stack shows them to bob (each must be there). */
const OPERATIONS_TABS = ["Service requests", "Change requests", "Incidents", "Problem management", "Outages", "Runbooks"];

/**
 * A filter of the change request list (its own `cr*` parameters): a search no change request matches, and a state.
 * Standard changes keeps it in the URL, unread, and the list comes back with it applied.
 */
const CR_LIST_SEARCH = `qa-std-e2e-filter-${RUN}`;
const CR_LIST_FILTERS = `crQ=${CR_LIST_SEARCH}&crStates=closed`;

/**
 * What scripts/csm-compose/seed-standard-changes.sh leaves, by the names it gives (its
 * own text, not read back from the database the page shows).
 */
const SEEDED_TEMPLATES = [
  { name: "Rotate the TLS certificate on an API gateway", version: "v1", state: "Active" },
  { name: "Scale a Kubernetes deployment's replicas", version: "v2", state: "Active" },
  { name: "Apply a WSO2 product update level to a deployment", version: "v1", state: "Active" },
  { name: "Restart an API Manager node", version: "v1", state: "Suspended" },
  { name: "Add an IP range to a customer's gateway allowlist", version: "v1", state: "Active" },
] as const;
const SUSPENDED_TEMPLATE = "Restart an API Manager node";
/** The reason the seed suspends it with (seed-standard-changes.sh replaces an older one on a re-run). */
const SUSPENDED_REASON =
  "The owner is reworking the restart procedure for clustered deployments. Raise node restarts as normal changes until this template is resumed.";
const REVISED_TEMPLATE = "Scale a Kubernetes deployment's replicas";
/** bob's proposals in states other than approved, and his approved revision. */
const SEEDED_PROPOSALS = [
  { name: "Purge API Manager analytics data older than a retention period", kind: "New template", state: "Rejected" },
  { name: "Raise the memory limit of a Choreo component", kind: "New template", state: "CAB review" },
  { name: "Enable debug logging on a Micro Integrator node for a fixed period", kind: "New template", state: "Draft" },
  { name: REVISED_TEMPLATE, kind: "Revision", state: "Approved" },
] as const;
const SEED_HINT = "run scripts/csm-compose/seed-standard-changes.sh against the stack first";

/**
 * With E2E_SHOT_DIR set, saves a picture as `<name>.png` there (for reviewing the UI); otherwise nothing. The app's
 * main area scrolls inside a 100dvh shell, so a full-page picture shows only the top: with `target` the region is
 * scrolled into view and pictured on its own, without it the viewport is.
 */
async function shot(page: Page, name: string, target?: Locator): Promise<void> {
  const dir = process.env.E2E_SHOT_DIR?.trim();
  if (!dir) return;
  fs.mkdirSync(dir, { recursive: true });
  await page.waitForTimeout(400); // let a dialog or menu finish fading in
  const file = path.join(dir, `${name}.png`);
  if (target) {
    await target.scrollIntoViewIfNeeded();
    await target.screenshot({ path: file });
  } else {
    await page.screenshot({ path: file });
  }
}

/** Runs SQL on the stack's Postgres and resolves with what it printed (unaligned, tuples only). */
function psql(sql: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn("docker", [
      "exec", "-i", POSTGRES_CONTAINER ?? "", "psql", "-U", "postgres", "-d", "csm_platform",
      "-v", "ON_ERROR_STOP=1", "-q", "-t", "-A", "-f", "-",
    ]);
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d: Buffer) => (stdout += d.toString()));
    child.stderr.on("data", (d: Buffer) => (stderr += d.toString()));
    child.on("error", reject);
    child.on("close", (code) => (code === 0 ? resolve(stdout.trim()) : reject(new Error(`psql exited ${code}: ${stderr}`))));
    child.stdin.end(sql);
  });
}

/**
 * Deletes what this spec wrote, and only that, in one transaction: the proposals named with TAG (or about a TAG
 * template), the change requests whose subject starts with TAG or that were raised from a TAG template, the TAG
 * templates (a change request holds its template version with ON DELETE RESTRICT, so it goes first), and the
 * event-outbox rows of exactly those records (`event_outbox.entity_id` has no foreign key, so they would otherwise be
 * left behind pointing at nothing). A proposal and a change request are work items: deleting the work item takes
 * their approval stages, evidence and comments with it.
 */
async function cleanUp(): Promise<void> {
  const like = `'${TAG}%'`;
  await psql(`
    BEGIN;
    CREATE TEMP TABLE qa_std_e2e_doomed ON COMMIT DROP AS
      SELECT id FROM work_item
       WHERE type = 'STANDARD_CHANGE_PROPOSAL'
         AND (subject LIKE ${like}
              OR id IN (SELECT p.id FROM standard_change_proposal p
                          JOIN standard_change_template t ON t.id = p.template_id
                         WHERE t.name LIKE ${like}))
      UNION
      SELECT id FROM work_item
       WHERE type = 'CHANGE_REQUEST'
         AND (subject LIKE ${like}
              OR id IN (SELECT cr.id FROM change_request cr
                          JOIN standard_change_template_version v ON v.id = cr.standard_change_template_version_id
                          JOIN standard_change_template t ON t.id = v.template_id
                         WHERE t.name LIKE ${like}))
      UNION
      SELECT id FROM standard_change_template WHERE name LIKE ${like};
    DELETE FROM event_outbox WHERE entity_id IN (SELECT id FROM qa_std_e2e_doomed);
    DELETE FROM work_item
     WHERE id IN (SELECT id FROM qa_std_e2e_doomed)
       AND type IN ('STANDARD_CHANGE_PROPOSAL', 'CHANGE_REQUEST');
    DELETE FROM standard_change_template WHERE id IN (SELECT id FROM qa_std_e2e_doomed);
    COMMIT;`);
}

/** Signs `email` in through the mock identity provider in a context of its own, and lands on `path`. */
async function signedIn(browser: Browser, email: string, path: string): Promise<Page> {
  const context = await browser.newContext({ baseURL: BASE_URL, viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  await page.goto(path);
  const field = page.getByLabel("Email");
  await field.waitFor({ timeout: 20_000 });
  await field.fill(email);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("button", { name: "Account" })).toBeVisible({ timeout: 30_000 });
  return page;
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** The editable surface of a rich-text field (`ExpandableRichTextField`), by its label. */
function richEditor(scope: Locator, label: string): Locator {
  return scope.getByRole("group", { name: label, exact: true }).locator(".editor-input");
}

/** Replaces a rich-text field's whole text with `text`. */
async function replaceRichText(page: Page, editor: Locator, text: string): Promise<void> {
  await editor.click();
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.press("Backspace");
  await page.keyboard.type(text);
}

/** Picks `option` in an MUI select (a combobox opening a listbox). */
async function pick(page: Page, combobox: Locator, option: string): Promise<void> {
  await combobox.click();
  await page.getByRole("option", { name: option, exact: true }).click();
  await expect(combobox).toContainText(option);
}

/** The sidebar (not the breadcrumb trail, which is a navigation too). */
function sidebar(page: Page): Locator {
  return page.locator("nav").filter({ has: page.getByRole("button", { name: "Dashboard", exact: true }) });
}

/** The sidebar's highlighted entry is "Change requests", and it is the only one. */
async function expectChangeRequestsActive(page: Page): Promise<void> {
  const selected = sidebar(page).locator(".Mui-selected");
  await expect(selected).toHaveText(["Change requests"]);
}

/**
 * The page heading of an Operations page reads exactly `heading` (its title, and the line under it). For a
 * standard change sub-view it also says nothing about the other Operations entities: checked once the exact text
 * has been seen, so the negative cannot pass on a page that has not rendered yet.
 */
async function expectHeading(main: Locator, heading: { title: string; subtitle: string }): Promise<void> {
  const title = main.getByRole("heading", { level: 5 }).first();
  await expect(title).toHaveText(heading.title);
  const subtitle = title.locator("xpath=following-sibling::*[1]");
  await expect(subtitle).toHaveText(heading.subtitle);
  if (heading.title !== OPERATIONS_HEADING.title) {
    await expect(title.locator("xpath=..")).not.toContainText(OTHER_OPERATIONS_ENTITIES);
    await expect(main.getByText(OPERATIONS_HEADING.subtitle, { exact: true })).toHaveCount(0);
    await expect(main.getByRole("heading", { name: OPERATIONS_HEADING.title, exact: true })).toHaveCount(0);
  }
}

/** The body of one rich-text section of `StandardChangeContentView` (a region named by its title). */
function renderedSection(scope: Locator, title: string): Locator {
  return scope.getByRole("region", { name: title, exact: true }).locator(":scope > div");
}

/** The value of one labelled field of `StandardChangeContentView` (Subject, Category, Impact, ...). */
function renderedField(scope: Locator, label: string): Locator {
  return scope.getByText(label, { exact: true }).locator("xpath=following-sibling::*[1]");
}

/** `StandardChangeContentView` shows the template's content with `host` / `env` filled in, every placeholder replaced. */
async function expectRenderedContent(scope: Locator, host: string, env: string): Promise<void> {
  await expect(renderedField(scope, "Subject")).toHaveText(TEMPLATE.subject(host, env));
  await expect(renderedField(scope, "Category")).toHaveText(TEMPLATE.category);
  await expect(renderedField(scope, "Impact")).toHaveText(TEMPLATE.impact);
  await expect(renderedField(scope, "Risk")).toHaveText(TEMPLATE.risk);
  await expect(renderedField(scope, "Rollback duration")).toHaveText(TEMPLATE.rollbackDuration);
  await expect(renderedField(scope, "Implementation plan visible to customers")).toHaveText("Yes");
  await expect(renderedField(scope, "Customer approval")).toHaveText("No");
  await expect(renderedField(scope, "Customer review")).toHaveText("No");
  await expect(renderedSection(scope, "Description")).toHaveText(TEMPLATE.description);
  await expect(renderedSection(scope, "Justification")).toHaveText(TEMPLATE.justification);
  await expect(renderedSection(scope, "Risk and impact analysis")).toHaveText(TEMPLATE.riskImpactAnalysis);
  await expect(renderedSection(scope, "Implementation plan")).toHaveText(TEMPLATE.implementation(host, env));
  await expect(renderedSection(scope, "Backout plan")).toHaveText(TEMPLATE.backout(host));
  await expect(renderedSection(scope, "Test plan")).toHaveText(TEMPLATE.test(env));
  // Every exact text above has been seen, so this cannot pass on a preview that has not rendered yet.
  await expect(scope).not.toContainText("{{");
}

/** How often each placeholder appears in what the template specifies (subject and plans, as bob writes them below). */
const PLACEHOLDER_COUNT = { host: 4, env: 3 } as const;

/** The marks `StandardChangeContentView` wraps a placeholder in: open (`{{key}}`, no value) or filled with a value. */
function openMarks(scope: Locator, key: string): Locator {
  return scope.locator(`mark[data-param="${key}"]:not([data-filled])`);
}
function filledMarks(scope: Locator, key: string): Locator {
  return scope.locator(`mark[data-param="${key}"][data-filled="true"]`);
}

/** What a mark's computed style says about how it is drawn. */
interface MarkStyle {
  backgroundColor: string;
  fontFamily: string;
  textDecorationLine: string;
  textDecorationStyle: string;
}
async function markStyle(mark: Locator): Promise<MarkStyle> {
  return mark.evaluate((el) => {
    const s = getComputedStyle(el);
    return {
      backgroundColor: s.backgroundColor,
      fontFamily: s.fontFamily,
      textDecorationLine: s.textDecorationLine,
      textDecorationStyle: s.textDecorationStyle,
    };
  });
}

/** Chromium's own `<mark>` background (yellow), which a mark the app did not style would still have. */
const USER_AGENT_MARK_BACKGROUND = "rgb(255, 255, 0)";

/**
 * A mark is drawn as the app styles it (`StandardChangeContentView`'s PLACEHOLDER_MARK_SX), not as a bare `<mark>`:
 * a background of its own that is not the browser's yellow; an open one in monospace with no underline, a filled one
 * in the text's own font with a dotted underline.
 */
async function expectHighlighted(mark: Locator, kind: "open" | "filled"): Promise<MarkStyle> {
  const style = await markStyle(mark);
  expect(style.backgroundColor, `a ${kind} placeholder mark has a background of its own`).not.toMatch(/^(transparent|rgba\(0, 0, 0, 0\))$/);
  expect(style.backgroundColor, `a ${kind} placeholder mark is styled by the app, not the browser's yellow`).not.toBe(USER_AGENT_MARK_BACKGROUND);
  if (kind === "open") {
    expect(style.fontFamily, "an open placeholder is in monospace").toMatch(/monospace/);
    expect(style.textDecorationLine, "an open placeholder is not underlined").toBe("none");
  } else {
    expect(style.fontFamily, "a filled placeholder is in the text's own font").not.toMatch(/monospace/);
    expect(style.textDecorationLine, "a filled placeholder is underlined").toBe("underline");
    expect(style.textDecorationStyle, "... dotted").toBe("dotted");
  }
  return style;
}

/**
 * With some placeholders filled and others not, on the same page in the same state, the open and the filled marks are
 * drawn differently (background, font and underline), each as the app styles it.
 */
async function expectOpenAndFilledDiffer(open: Locator, filled: Locator): Promise<void> {
  const openStyle = await expectHighlighted(open, "open");
  const filledStyle = await expectHighlighted(filled, "filled");
  expect(filledStyle.backgroundColor, "open and filled marks have different backgrounds").not.toBe(openStyle.backgroundColor);
  expect(filledStyle.fontFamily, "open and filled marks have different fonts").not.toBe(openStyle.fontFamily);
  expect(filledStyle.textDecorationLine, "only the filled mark is underlined").not.toBe(openStyle.textDecorationLine);
}

/** Every {{host}} / {{env}} in `scope` shows as an open, highlighted placeholder (none filled), in the subject and the plans. */
async function expectOpenPlaceholders(scope: Locator): Promise<void> {
  for (const key of ["host", "env"] as const) {
    const open = openMarks(scope, key);
    await expect(open, `open {{${key}}} marks`).toHaveCount(PLACEHOLDER_COUNT[key]);
    await expect(open).toHaveText(Array(PLACEHOLDER_COUNT[key]).fill(`{{${key}}}`));
    await expect(filledMarks(scope, key)).toHaveCount(0);
    await expectHighlighted(open.first(), "open");
  }
  await expect(openMarks(renderedField(scope, "Subject"), "host")).toHaveCount(1);
  await expect(openMarks(renderedField(scope, "Subject"), "env")).toHaveCount(1);
  await expect(openMarks(renderedSection(scope, "Implementation plan"), "host")).toHaveCount(2);
  await expect(openMarks(renderedSection(scope, "Test plan"), "env")).toHaveCount(1);
}

/** Every {{host}} / {{env}} in `scope` shows as a filled, highlighted mark with the value; none is left open. */
async function expectFilledPlaceholders(scope: Locator, values: { host: string; env: string }): Promise<void> {
  for (const key of ["host", "env"] as const) {
    const filled = filledMarks(scope, key);
    await expect(filled, `filled {{${key}}} marks`).toHaveCount(PLACEHOLDER_COUNT[key]);
    await expect(filled).toHaveText(Array(PLACEHOLDER_COUNT[key]).fill(values[key]));
    await expect(openMarks(scope, key)).toHaveCount(0);
    await expectHighlighted(filled.first(), "filled");
  }
}

/** The Plan tab's section titled `title` (the text under its heading). */
function planSection(main: Locator, title: string): Locator {
  return main.getByText(title, { exact: true }).locator("xpath=following-sibling::*[1]");
}

/** The page's breadcrumb trail reads `links`, then the record's number. */
async function expectBreadcrumb(main: Locator, links: string[], current: RegExp): Promise<void> {
  const trail = main.getByRole("navigation", { name: "Breadcrumb" });
  await expect(trail.getByRole("link")).toHaveText(links);
  await expect(trail.locator('[aria-current="page"]')).toHaveText(current);
}

/** "Pin this page", then the pinned tab in the top bar is named `label`. */
async function pinThisPage(page: Page, label: string | RegExp): Promise<void> {
  await page.getByRole("button", { name: "Pin this page to top nav bar" }).click();
  await expect(page.getByRole("button", { name: "Unpin this page" })).toBeVisible();
  const name = typeof label === "string" ? new RegExp(`^${escapeRegExp(label)} — pinned tab$`) : label;
  await expect(page.getByRole("button", { name })).toBeVisible();
}

/** The "needs your attention" strip above the catalogue. */
function attentionStrip(main: Locator): Locator {
  return main.getByRole("status", { name: "Needs your attention" });
}

/**
 * What Postgres holds for `email`, by the backend's own definitions (entity-service's `SearchProposals`): proposals in
 * CAB review on which they have a REQUESTED approver row, and their own drafts.
 */
async function pendingFor(email: string): Promise<{ awaiting: number; drafts: number }> {
  const out = await psql(`
    SELECT (SELECT count(*) FROM standard_change_proposal p
             WHERE p.state = 'CAB_REVIEW'
               AND EXISTS (SELECT 1 FROM approval_stage_approver a JOIN "user" u ON u.id = a.approver_user_id
                            WHERE a.work_item_id = p.id AND a.state = 'REQUESTED' AND lower(u.email) = lower('${email}'))),
           (SELECT count(*) FROM standard_change_proposal p JOIN work_item wi ON wi.id = p.id
             WHERE p.state = 'DRAFT' AND lower(wi.created_by) = lower('${email}'));`);
  const [awaiting, drafts] = out.split("|").map(Number);
  return { awaiting, drafts };
}

/** How many standard change proposals `email` has started, in Postgres. */
async function proposalsBy(email: string): Promise<number> {
  return Number(
    await psql(`SELECT count(*) FROM work_item WHERE type = 'STANDARD_CHANGE_PROPOSAL' AND lower(created_by) = lower('${email}');`),
  );
}

/** The strip's parts for these counts, as the page words them ("1 proposal awaits ...", "2 drafts of yours"). */
function attentionParts({ awaiting, drafts }: { awaiting: number; drafts: number }): string[] {
  const parts: string[] = [];
  if (awaiting > 0) parts.push(`${awaiting} ${awaiting === 1 ? "proposal awaits" : "proposals await"} your CAB decision`);
  if (drafts > 0) parts.push(`${drafts} ${drafts === 1 ? "draft" : "drafts"} of yours`);
  return parts;
}

/** Opens Standard changes and waits until the strip's two counts have been answered (so "no strip" means none, not "not yet"). */
async function openStandardChangesCounted(page: Page): Promise<void> {
  const counted = (key: string) =>
    page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        r.url().endsWith("/standard-change-proposals/search") &&
        (r.request().postData() ?? "").includes(key),
    );
  const [awaiting, drafts] = await Promise.all([
    counted('"awaitingMyDecision":true'),
    counted('"proposedByMe":true'),
    page.goto(STANDARD_CHANGES),
  ]);
  expect(awaiting.status()).toBe(200);
  expect(drafts.status()).toBe(200);
}

test.describe("standard changes (real stack)", () => {
  test.skip(
    !POSTGRES_CONTAINER || !realStackNamed(),
    "writes to the stack under test: set E2E_POSTGRES_CONTAINER (local compose: csmcr-postgres-1) and name the stack's CSM " +
      "backend, mock identity provider and entity-service (E2E_CSM_BFF_URL, E2E_OIDC_URL, E2E_ENTITY_SERVICE_URL)",
  );
  test.describe.configure({ mode: "serial" });

  test.beforeAll(async () => {
    await cleanUp();
  });
  test.afterAll(async () => {
    if (POSTGRES_CONTAINER && realStackNamed()) await cleanUp();
  });

  test("Standard changes is one tab of Operations > Change requests, headed as such, old links land there, and the seed is listed", async ({ browser }) => {
    test.setTimeout(240_000);

    const page = await signedIn(browser, USERS.bob, "/operations");
    try {
      const main = page.getByRole("main");
      const nav = sidebar(page);
      await expectHeading(main, OPERATIONS_HEADING);
      // The sidebar has rendered every Operations entry (each polled until it is visible, none skipped) ...
      for (const tab of OPERATIONS_TABS) {
        await expect(nav.getByRole("button", { name: tab, exact: true }), `the sidebar's "${tab}" entry`).toBeVisible();
      }
      // ... so this negative is about a rendered sidebar: there is no "Standard changes" entry of its own in Operations.
      await expect(nav.getByRole("button", { name: /^Standard changes?$/ })).toHaveCount(0);
      await expect(nav.getByText(/^Standard changes?$/)).toHaveCount(0);

      // Every other Operations tab, the change request list included, keeps the Operations heading.
      for (const tab of OPERATIONS_TABS) {
        const entry = nav.getByRole("button", { name: tab, exact: true });
        await entry.click();
        await expect(page, `the ${tab} tab`).toHaveURL(/\/operations\/[a-z-]+$/);
        await expectHeading(main, OPERATIONS_HEADING);
      }

      await nav.getByRole("button", { name: "Change requests", exact: true }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests"));
      await expectHeading(main, OPERATIONS_HEADING);
      const subNav = main.getByRole("tablist", { name: "Change requests" });
      await expect(subNav.getByRole("tab")).toHaveText(["Change requests", "Standard changes"]);
      await expect(subNav.getByRole("tab", { name: "Change requests" })).toHaveAttribute("aria-selected", "true");

      // The unfiltered list has change requests (so "none found" below is the filter's doing).
      const listSearch = main.getByPlaceholder("Search by number or subject…");
      const noneFound = main.getByText("No change requests found.", { exact: true });
      // A loaded row carries a change request number (CHG0039104, CS-PORTAL-000088, ...); a loading skeleton has no text.
      await expect(main.getByRole("row").filter({ hasText: /\b[A-Z][A-Z-]*\d{4,}\b/ }).first()).toBeVisible({ timeout: 20_000 });
      await expect(listSearch).toHaveValue("");
      await expect(noneFound).toHaveCount(0);

      // ---- Standard changes: the list's own filters ride along unread, and come back on the way out ----------------
      /** The filtered list is shown with its filter applied: the search box holds it and no change request matches. */
      const expectFilteredList = async (): Promise<void> => {
        await expect(page).toHaveURL(exactly("/operations/change-requests", CR_LIST_FILTERS));
        await expect(subNav.getByRole("tab", { name: "Change requests" })).toHaveAttribute("aria-selected", "true");
        await expectHeading(main, OPERATIONS_HEADING);
        await expect(listSearch).toHaveValue(CR_LIST_SEARCH);
        await expect(noneFound).toBeVisible();
      };
      await page.goto(`/operations/change-requests?${CR_LIST_FILTERS}`);
      await expectFilteredList();
      await subNav.getByRole("tab", { name: "Standard changes" }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", `view=standard&${CR_LIST_FILTERS}`));
      await expect(subNav.getByRole("tab", { name: "Standard changes" })).toHaveAttribute("aria-selected", "true");
      await expectHeading(main, STANDARD_CHANGES_HEADING);
      await expectChangeRequestsActive(page);
      // The list's filters do not apply here: its search box is not on the page.
      await expect(listSearch).toHaveCount(0);
      // The toolbar: the Templates | Proposals toggle on the catalogue, and Propose.
      const toggle = main.getByRole("group", { name: "Templates or proposals", exact: true });
      await expect(toggle.getByRole("button")).toHaveText(["Templates", "Proposals"]);
      await expect(toggle.getByRole("button", { name: "Templates" })).toHaveAttribute("aria-pressed", "true");
      await expect(main.getByRole("button", { name: "Propose a standard change" })).toBeVisible();
      // The strip is a push: Back returns to the filtered list it came from.
      await page.goBack();
      await expectFilteredList();
      await page.goForward();
      await expect(page).toHaveURL(exactly("/operations/change-requests", `view=standard&${CR_LIST_FILTERS}`));
      // The "Change requests" tab returns to the list as it was left, its filter applied, ...
      await subNav.getByRole("tab", { name: "Change requests" }).click();
      await expectFilteredList();
      // ... and back again, the filter still riding along.
      await subNav.getByRole("tab", { name: "Standard changes" }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", `view=standard&${CR_LIST_FILTERS}`));

      // ---- the catalogue: the seed's five templates with their states and versions -------------------------------
      const catalogue = main.getByRole("table", { name: "Standard change templates" });
      const templateSearch = main.getByRole("textbox", { name: "Search templates by number or name" });
      for (const seeded of SEEDED_TEMPLATES) {
        await templateSearch.fill(seeded.name);
        const row = catalogue.getByRole("row").filter({ hasText: seeded.name });
        await expect(row, `seeded template "${seeded.name}" (${SEED_HINT})`).toHaveCount(1);
        const cells = row.getByRole("cell");
        // Number, Name, Version, Risk, Impact, Used, State.
        await expect(cells.nth(0)).toHaveText(/^STD\d+$/);
        await expect(cells.nth(2), `${seeded.name}: version`).toHaveText(seeded.version);
        await expect(cells.nth(6), `${seeded.name}: state`).toHaveText(seeded.state);
      }
      await templateSearch.fill("");
      await expect(catalogue.getByRole("row").filter({ hasText: SUSPENDED_TEMPLATE })).toHaveCount(1);
      await shot(page, "01-standard-changes-catalogue", main);
      // Pinned with the list's two filters in the URL: named "Standard changes", with no filter count.
      await expect(page).toHaveURL(exactly("/operations/change-requests", `view=standard&${CR_LIST_FILTERS}`));
      await pinThisPage(page, "Standard changes");
      await expect(page.getByRole("button", { name: /^Standard changes: \d+ filters? — pinned tab$/ })).toHaveCount(0);

      // The suspended template's page says why; its trail and the sidebar place it under Change requests.
      await catalogue.getByRole("row").filter({ hasText: SUSPENDED_TEMPLATE }).click();
      await expect(page).toHaveURL(/\/operations\/standard-changes\/[^/?]+$/);
      await expect(main.getByRole("heading", { name: SUSPENDED_TEMPLATE })).toBeVisible();
      const suspended = main.getByRole("region", { name: "Suspended" });
      await expect(suspended).toContainText(`Suspended: ${SUSPENDED_REASON}`);
      await expect(suspended).toContainText(/\(by .+ on .+\)/);
      await expect(main.getByText("Version 1", { exact: true })).toBeVisible();
      await expectBreadcrumb(main, ["Change requests", "Standard changes"], /^STD\d+$/);
      await expectChangeRequestsActive(page);
      await shot(page, "02-suspended-template-with-reason", suspended);
      // Pinned by its number (the Recent entry the page records), not as "Operations".
      const suspendedNumber = (await main.getByRole("navigation", { name: "Breadcrumb" }).locator('[aria-current="page"]').textContent())!.trim();
      await pinThisPage(page, suspendedNumber);
      // Back returns to the catalogue it was opened from, as it was (the list's filters still riding along).
      await main.getByRole("button", { name: "Back", exact: true }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", `view=standard&${CR_LIST_FILTERS}`));

      // The revised template is at version 2; the trail's "Standard changes" leads back to the catalogue.
      await templateSearch.fill(REVISED_TEMPLATE);
      await catalogue.getByRole("row").filter({ hasText: REVISED_TEMPLATE }).click();
      await expect(main.getByRole("heading", { name: REVISED_TEMPLATE })).toBeVisible();
      await expect(main.getByText("Version 2", { exact: true })).toBeVisible();
      await expect(main.getByText("Active", { exact: true })).toBeVisible();
      await expectChangeRequestsActive(page);
      await main.getByRole("navigation", { name: "Breadcrumb" }).getByRole("link", { name: "Standard changes" }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard"));
      await expect(catalogue).toBeVisible();

      // ---- the proposals: the secondary view of the same tab, same heading, bob's seeded proposals in their states --
      await toggle.getByRole("button", { name: "Proposals" }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals"));
      await expect(toggle.getByRole("button", { name: "Proposals" })).toHaveAttribute("aria-pressed", "true");
      await expect(subNav.getByRole("tab", { name: "Standard changes" })).toHaveAttribute("aria-selected", "true");
      await expectHeading(main, STANDARD_CHANGES_HEADING);
      await expectChangeRequestsActive(page);
      // The strip is the catalogue's only.
      await expect(attentionStrip(main)).toHaveCount(0);
      const proposals = main.getByRole("table", { name: "Standard change proposals" });
      // The toggle and the scope select have names of their own: "Show" is the select's only.
      const scopeSelect = main.getByRole("combobox", { name: "Show", exact: true });
      await expect(scopeSelect).toHaveCount(1);
      await expect(main.getByRole("group", { name: "Templates or proposals", exact: true })).toHaveCount(1);
      await expect(main.getByRole("group", { name: "Show", exact: true })).toHaveCount(0);
      await expect(main.getByRole("combobox", { name: "Templates or proposals", exact: true })).toHaveCount(0);
      // The proposals open on all of them (no scope in the URL), and the select offers the three scopes in this order.
      await expect(scopeSelect).toHaveText("All proposals");
      await scopeSelect.click();
      await expect(page.getByRole("listbox").getByRole("option")).toHaveText(["All proposals", "Proposed by me", "Awaiting my decision"]);
      await page.keyboard.press("Escape");
      await expect(page.getByRole("listbox")).toHaveCount(0);
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals"));
      const proposalSearch = main.getByRole("textbox", { name: "Search proposals by number or name" });
      for (const seeded of SEEDED_PROPOSALS) {
        await proposalSearch.fill(seeded.name);
        const row = proposals.getByRole("row").filter({ hasText: seeded.name }).filter({ hasText: seeded.kind });
        await expect(row, `seeded proposal "${seeded.name}" (${seeded.kind}; ${SEED_HINT})`).toHaveCount(1);
        const cells = row.getByRole("cell");
        // Number, Name, Kind, State, Template, Proposed by, Updated.
        await expect(cells.nth(0)).toHaveText(/^SCP\d+$/);
        await expect(cells.nth(2)).toHaveText(seeded.kind);
        await expect(cells.nth(3), `${seeded.name}: state`).toHaveText(seeded.state);
        await expect(cells.nth(5)).toHaveText("Bob Fernando");
      }
      await proposalSearch.fill("");
      await expect(proposals.getByRole("row").filter({ hasText: SEEDED_PROPOSALS[0].name })).toHaveCount(1);
      await shot(page, "03-standard-change-proposals", main);

      // The proposals' filters are in the URL (replaced, not pushed), and the place pins with them counted.
      await pick(page, scopeSelect, "Proposed by me");
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals&scope=mine"));
      await expect(proposals.getByRole("row").filter({ hasText: SEEDED_PROPOSALS[0].name })).toHaveCount(1);
      await pinThisPage(page, "Standard change proposals: 1 filter");

      // A proposal page: the trail adds Proposals, the sidebar still highlights Change requests, Back returns to the filtered list.
      const rejected = SEEDED_PROPOSALS[0];
      const openRejected = async (): Promise<void> => {
        await proposals.getByRole("row").filter({ hasText: rejected.name }).click();
        await expect(page).toHaveURL(/\/operations\/standard-change-proposals\/[^/?]+$/);
        await expect(main.getByRole("heading", { name: rejected.name })).toBeVisible();
      };
      await openRejected();
      await expect(main.getByText("Rejected", { exact: true }).first()).toBeVisible();
      await expectBreadcrumb(main, ["Change requests", "Standard changes", "Proposals"], /^SCP\d+$/);
      await expectChangeRequestsActive(page);
      await main.getByRole("button", { name: "Back", exact: true }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals&scope=mine"));
      await expect(scopeSelect).toHaveText("Proposed by me");
      // The trail's "Proposals" opens all the proposals, whatever filter the record was opened from.
      await openRejected();
      await main.getByRole("navigation", { name: "Breadcrumb" }).getByRole("link", { name: "Proposals", exact: true }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals"));
      await expect(scopeSelect).toHaveText("All proposals");
      await expect(main.getByRole("combobox", { name: "State", exact: true })).toHaveText("Any state");
      await expect(proposals.getByRole("row").filter({ hasText: rejected.name })).toHaveCount(1);

      // Leaving for the list drops Standard changes' parameters.
      await subNav.getByRole("tab", { name: "Change requests" }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests"));
      await expectHeading(main, OPERATIONS_HEADING);

      // A pinned place opens where it was pinned.
      await page.getByRole("button", { name: /^Standard changes — pinned tab$/ }).click();
      await expect(page).toHaveURL(exactly("/operations/change-requests", "view=standard"));
      await expect(catalogue).toBeVisible();

      // ---- every old link lands on the right place --------------------------------------------------------------
      const stateSelect = main.getByRole("combobox", { name: "State", exact: true });
      const crList = `crQ=${CR_LIST_SEARCH}`;
      const legacy: Array<{ from: string; to: string; scope?: string; state?: string; listFilter?: string }> = [
        // The separate tab, in both spellings and both forms, with and without the old tab's own `view`.
        { from: "/operations?tab=standard_changes", to: "view=standard" },
        { from: "/operations?tab=standard_changes&view=catalogue", to: "view=standard" },
        { from: "/operations?tab=standard_changes&view=proposals", to: "view=standard&show=proposals", scope: "All proposals" },
        { from: "/operations?tab=standard-changes", to: "view=standard" },
        { from: "/operations?tab=standard-changes&view=catalogue", to: "view=standard" },
        { from: "/operations?tab=standard-changes&view=proposals", to: "view=standard&show=proposals", scope: "All proposals" },
        { from: "/operations/standard-changes", to: "view=standard" },
        { from: "/operations/standard-changes?view=catalogue", to: "view=standard" },
        { from: "/operations/standard-changes?view=proposals", to: "view=standard&show=proposals", scope: "All proposals" },
        // The two sub-views Standard changes replaced, with the proposals' filters ...
        { from: "/operations/change-requests?view=catalogue", to: "view=standard" },
        { from: "/operations/change-requests?view=proposals", to: "view=standard&show=proposals", scope: "All proposals", state: "Any state" },
        { from: "/operations/change-requests?view=proposals&scope=awaiting", to: "view=standard&show=proposals&scope=awaiting", scope: "Awaiting my decision" },
        { from: "/operations/change-requests?view=proposals&state=draft", to: "view=standard&show=proposals&state=draft", scope: "All proposals", state: "Draft" },
        {
          from: "/operations/change-requests?view=proposals&scope=mine&state=rejected",
          to: "view=standard&show=proposals&scope=mine&state=rejected",
          scope: "Proposed by me",
          state: "Rejected",
        },
        // ... and with the list's own `cr*` filters, kept for the way back to the list.
        { from: `/operations/change-requests?view=catalogue&${crList}`, to: `view=standard&${crList}`, listFilter: crList },
        {
          from: `/operations/change-requests?${crList}&view=proposals&scope=awaiting`,
          to: `view=standard&show=proposals&scope=awaiting&${crList}`,
          scope: "Awaiting my decision",
          listFilter: crList,
        },
      ];
      for (const { from, to, scope, state, listFilter } of legacy) {
        // Each is opened from the change request list, so Back shows whether the old URL stayed in the history.
        await page.goto("/operations/change-requests");
        await expect(subNav.getByRole("tab", { name: "Change requests" })).toHaveAttribute("aria-selected", "true");
        await page.goto(from);
        await expect(page, `${from} lands on ?${to}`).toHaveURL(exactly("/operations/change-requests", to));
        await expect(subNav.getByRole("tab", { name: "Standard changes" })).toHaveAttribute("aria-selected", "true");
        await expectHeading(main, STANDARD_CHANGES_HEADING);
        const toProposals = to.includes("show=proposals");
        await expect(toggle.getByRole("button", { name: toProposals ? "Proposals" : "Templates" })).toHaveAttribute("aria-pressed", "true");
        await expect(toProposals ? proposals : catalogue).toBeVisible();
        if (scope) await expect(scopeSelect, `${from}: scope`).toHaveText(scope);
        if (state) await expect(stateSelect, `${from}: state`).toHaveText(state);
        await expectChangeRequestsActive(page);
        // The rewrite replaced the old URL: one Back is the list it was opened from.
        await page.goBack();
        await expect(page, `${from}: no history entry of its own`).toHaveURL(exactly("/operations/change-requests"));
        if (listFilter) {
          // The list's filter it carried comes back with the "Change requests" tab.
          await page.goForward();
          await expect(page).toHaveURL(exactly("/operations/change-requests", to));
          await subNav.getByRole("tab", { name: "Change requests" }).click();
          await expect(page, `${from}: back to the filtered list`).toHaveURL(exactly("/operations/change-requests", listFilter));
          await expect(listSearch).toHaveValue(CR_LIST_SEARCH);
          await expect(noneFound).toBeVisible();
        }
      }
    } finally {
      await page.context().close();
    }
  });

  test("the attention strip above the catalogue: nothing for jane, carol's CAB decision and bob's draft, each linking to them", async ({ browser }) => {
    test.setTimeout(180_000);

    // jane: no proposal awaits her and she has no draft, so there is no strip at all.
    expect(await pendingFor(USERS.jane), "jane has nothing pending in Postgres").toEqual({ awaiting: 0, drafts: 0 });
    const jane = await signedIn(browser, USERS.jane, "/operations");
    try {
      const main = jane.getByRole("main");
      await openStandardChangesCounted(jane);
      await expect(main.getByRole("table", { name: "Standard change templates" }).getByRole("row").filter({ hasText: SUSPENDED_TEMPLATE })).toHaveCount(1);
      await expect(attentionStrip(main)).toHaveCount(0);
      await expect(main.getByText(/your CAB decision|of yours/)).toHaveCount(0);
      await shot(jane, "04-no-attention-strip", main);

      // The proposals open on all of them: jane proposed nothing, yet she sees bob's seeded proposals.
      expect(await proposalsBy(USERS.jane), "jane has proposed nothing (Postgres)").toBe(0);
      await main.getByRole("group", { name: "Templates or proposals", exact: true }).getByRole("button", { name: "Proposals" }).click();
      await expect(jane).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals"));
      await expect(main.getByRole("combobox", { name: "Show", exact: true })).toHaveText("All proposals");
      const proposals = main.getByRole("table", { name: "Standard change proposals" });
      const search = main.getByRole("textbox", { name: "Search proposals by number or name" });
      for (const seeded of SEEDED_PROPOSALS) {
        await search.fill(seeded.name);
        const row = proposals.getByRole("row").filter({ hasText: seeded.name }).filter({ hasText: seeded.kind });
        await expect(row, `jane sees bob's "${seeded.name}" (${seeded.kind})`).toHaveCount(1);
        await expect(row.getByRole("cell").nth(5)).toHaveText("Bob Fernando");
      }
      await search.fill("");
      await shot(jane, "04b-all-proposals-for-jane", main);
      // Her own proposals are none, which is what "Proposed by me" lists.
      await pick(jane, main.getByRole("combobox", { name: "Show", exact: true }), "Proposed by me");
      await expect(jane).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals&scope=mine"));
      await expect(proposals.getByText("No proposals found.", { exact: true })).toBeVisible();
      await expect(proposals.getByRole("row").filter({ hasText: SEEDED_PROPOSALS[0].name })).toHaveCount(0);
    } finally {
      await jane.context().close();
    }

    // carol (CAB) is asked about the seed's proposal in CAB review; bob has the seed's draft. Each link opens the
    // proposals filtered to it, which lists that proposal.
    const inCabReview = SEEDED_PROPOSALS.find((p) => p.state === "CAB review")!;
    const draft = SEEDED_PROPOSALS.find((p) => p.state === "Draft")!;
    const cases = [
      { who: USERS.carol, part: "awaiting", search: "view=standard&show=proposals&scope=awaiting", scope: "Awaiting my decision", state: "Any state", lists: inCabReview },
      { who: USERS.bob, part: "drafts", search: "view=standard&show=proposals&scope=mine&state=draft", scope: "Proposed by me", state: "Draft", lists: draft },
    ] as const;
    for (const c of cases) {
      const pending = await pendingFor(c.who);
      expect(pending[c.part], `${c.who} has a ${c.part === "awaiting" ? "proposal awaiting a decision" : "draft"} (${SEED_HINT})`).toBeGreaterThan(0);
      const parts = attentionParts(pending);
      const page = await signedIn(browser, c.who, "/operations");
      try {
        const main = page.getByRole("main");
        await openStandardChangesCounted(page);
        const strip = attentionStrip(main);
        await expect(strip, `${c.who}'s strip`).toHaveText(parts.join(" · "));
        await expect(strip.getByRole("link")).toHaveText(parts);
        await shot(page, `05-attention-strip-${c.part}`, strip);
        const link = strip.getByRole("link", { name: c.part === "awaiting" ? /your CAB decision$/ : /of yours$/ });
        await link.click();
        await expect(page).toHaveURL(exactly("/operations/change-requests", c.search));
        await expect(main.getByRole("group", { name: "Templates or proposals", exact: true }).getByRole("button", { name: "Proposals" })).toHaveAttribute("aria-pressed", "true");
        await expect(main.getByRole("combobox", { name: "Show", exact: true })).toHaveText(c.scope);
        await expect(main.getByRole("combobox", { name: "State" })).toHaveText(c.state);
        await expect(attentionStrip(main)).toHaveCount(0);
        const proposals = main.getByRole("table", { name: "Standard change proposals" });
        const row = proposals.getByRole("row").filter({ hasText: c.lists.name });
        await expect(row).toHaveCount(1);
        await expect(row.getByRole("cell").nth(3)).toHaveText(c.lists.state);
        // Every row the link lists is what the count counted.
        await expect(proposals.getByRole("row")).toHaveCount(1 + pending[c.part]);
      } finally {
        await page.context().close();
      }
    }
  });

  test("propose from a closed change request, the CAB approves, raise a change request from the template, roll it back: the template needs review", async ({ browser }) => {
    test.setTimeout(360_000);

    // The evidence: a closed change request, raised and walked through the real API.
    const created = await staff("jane").create({
      subject: SOURCE_SUBJECT,
      type: "normal",
      groupId: EXAMPLE_CORP_ABT_GROUP_ID,
      customerApprovalRequired: false,
      customerReviewRequired: false,
      implementationPlan: "<p>Drain cache-01, restart the cache service, put it back in the pool.</p>",
      backoutPlan: `<p>${SOURCE_BACKOUT}</p>`,
      testPlan: `<p>${SOURCE_TEST}</p>`,
    });
    const source = created.body.changeRequest;
    expect(created.status, JSON.stringify(created.body)).toBe(201);
    await walkTo(source!, "closed");

    // ---- bob proposes it as a standard change and templates it --------------------------------------------------
    const bob = await signedIn(browser, USERS.bob, `/operations/change-requests/${source!.id}`);
    let proposalUrl = "";
    try {
      const detail = new ChangeRequestDetailPage(bob);
      await expect(detail.lifecycleStepper()).toBeVisible({ timeout: 20_000 });
      await expect(detail.currentStep()).toContainText("Closed");
      await bob.getByRole("button", { name: "Propose as standard change" }).click();
      await expect(bob).toHaveURL(/\/operations\/standard-change-proposals\/[^/]+$/, { timeout: 20_000 });
      proposalUrl = new URL(bob.url()).pathname;

      const main = bob.getByRole("main");
      await expectBreadcrumb(main, ["Change requests", "Standard changes", "Proposals"], /^SCP\d+$/);
      await expectChangeRequestsActive(bob);
      await expect(main.getByText(`Started from:`)).toContainText(source!.number);
      const evidence = main.getByRole("table", { name: "Evidence" });
      await expect(evidence).toContainText(source!.number);
      // The change request's content was copied in.
      const specifies = main.getByRole("region", { name: "What the template specifies" });
      await expect(richEditor(specifies, "Backout plan")).toContainText(SOURCE_BACKOUT);
      await expect(richEditor(specifies, "Test plan")).toContainText(SOURCE_TEST);

      const proposalSection = main.getByRole("region", { name: "Proposal" });
      await proposalSection.getByRole("textbox", { name: /^Name/ }).fill(TEMPLATE_NAME);
      await replaceRichText(bob, richEditor(proposalSection, "Rationale"), "Done many times without incident; the steps never change, only the node.");

      // Two parameters: the node (text) and its environment (a choice).
      const params = main.getByRole("region", { name: "Parameters (change specifics)" });
      await params.getByRole("button", { name: "Add parameter" }).click();
      await params.getByLabel("Key of parameter 1").fill("host");
      await params.getByLabel("Label of parameter 1").fill("Host name");
      await expect(params.getByLabel("Type of parameter 1")).toHaveValue("text");
      await params.getByLabel("Help text of parameter 1").fill("The cache node to restart.");
      await params.getByRole("button", { name: "Add parameter" }).click();
      await params.getByLabel("Key of parameter 2").fill("env");
      await params.getByLabel("Label of parameter 2").fill("Environment");
      await params.getByLabel("Type of parameter 2").selectOption("select");
      await params.getByLabel("Options of parameter 2").fill("Production\nStaging");

      // What the template specifies: every field a change request raised from it takes, with placeholders in the
      // subject and the three plans (typed, and through "Insert {{…}}").
      await specifies.getByRole("textbox", { name: /^Subject/ }).fill(TEMPLATE_SUBJECT);
      await pick(bob, specifies.getByRole("combobox", { name: "Category" }), TEMPLATE.category);
      await pick(bob, specifies.getByRole("combobox", { name: "Impact" }), TEMPLATE.impact);
      await pick(bob, specifies.getByRole("combobox", { name: "Risk" }), TEMPLATE.risk);
      await specifies.getByRole("textbox", { name: "Rollback duration" }).fill(TEMPLATE.rollbackDuration);
      const planVisible = specifies.getByRole("checkbox", { name: "Implementation plan visible to customers" });
      await planVisible.setChecked(TEMPLATE.planVisibleToCustomers);
      await expect(specifies.getByRole("checkbox", { name: "Customer approval" })).not.toBeChecked();
      await expect(specifies.getByRole("checkbox", { name: "Customer review" })).not.toBeChecked();
      await replaceRichText(bob, richEditor(specifies, "Description"), TEMPLATE.description);
      await replaceRichText(bob, richEditor(specifies, "Justification"), TEMPLATE.justification);
      await replaceRichText(bob, richEditor(specifies, "Risk and impact analysis"), TEMPLATE.riskImpactAnalysis);
      await replaceRichText(bob, richEditor(specifies, "Implementation plan"), TEMPLATE.implementationTyped);
      await specifies.getByRole("button", { name: "Insert a placeholder into Implementation plan" }).click();
      await bob.getByRole("menuitem", { name: /\{\{host\}\}/ }).click();
      await expect(richEditor(specifies, "Implementation plan")).toHaveText(`${TEMPLATE.implementationTyped}{{host}}`);
      await replaceRichText(bob, richEditor(specifies, "Backout plan"), TEMPLATE.backoutTyped);
      await expect(richEditor(specifies, "Backout plan")).toHaveText(TEMPLATE.backoutTyped);
      await replaceRichText(bob, richEditor(specifies, "Test plan"), TEMPLATE.testTyped);
      await specifies.getByRole("button", { name: "Insert a placeholder into Test plan" }).click();
      await bob.getByRole("menuitem", { name: /\{\{env\}\}/ }).click();
      await expect(richEditor(specifies, "Test plan")).toHaveText(`${TEMPLATE.testTyped}{{env}}`);
      await expect(main.getByText(/The text refers to placeholders no parameter declares/)).toHaveCount(0);
      await expect(main.getByText(/Declared but not used in the text/)).toHaveCount(0);

      // The live preview: before any sample value every placeholder is shown open and highlighted, ...
      const preview = main.getByRole("region", { name: "Preview", exact: true });
      const rendered = preview.getByRole("region", { name: "Preview of a change request raised from this template" });
      await expect(renderedSection(rendered, "Description")).toHaveText(TEMPLATE.description);
      await expectOpenPlaceholders(rendered);
      await expect(renderedField(rendered, "Subject")).toHaveText(TEMPLATE_SUBJECT);
      await shot(bob, "06-proposal-preview-open-placeholders", preview);
      // ... with only the host typed, {{host}} filled and {{env}} still open side by side on the same preview, drawn
      // differently (the filled one with the dotted underline), ...
      await preview.getByRole("textbox", { name: /^Host name/ }).fill("cache-07");
      await expect(filledMarks(rendered, "host")).toHaveCount(PLACEHOLDER_COUNT.host);
      await expect(filledMarks(rendered, "host")).toHaveText(Array(PLACEHOLDER_COUNT.host).fill("cache-07"));
      await expect(openMarks(rendered, "env")).toHaveCount(PLACEHOLDER_COUNT.env);
      await expect(openMarks(rendered, "host")).toHaveCount(0);
      await expectOpenAndFilledDiffer(openMarks(rendered, "env").first(), filledMarks(rendered, "host").first());
      // The same holds inside one field: the subject carries both.
      const subjectField = renderedField(rendered, "Subject");
      await expectOpenAndFilledDiffer(openMarks(subjectField, "env"), filledMarks(subjectField, "host"));
      await shot(bob, "06b-proposal-preview-open-and-filled", preview);
      // ... and with every value, filled (the inserted ones included), still marked.
      await preview.getByRole("combobox", { name: /^Environment/ }).selectOption("Staging");
      await expectRenderedContent(rendered, "cache-07", "Staging");
      await expectFilledPlaceholders(rendered, { host: "cache-07", env: "Staging" });
      await shot(bob, "07-proposal-preview-with-sample-values", preview);

      await expect(main.getByText("Ready to send to the CAB. Submitting saves first.")).toBeVisible();
      const [saved] = await Promise.all([
        bob.waitForResponse((r) => r.request().method() === "PATCH" && /\/standard-change-proposals\/[^/]+$/.test(r.url())),
        main.getByRole("button", { name: "Save", exact: true }).click(),
      ]);
      expect(saved.status(), await saved.text()).toBe(200);

      await main.getByRole("button", { name: "Submit to CAB" }).click();
      await expect(main.getByText("CAB review", { exact: true })).toBeVisible({ timeout: 15_000 });
      await expect(main.getByRole("heading", { name: TEMPLATE_NAME })).toBeVisible();
      const approvers = main.getByRole("table", { name: "CAB approvers" });
      await expect(approvers.getByRole("row").filter({ hasText: "Carol Silva" })).toContainText("Requested");
      // The proposer is not one of the deciders, and has no Approve of his own.
      await expect(main.getByRole("region", { name: "CAB approval" }).getByRole("button", { name: "Approve" })).toHaveCount(0);
      await shot(bob, "08-proposal-in-cab-review", main.getByRole("region", { name: "CAB approval" }));
    } finally {
      await bob.context().close();
    }

    // ---- carol finds it through her attention strip and approves it for the CAB ---------------------------------
    const carol = await signedIn(browser, USERS.carol, "/operations");
    let templatePath = "";
    let templateId = "";
    let templateNumber = "";
    try {
      const main = carol.getByRole("main");
      const pending = await pendingFor(USERS.carol);
      expect(pending.awaiting, "bob's proposal now awaits carol").toBeGreaterThan(0);
      await openStandardChangesCounted(carol);
      const strip = attentionStrip(main);
      await expect(strip).toHaveText(attentionParts(pending).join(" · "));
      await strip.getByRole("link", { name: /your CAB decision$/ }).click();
      await expect(carol).toHaveURL(exactly("/operations/change-requests", "view=standard&show=proposals&scope=awaiting"));
      await main.getByRole("textbox", { name: "Search proposals by number or name" }).fill(TEMPLATE_NAME);
      const mine = main.getByRole("table", { name: "Standard change proposals" }).getByRole("row").filter({ hasText: TEMPLATE_NAME });
      await expect(mine).toHaveCount(1);
      await expect(mine.getByRole("cell").nth(3)).toHaveText("CAB review");
      await mine.click();
      await expect(carol).toHaveURL(new RegExp(`${escapeRegExp(proposalUrl)}$`));

      const cab = main.getByRole("region", { name: "CAB approval" });
      await expect(cab.getByRole("button", { name: "Approve" })).toBeVisible({ timeout: 20_000 });
      await cab.getByRole("button", { name: "Approve" }).click();
      const dialog = carol.getByRole("dialog", { name: "Approve the proposal" });
      await dialog.getByRole("textbox", { name: /^Comment/ }).fill("Low risk and well rehearsed.");
      await dialog.getByRole("button", { name: "Approve", exact: true }).click();
      await expect(dialog).toBeHidden();
      const decision = main.getByRole("region", { name: "Decision" });
      await expect(decision).toContainText("Approved by the CAB");
      // The template is named by its catalogue number and name.
      await expect(decision).toContainText(new RegExp(`Resulting template: STD\\d+ ${escapeRegExp(TEMPLATE_NAME)}, version 1`));

      await decision.getByRole("button", { name: TEMPLATE_NAME }).click();
      await expect(carol).toHaveURL(/\/operations\/standard-changes\/[^/]+$/);
      templatePath = new URL(carol.url()).pathname;
      templateId = decodeURIComponent(templatePath.split("/").pop()!);
      await expect(main.getByRole("heading", { name: TEMPLATE_NAME })).toBeVisible();
      await expect(main.getByText("Active", { exact: true })).toBeVisible();
      await expect(main.getByText("Version 1", { exact: true })).toBeVisible();
      await expect(main.getByText("Review needed", { exact: true })).toHaveCount(0);
      await expectChangeRequestsActive(carol);
      templateNumber = (await main.getByText(/^STD\d+$/).first().textContent())!.trim();
      await expectBreadcrumb(main, ["Change requests", "Standard changes"], new RegExp(`^${templateNumber}$`));
      // The current version, with every placeholder highlighted (open: a template has no values).
      const currentVersion = main
        .locator(".MuiCard-root")
        .filter({ has: carol.getByText("What the template specifies (version 1)", { exact: true }) });
      await expect(currentVersion).toHaveCount(1);
      await expect(renderedField(currentVersion, "Impact")).toHaveText(TEMPLATE.impact);
      await expect(renderedSection(currentVersion, "Description")).toHaveText(TEMPLATE.description);
      await expectOpenPlaceholders(currentVersion);
      await shot(carol, "09-template-active-v1-placeholders", currentVersion);
      // There is no proposal left for her to decide.
      expect((await pendingFor(USERS.carol)).awaiting).toBe(pending.awaiting - 1);
    } finally {
      await carol.context().close();
    }

    // What the template's version holds, as the API returns it: what the change request raised from it must equal.
    const templateRead = await staff("alice").request<{
      currentVersion: {
        description: string;
        justification: string;
        riskImpactAnalysis: string;
        isPlanningVisibleToCustomers: boolean;
        impact: string;
        risk: string;
        category: string;
      };
    }>("GET", `/standard-change-templates/${templateId}`);
    expect(templateRead.status, JSON.stringify(templateRead.body)).toBe(200);
    const version = templateRead.body.currentVersion;
    expect(version.impact).toBe("medium");
    expect(version.isPlanningVisibleToCustomers).toBe(TEMPLATE.planVisibleToCustomers);
    expect(version.description).toContain(TEMPLATE.description);
    expect(version.justification).toContain(TEMPLATE.justification);
    expect(version.riskImpactAnalysis).toContain(TEMPLATE.riskImpactAnalysis);

    // ---- alice finds it in the catalogue, raises a change request from it and walks it to a rollback ------------
    const alice = await signedIn(browser, USERS.alice, STANDARD_CHANGES);
    try {
      const main = alice.getByRole("main");
      await main.getByRole("textbox", { name: "Search templates by number or name" }).fill(TEMPLATE_NAME);
      const row = main.getByRole("table", { name: "Standard change templates" }).getByRole("row").filter({ hasText: TEMPLATE_NAME });
      await expect(row).toHaveCount(1);
      await expect(row).toContainText(templateNumber);
      await expect(row).toContainText("v1");
      await expect(row).toContainText("Active");
      await expect(row.getByRole("cell").nth(4)).toHaveText(TEMPLATE.impact);

      // The create form: type Standard, the template, its change specifics, the read-only preview.
      await alice.goto("/operations/change-requests/new");
      await alice.getByRole("group", { name: /what type of change is required/i }).getByRole("radio", { name: /^Standard/ }).check();
      const templatePicker = alice.getByRole("combobox", { name: /^Standard change template/ });
      await templatePicker.fill(RUN);
      await alice.getByRole("option", { name: `${templateNumber} — ${TEMPLATE_NAME}` }).click();
      const specifics = alice.getByRole("group", { name: "Change specifics" });
      await specifics.getByRole("textbox", { name: /^Host name/ }).fill("cache-42");
      await specifics.getByRole("combobox", { name: /^Environment/ }).selectOption("Production");
      const preview = alice.getByRole("region", { name: "What the template specifies (version 1)" });
      await expectRenderedContent(preview, "cache-42", "Production");
      await expectFilledPlaceholders(preview, { host: "cache-42", env: "Production" });
      // Read-only: the template's text is shown, not offered as fields.
      await expect(preview.getByRole("textbox")).toHaveCount(0);
      await expect(alice.getByRole("textbox", { name: /^Subject/ })).toHaveCount(0);
      await shot(alice, "10-create-from-template", preview);
      const group = alice.getByRole("combobox", { name: /^Assignment group/ });
      await group.fill("Example Corp ABT");
      await alice.getByRole("option", { name: "Example Corp ABT" }).click();
      const [createdResponse] = await Promise.all([
        alice.waitForResponse((r) => r.request().method() === "POST" && /\/change-requests$/.test(r.url())),
        alice.getByRole("button", { name: "Create change request" }).click(),
      ]);
      expect(createdResponse.status(), await createdResponse.text()).toBe(201);
      // The form sent none of the fields the template specifies: the impact below can only be the template's.
      const sent = createdResponse.request().postDataJSON() as Record<string, unknown>;
      for (const field of ["impact", "category", "description", "justification", "impactDescription", "riskImpactAnalysis", "implementationPlan", "isPlanningVisibleToCustomers"]) {
        expect(sent, `the create request does not send ${field}`).not.toHaveProperty(field);
      }
      await expect(alice).toHaveURL(/\/operations\/change-requests\/(?!new(?:[/?#]|$))[^/]+$/, { timeout: 15_000 });
      const raisedId = new URL(alice.url()).pathname.split("/").pop()!;

      const detail = new ChangeRequestDetailPage(alice);
      await expect(detail.lifecycleStepper()).toBeVisible({ timeout: 20_000 });
      const subject = TEMPLATE.subject("cache-42", "Production");
      await expect(main.getByRole("heading", { name: subject })).toBeVisible();
      const banner = main.getByRole("region", { name: "Standard change template" });
      await expect(banner).toContainText("Raised from standard change template");
      await expect(banner).toContainText(`${templateNumber} ${TEMPLATE_NAME}`);
      await expect(banner).toContainText("(version 1)");
      await expect(banner.locator("dt")).toHaveText(["Host name", "Environment"]);
      await expect(banner.locator("dd")).toHaveText(["cache-42", "Production"]);
      await shot(alice, "11-raised-change-request-banner", banner);

      // A standard change is pre-authorised: the lifecycle bar never takes it through Assess or Authorize (no customer
      // steps either: the template asks for neither).
      await expect(detail.stepLabels()).toHaveText([
        "New, current",
        "Assess, not taken",
        "Authorize, not taken",
        "Scheduled, upcoming",
        "Implement, upcoming",
        "Review, upcoming",
        "Rollback, not taken",
        "Closed, upcoming",
        "Canceled, not taken",
      ]);
      await shot(alice, "12-lifecycle-standard-change", detail.lifecycleStepper());

      // What the template specifies, rendered with alice's values, is what the change request holds.
      await expect(detail.overviewCell("Impact")).toContainText(TEMPLATE.impact);
      await expect(detail.overviewCell("Category")).toContainText(TEMPLATE.category);
      await detail.planTab().click();
      await expect(planSection(main, "Implementation plan")).toHaveText(TEMPLATE.implementation("cache-42", "Production"));
      await expect(planSection(main, "Rollback plan")).toHaveText(TEMPLATE.backout("cache-42"));
      await expect(planSection(main, "Test plan")).toHaveText(TEMPLATE.test("Production"));
      await expect(planSection(main, "Rollback duration")).toHaveText(TEMPLATE.rollbackDuration);
      await expect(main).not.toContainText("{{");
      await shot(
        alice,
        "13-raised-change-request-plan",
        planSection(main, "Implementation plan").locator("xpath=ancestor::*[contains(@class, 'MuiCard-root')][1]"),
      );
      // The portal shows no risk on a change request, nor some of the template's other fields: read them, and the
      // rest once more, as the API returns them, against the template's version.
      const stored = await staff("alice").get(raisedId);
      expect(stored.status).toBe(200);
      const body = stored.body as unknown as Record<string, unknown>;
      expect(body.subject ?? body.title).toBe(subject);
      expect(String(body.impact).toLowerCase()).toBe(version.impact);
      expect(String(typeof body.category === "object" && body.category ? (body.category as { id?: string }).id : body.category).toLowerCase()).toBe(version.category);
      expect(body.rollbackDurationText).toBe(TEMPLATE.rollbackDuration);
      expect(body.description, "description").toBe(version.description);
      expect(body.justification, "justification").toBe(version.justification);
      expect(body.impactDescription, "risk and impact analysis").toBe(version.riskImpactAnalysis);
      expect(body.isPlanningVisibleToCustomers, "plan visible to customers").toBe(version.isPlanningVisibleToCustomers);
      expect(body.customerApprovalRequired).toBe(false);
      expect(body.customerReviewRequired).toBe(false);
      expect(await psql(`SELECT risk || '|' || impact || '|' || is_planning_visible_to_customers FROM change_request WHERE id = '${raisedId}';`)).toBe("LOW|MEDIUM|true");

      // Edit: every field the template specifies is disabled and shows the template's value, the type stays Standard;
      // the others stay editable.
      await detail.editButton().click();
      const edit = detail.editDialog();
      await expect(edit).toBeVisible();
      await expect(edit).toContainText(`Raised from standard change template ${templateNumber} (version 1)`);
      await expect(edit.getByRole("textbox", { name: /^Subject/ })).toBeDisabled();
      await expect(edit.getByRole("textbox", { name: /^Subject/ })).toHaveValue(subject);
      await expect(edit.getByRole("combobox", { name: /^Type/ })).toBeDisabled();
      await expect(edit.getByRole("combobox", { name: /^Type/ })).toContainText("Standard");
      await expect(edit.getByRole("combobox", { name: /^Impact/ })).toBeDisabled();
      await expect(edit.getByRole("combobox", { name: /^Impact/ })).toHaveText(TEMPLATE.impact);
      await expect(edit.getByRole("combobox", { name: /^Category/ })).toBeDisabled();
      await expect(edit.getByRole("combobox", { name: /^Category/ })).toHaveText(TEMPLATE.category);
      await expect(edit.getByRole("textbox", { name: /^Rollback duration/ })).toBeDisabled();
      await expect(edit.getByRole("textbox", { name: /^Rollback duration/ })).toHaveValue(TEMPLATE.rollbackDuration);
      for (const box of ["Customer Approval", "Customer Review"]) {
        await expect(edit.getByRole("checkbox", { name: box }), box).toBeDisabled();
        await expect(edit.getByRole("checkbox", { name: box }), `${box}: the template's (no)`).not.toBeChecked();
      }
      const planSwitch = edit.getByRole("switch", { name: "Implementation Plan visible to customers" });
      await expect(planSwitch).toBeDisabled();
      await expect(planSwitch, "plan visibility: the template's (yes)").toBeChecked();
      const templateText: Array<[string, string]> = [
        ["Description", TEMPLATE.description],
        ["Justification", TEMPLATE.justification],
        ["Risk and impact analysis", TEMPLATE.riskImpactAnalysis],
        ["Implementation plan", TEMPLATE.implementation("cache-42", "Production")],
        ["Rollback plan", TEMPLATE.backout("cache-42")],
        ["Test plan", TEMPLATE.test("Production")],
      ];
      for (const [label, text] of templateText) {
        await expect(richEditor(edit, label), `${label} is the template's`).toHaveAttribute("contenteditable", "false");
        await expect(richEditor(edit, label), `${label} shows the template's text`).toHaveText(text);
      }
      await expect(edit.getByText("Set by the standard change template.").first()).toBeVisible();
      // What the template does not specify is still the engineer's to set.
      await expect(edit.getByRole("combobox", { name: /^Priority/ })).toBeEnabled();
      await expect(edit.getByRole("combobox", { name: /^Assignment group/ })).toBeEnabled();
      await expect(edit.getByRole("combobox", { name: /^Assigned to/ })).toBeEnabled();
      await expect(richEditor(edit, "Affected services")).toHaveAttribute("contenteditable", "true");
      await shot(alice, "14-edit-template-fields-disabled", edit);
      await edit.getByRole("button", { name: "Cancel" }).click();
      await expect(edit).toBeHidden();

      // The backend holds the same line, whatever the page offers: through the BFF, a PATCH that changes a field the
      // template specifies is refused with its own error code and changes nothing ...
      const before = await staff("alice").get(raisedId);
      const storedPlan = (before.body as unknown as Record<string, unknown>).implementationPlan as string;
      expect(storedPlan).toContain("cache-42");
      for (const change of [
        { implementationPlan: "<p>Something other than the template's plan.</p>" },
        { title: `${TAG} Another subject (${RUN})` },
        { impact: "low" },
      ]) {
        const refused = await staff("alice").patch(raisedId, change);
        const refusal = refused.body as unknown as { errorCode?: string; message?: string };
        expect(refused.status, `${JSON.stringify(change)}: ${JSON.stringify(refused.body)}`).toBe(400);
        expect(refusal.errorCode).toBe("change_request_standard_template_field");
        expect(refusal.message).toMatch(/raised from a standard change template/);
      }
      const unchanged = (await staff("alice").get(raisedId)).body as unknown as Record<string, unknown>;
      expect(unchanged.implementationPlan).toBe(storedPlan);
      expect(unchanged.subject ?? unchanged.title).toBe(subject);
      expect(String(unchanged.impact).toLowerCase()).toBe("medium");
      // ... while the stored value resent, with a work note, is accepted (a no-op for the plan).
      const workNote = `${TAG} work note beside the plan the template set (${RUN})`;
      const resent = await staff("alice").patch(raisedId, { implementationPlan: storedPlan, workNote });
      expect(resent.status, JSON.stringify(resent.body)).toBe(200);
      expect(((await staff("alice").get(raisedId)).body as unknown as Record<string, unknown>).implementationPlan).toBe(storedPlan);
      expect(
        await psql(`SELECT count(*) FROM comment WHERE work_item_id = '${raisedId}' AND content LIKE '%${workNote}%';`),
        "the work note was recorded",
      ).toBe("1");

      // Request Approval reads "Schedule" (the template is pre-authorised, no customer approval): straight to Scheduled.
      await alice.reload();
      await expect(detail.lifecycleStepper()).toBeVisible({ timeout: 20_000 });
      await expect(detail.requestApprovalButton()).toHaveCount(0);
      await alice.getByRole("button", { name: "Schedule", exact: true }).click();
      await expect(detail.currentStep()).toContainText("Scheduled", { timeout: 15_000 });
      await expect(detail.stepLabels()).toHaveText([
        "New, done",
        "Assess, not taken",
        "Authorize, not taken",
        "Scheduled, current",
        "Implement, upcoming",
        "Review, upcoming",
        "Rollback, not taken",
        "Closed, upcoming",
        "Canceled, not taken",
      ]);
      await alice.getByRole("button", { name: "Start implementation" }).click();
      await expect(detail.currentStep()).toContainText("Implement", { timeout: 15_000 });
      await alice.getByRole("button", { name: "Mark implemented" }).click();
      await expect(detail.currentStep()).toContainText("Review", { timeout: 15_000 });
      await detail.changeStateButton().click();
      await detail.rollbackMenuItem().click();
      const reason = detail.reasonDialog();
      await reason.getByLabel("Reason").fill("The node did not rejoin the pool; restored the previous build.");
      await reason.getByRole("button", { name: "Roll back", exact: true }).click();
      await expect(detail.currentStep()).toContainText("Rollback", { timeout: 15_000 });
      await expect(detail.stage("Assess")).toHaveText("Assess, not taken");
      await expect(detail.stage("Authorize")).toHaveText("Authorize, not taken");

      // The template now needs review, linking to the change request that was rolled back.
      await alice.goto(templatePath);
      const review = main.getByRole("region", { name: "Review needed" });
      await expect(review).toBeVisible({ timeout: 20_000 });
      await expect(review).toContainText(/rolled back/i);
      await expect(main.getByText("Active", { exact: true })).toBeVisible();
      await shot(alice, "15-template-review-needed", review);
      await review.getByRole("button").filter({ hasNotText: "Clear review" }).click();
      await expect(alice).toHaveURL(new RegExp(`/operations/change-requests/${raisedId}$`));
      await expect(detail.currentStep()).toContainText("Rollback");

      // And the catalogue flags it.
      await alice.goto(STANDARD_CHANGES);
      await main.getByRole("textbox", { name: "Search templates by number or name" }).fill(TEMPLATE_NAME);
      const flagged = main.getByRole("table", { name: "Standard change templates" }).getByRole("row").filter({ hasText: TEMPLATE_NAME });
      await expect(flagged).toContainText("Review needed");
      await expect(flagged).toContainText("1 total, 0 open");
      await expect(flagged).toContainText("1 rolled back");
    } finally {
      await alice.context().close();
    }
  });
});
