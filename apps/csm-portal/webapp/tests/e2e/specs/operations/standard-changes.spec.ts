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
//   1. Operations > Change requests carries the standard change catalogue and
//      proposals as sub-views. Each is headed "Standard changes" with its own
//      line, and neither the heading nor the line says anything about service
//      requests, incidents, problems or runbooks; every other Operations tab
//      keeps the Operations heading. Every old link to the separate tab lands
//      on the right sub-view. The catalogue lists the seed's five templates
//      (scripts/csm-compose/seed-standard-changes.sh) with their states and
//      versions (the suspended one, the one at version 2), the suspended one's
//      page shows the reason it was suspended, the proposals sub-view lists the
//      seed's rejected, in-CAB-review and draft proposals and its revision, and
//      the sidebar's "Change requests" is the active entry on a template page
//      and on a proposal page.
//   2. bob proposes a standard change from a CLOSED change request (its detail
//      page's "Propose as standard change"), templates it (name, category, a
//      text and a select parameter, {{placeholders}} in the subject and the
//      implementation, backout and test plans, typed and inserted with
//      "Insert {{…}}"; the live preview renders every one with the sample
//      values), saves it and submits it to the CAB; carol, a CAB member,
//      approves it on the proposal page; the template is in the catalogue as
//      active version 1; alice raises a change request from it on the create
//      form (type Standard, the template, the change specifics, the read-only
//      preview). The change request holds the template's content rendered with
//      her values (subject, the three plans, impact, risk, category, rollback
//      duration), shows the "Raised from standard change template" banner, its
//      lifecycle bar never passes through Assess or Authorize, and in Edit
//      every field the template specifies is disabled while the others
//      (priority, assignment, affected services) stay editable. She schedules
//      it (Request Approval reads "Schedule"), implements, reviews and rolls it
//      back; the template then reads "Review needed", linking to that change
//      request.
//
// It WRITES to the stack: a closed change request raised and walked through the
// CSM backend's API (the evidence), the proposal, the template and the change
// request raised from it, all named with TAG. Before and after, the clean-up
// deletes from Postgres everything carrying TAG (and every change request raised
// from a TAG template), with the event-outbox rows of the records it deletes,
// and nothing else. So it runs only when the stack is named:
//
//   E2E_NO_WEBSERVER=1 E2E_POSTGRES_CONTAINER=csmcr-postgres-1 \
//   E2E_CSM_BFF_URL=http://localhost:8082 E2E_OIDC_URL=http://localhost:9100 \
//   E2E_ENTITY_SERVICE_URL=http://localhost:8081 \
//     npx playwright test tests/e2e/specs/operations/standard-changes.spec.ts
//
// The seed must have been run first (scripts/csm-compose/seed-standard-changes.sh).
// The personas sign in through the local mock identity provider (no password):
// bob proposes, carol decides as a member of the "CAB Approval" group (the
// proposer cannot decide his own), alice raises and moves the change request.

import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { ChangeRequestDetailPage } from "../../pages/ChangeRequestDetailPage";
import { EXAMPLE_CORP_ABT_GROUP_ID, realStackNamed, staff, walkTo } from "../../utils/realStackApi";

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
  impact: "Low",
  risk: "Low",
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

const USERS = {
  alice: "alice.perera@example.com",
  bob: "bob.fernando@example.com",
  carol: "carol.silva@example.com",
} as const;

/** The Operations heading, which every Operations tab but the two standard change sub-views keeps. */
const OPERATIONS_HEADING = {
  title: "Operations",
  subtitle: "Service requests, change requests, incidents, problems, and runbooks across customers.",
};
const CATALOGUE_HEADING = {
  title: "Standard changes",
  subtitle: "Pre-authorised change requests, raised from templates the CAB has approved.",
};
const PROPOSALS_HEADING = {
  title: "Standard changes",
  subtitle: "Proposals to the CAB to add, revise or retire a standard change template.",
};
/** What the user complained about: the standard change views headed with the entities they have nothing to do with. */
const OTHER_OPERATIONS_ENTITIES = /service requests?|incidents?|problems?|runbooks?/i;
/** The Operations tabs of the sidebar (a deployment may restrict some; those are skipped). */
const OPERATIONS_TABS = ["Service requests", "Change requests", "Incidents", "Problem management", "Outages", "Runbooks"];

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
const SUSPENDED_REASON_START = "A Traffic Manager restart on a clustered deployment dropped its throttling counters.";
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
  await expect(renderedSection(scope, "Implementation plan")).toHaveText(TEMPLATE.implementation(host, env));
  await expect(renderedSection(scope, "Backout plan")).toHaveText(TEMPLATE.backout(host));
  await expect(renderedSection(scope, "Test plan")).toHaveText(TEMPLATE.test(env));
  // Every exact text above has been seen, so this cannot pass on a preview that has not rendered yet.
  await expect(scope).not.toContainText("{{");
}

/** The Plan tab's section titled `title` (the text under its heading). */
function planSection(main: Locator, title: string): Locator {
  return main.getByText(title, { exact: true }).locator("xpath=following-sibling::*[1]");
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

  test("standard changes sit under Operations > Change requests, headed as such, old links land there, and the seed is listed", async ({ browser }) => {
    test.setTimeout(180_000);

    const page = await signedIn(browser, USERS.bob, "/operations");
    try {
      const main = page.getByRole("main");
      const nav = sidebar(page);
      await expectHeading(main, OPERATIONS_HEADING);
      // There is no "Standard changes" entry of its own in Operations.
      await expect(nav.getByRole("button", { name: /^Standard changes?$/ })).toHaveCount(0);

      // Every other Operations tab, the change request list included, keeps the Operations heading.
      for (const tab of OPERATIONS_TABS) {
        const entry = nav.getByRole("button", { name: tab, exact: true });
        if ((await entry.count()) === 0) continue;
        await entry.click();
        await expect(page, `the ${tab} tab`).toHaveURL(/\/operations\/[a-z-]+$/);
        await expectHeading(main, OPERATIONS_HEADING);
      }

      await nav.getByRole("button", { name: "Change requests", exact: true }).click();
      await expect(page).toHaveURL(/\/operations\/change-requests$/);
      await expectHeading(main, OPERATIONS_HEADING);
      const subNav = main.getByRole("tablist", { name: "Change requests" });
      await expect(subNav.getByRole("tab")).toHaveText(["Change requests", "Standard change catalogue", "Standard change proposals"]);
      await expect(subNav.getByRole("tab", { name: "Change requests" })).toHaveAttribute("aria-selected", "true");

      // ---- the catalogue: headed "Standard changes", the seed's five templates with their states and versions ----
      await subNav.getByRole("tab", { name: "Standard change catalogue" }).click();
      await expect(page).toHaveURL(/\/operations\/change-requests\?view=catalogue$/);
      await expectHeading(main, CATALOGUE_HEADING);
      await expectChangeRequestsActive(page);
      await expect(main.getByRole("button", { name: "Propose a standard change" })).toBeVisible();
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
      await shot(page, "01-catalogue-under-change-requests", main);

      // The suspended template's page says why, and the sidebar still highlights Change requests.
      await catalogue.getByRole("row").filter({ hasText: SUSPENDED_TEMPLATE }).click();
      await expect(page).toHaveURL(/\/operations\/standard-changes\/[^/?]+$/);
      await expect(main.getByRole("heading", { name: SUSPENDED_TEMPLATE })).toBeVisible();
      const suspended = main.getByRole("region", { name: "Suspended" });
      await expect(suspended).toContainText(`Suspended: ${SUSPENDED_REASON_START}`);
      await expect(suspended).toContainText(/\(by .+ on .+\)/);
      await expect(main.getByText("Version 1", { exact: true })).toBeVisible();
      await expect(main.getByRole("navigation", { name: "Breadcrumb" })).toContainText("Standard change catalogue");
      await expectChangeRequestsActive(page);
      await shot(page, "02-suspended-template-with-reason", suspended);

      // The revised template is at version 2.
      await page.goto("/operations/change-requests?view=catalogue");
      await templateSearch.fill(REVISED_TEMPLATE);
      await catalogue.getByRole("row").filter({ hasText: REVISED_TEMPLATE }).click();
      await expect(main.getByRole("heading", { name: REVISED_TEMPLATE })).toBeVisible();
      await expect(main.getByText("Version 2", { exact: true })).toBeVisible();
      await expect(main.getByText("Active", { exact: true })).toBeVisible();
      await expectChangeRequestsActive(page);

      // ---- the proposals: headed "Standard changes", bob's seeded proposals in their states ----------------------
      await page.goto("/operations/change-requests");
      await subNav.getByRole("tab", { name: "Standard change proposals" }).click();
      await expect(page).toHaveURL(/\/operations\/change-requests\?view=proposals$/);
      await expectHeading(main, PROPOSALS_HEADING);
      await expectChangeRequestsActive(page);
      const proposals = main.getByRole("table", { name: "Standard change proposals" });
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
      await shot(page, "03-proposals-under-change-requests", main);

      // A proposal page: the sidebar still highlights Change requests.
      const rejected = SEEDED_PROPOSALS[0];
      await proposals.getByRole("row").filter({ hasText: rejected.name }).click();
      await expect(page).toHaveURL(/\/operations\/standard-change-proposals\/[^/?]+$/);
      await expect(main.getByRole("heading", { name: rejected.name })).toBeVisible();
      await expect(main.getByText("Rejected", { exact: true }).first()).toBeVisible();
      await expect(main.getByRole("navigation", { name: "Breadcrumb" })).toContainText("Standard change proposals");
      await expectChangeRequestsActive(page);

      // ---- every old link to the separate tab lands on the right sub-view ----------------------------------------
      const legacy: Array<{ from: string; view: "catalogue" | "proposals" }> = [
        { from: "/operations?tab=standard_changes", view: "catalogue" },
        { from: "/operations?tab=standard_changes&view=proposals", view: "proposals" },
        { from: "/operations/standard-changes", view: "catalogue" },
        { from: "/operations/standard-changes?view=proposals", view: "proposals" },
      ];
      for (const { from, view } of legacy) {
        await page.goto(from);
        await expect(page, `${from} lands on the ${view}`).toHaveURL(new RegExp(`/operations/change-requests\\?view=${view}$`));
        if (view === "catalogue") {
          await expect(subNav.getByRole("tab", { name: "Standard change catalogue" })).toHaveAttribute("aria-selected", "true");
          await expect(catalogue).toBeVisible();
          await expectHeading(main, CATALOGUE_HEADING);
        } else {
          await expect(subNav.getByRole("tab", { name: "Standard change proposals" })).toHaveAttribute("aria-selected", "true");
          await expect(proposals).toBeVisible();
          await expectHeading(main, PROPOSALS_HEADING);
        }
        await expectChangeRequestsActive(page);
      }
    } finally {
      await page.context().close();
    }
  });

  test("propose from a closed change request, the CAB approves, raise a change request from the template, roll it back: the template needs review", async ({ browser }) => {
    test.setTimeout(300_000);

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
      await expect(main.getByRole("navigation", { name: "Breadcrumb" })).toContainText("Change requests");
      await expect(main.getByRole("navigation", { name: "Breadcrumb" })).toContainText("Standard change proposals");
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

      // What the template specifies, with placeholders in the subject and the three plans: typed, and through "Insert {{…}}".
      await specifies.getByRole("textbox", { name: /^Subject/ }).fill(TEMPLATE_SUBJECT);
      await pick(bob, specifies.getByRole("combobox", { name: "Category" }), TEMPLATE.category);
      await pick(bob, specifies.getByRole("combobox", { name: "Impact" }), TEMPLATE.impact);
      await pick(bob, specifies.getByRole("combobox", { name: "Risk" }), TEMPLATE.risk);
      await specifies.getByRole("textbox", { name: "Rollback duration" }).fill(TEMPLATE.rollbackDuration);
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

      // The live preview renders every placeholder with the sample values, the inserted ones included.
      const preview = main.getByRole("region", { name: "Preview", exact: true });
      await preview.getByRole("textbox", { name: /^Host name/ }).fill("cache-07");
      await preview.getByRole("combobox", { name: /^Environment/ }).selectOption("Staging");
      const rendered = preview.getByRole("region", { name: "Preview of a change request raised from this template" });
      await expectRenderedContent(rendered, "cache-07", "Staging");
      await shot(bob, "04-proposal-preview-with-sample-values", preview);

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
      await shot(bob, "05-proposal-in-cab-review", main.getByRole("region", { name: "CAB approval" }));
    } finally {
      await bob.context().close();
    }

    // ---- carol approves it for the CAB -------------------------------------------------------------------------
    const carol = await signedIn(browser, USERS.carol, proposalUrl);
    let templatePath = "";
    let templateNumber = "";
    try {
      const main = carol.getByRole("main");
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
      await expect(main.getByRole("heading", { name: TEMPLATE_NAME })).toBeVisible();
      await expect(main.getByText("Active", { exact: true })).toBeVisible();
      await expect(main.getByText("Version 1", { exact: true })).toBeVisible();
      await expect(main.getByText("Review needed", { exact: true })).toHaveCount(0);
      await expectChangeRequestsActive(carol);
      templateNumber = (await main.getByText(/^STD\d+$/).first().textContent())!.trim();
      await shot(carol, "06-template-active-v1");
    } finally {
      await carol.context().close();
    }

    // ---- alice finds it in the catalogue, raises a change request from it and walks it to a rollback ------------
    const alice = await signedIn(browser, USERS.alice, "/operations/change-requests?view=catalogue");
    try {
      const main = alice.getByRole("main");
      await main.getByRole("textbox", { name: "Search templates by number or name" }).fill(TEMPLATE_NAME);
      const row = main.getByRole("table", { name: "Standard change templates" }).getByRole("row").filter({ hasText: TEMPLATE_NAME });
      await expect(row).toHaveCount(1);
      await expect(row).toContainText(templateNumber);
      await expect(row).toContainText("v1");
      await expect(row).toContainText("Active");

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
      // Read-only: the template's text is shown, not offered as fields.
      await expect(preview.getByRole("textbox")).toHaveCount(0);
      await expect(alice.getByRole("textbox", { name: /^Subject/ })).toHaveCount(0);
      await shot(alice, "07-create-from-template", preview);
      const group = alice.getByRole("combobox", { name: /^Assignment group/ });
      await group.fill("Example Corp ABT");
      await alice.getByRole("option", { name: "Example Corp ABT" }).click();
      const [createdResponse] = await Promise.all([
        alice.waitForResponse((r) => r.request().method() === "POST" && /\/change-requests$/.test(r.url())),
        alice.getByRole("button", { name: "Create change request" }).click(),
      ]);
      expect(createdResponse.status(), await createdResponse.text()).toBe(201);
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
      await shot(alice, "08-raised-change-request-banner", banner);

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
      await shot(alice, "09-lifecycle-standard-change", detail.lifecycleStepper());

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
        "10-raised-change-request-plan",
        planSection(main, "Implementation plan").locator("xpath=ancestor::*[contains(@class, 'MuiCard-root')][1]"),
      );
      // The portal shows no risk on a change request: read it, and the rest once more, as the API returns them.
      const stored = await staff("alice").get(raisedId);
      expect(stored.status).toBe(200);
      const body = stored.body as unknown as Record<string, unknown>;
      expect(body.subject ?? body.title).toBe(subject);
      expect(String(body.impact).toLowerCase()).toBe("low");
      expect(String(typeof body.category === "object" && body.category ? (body.category as { id?: string }).id : body.category).toLowerCase()).toBe("software");
      expect(body.rollbackDurationText).toBe(TEMPLATE.rollbackDuration);
      expect(await psql(`SELECT risk FROM change_request WHERE id = '${raisedId}';`)).toBe("LOW");

      // Edit: every field the template specifies is disabled, the type stays Standard; the others stay editable.
      await detail.editButton().click();
      const edit = detail.editDialog();
      await expect(edit).toBeVisible();
      await expect(edit).toContainText(`Raised from standard change template ${templateNumber} (version 1)`);
      await expect(edit.getByRole("textbox", { name: /^Subject/ })).toBeDisabled();
      await expect(edit.getByRole("textbox", { name: /^Subject/ })).toHaveValue(subject);
      await expect(edit.getByRole("combobox", { name: /^Type/ })).toBeDisabled();
      await expect(edit.getByRole("combobox", { name: /^Type/ })).toContainText("Standard");
      await expect(edit.getByRole("combobox", { name: /^Impact/ })).toBeDisabled();
      await expect(edit.getByRole("combobox", { name: /^Category/ })).toBeDisabled();
      await expect(edit.getByRole("textbox", { name: /^Rollback duration/ })).toBeDisabled();
      await expect(edit.getByRole("textbox", { name: /^Rollback duration/ })).toHaveValue(TEMPLATE.rollbackDuration);
      await expect(edit.getByRole("checkbox", { name: "Customer Approval" })).toBeDisabled();
      await expect(edit.getByRole("checkbox", { name: "Customer Review" })).toBeDisabled();
      await expect(edit.getByRole("switch", { name: "Implementation Plan visible to customers" })).toBeDisabled();
      for (const label of ["Description", "Justification", "Risk and impact analysis", "Implementation plan", "Rollback plan", "Test plan"]) {
        await expect(richEditor(edit, label), `${label} is the template's`).toHaveAttribute("contenteditable", "false");
      }
      await expect(richEditor(edit, "Implementation plan")).toHaveText(TEMPLATE.implementation("cache-42", "Production"));
      await expect(edit.getByText("Set by the standard change template.").first()).toBeVisible();
      // What the template does not specify is still the engineer's to set.
      await expect(edit.getByRole("combobox", { name: /^Priority/ })).toBeEnabled();
      await expect(edit.getByRole("combobox", { name: /^Assignment group/ })).toBeEnabled();
      await expect(edit.getByRole("combobox", { name: /^Assigned to/ })).toBeEnabled();
      await expect(richEditor(edit, "Affected services")).toHaveAttribute("contenteditable", "true");
      await shot(alice, "11-edit-template-fields-disabled", edit);
      await edit.getByRole("button", { name: "Cancel" }).click();
      await expect(edit).toBeHidden();

      // Request Approval reads "Schedule" (the template is pre-authorised, no customer approval): straight to Scheduled.
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
      await shot(alice, "12-template-review-needed", review);
      await review.getByRole("button").filter({ hasNotText: "Clear review" }).click();
      await expect(alice).toHaveURL(new RegExp(`/operations/change-requests/${raisedId}$`));
      await expect(detail.currentStep()).toContainText("Rollback");

      // And the catalogue flags it.
      await alice.goto("/operations/change-requests?view=catalogue");
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
