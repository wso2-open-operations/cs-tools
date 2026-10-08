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

//
// Deterministic Change Request lifecycle coverage — the compulsory-
// assigned-team gate, Assess-entry approver auto-provisioning, the
// approve/cancel-sibling cascade from Assess to Authorize, a terminal
// approval display, and how the Approvals tab shows the customer group's
// Customer Approval / Customer Review stages and their outcome — run against
// the eight fixed-UUID fixtures in
// scripts/csm-compose/seed-entity-service.sql (CHG-FIXED-001..008), not a
// freshly self-provisioned CR the way change-request-detail.spec.ts works.
//
// That's a deliberate, necessary difference, not a style choice:
// CreateChangeRequest always 503s against this stack's own Postgres data
// source (work_item.number has no DB sequence — see entity-service's own
// CLAUDE.md, "CreateCase and case numbers"/"Change requests"), so there is
// no "create one, then drive it" path available locally at all. The fixed
// fixtures exist specifically to give this spec something to navigate
// straight to by id.
//
// The flip side of a fixed fixture: they get moved forward by exactly the
// transition this spec exercises (New -> Assess, an approval decision, a
// customer's answer, applied server-side). The seed is self-healing on purpose — re-running
// seed-entity-service.sql deletes and re-inserts the fixtures' approval stages
// and approvers and upserts their change_request rows back to the starting
// state — so resetFixtures() below simply re-runs that file against the
// already-running local docker-compose Postgres (`csmcr-postgres-1` here;
// E2E_POSTGRES_CONTAINER) before anything else runs. One source of truth: the
// starting state lives in the seed only.
//
// WHO acts is the point of the seed's personas (see "Local seed personas" in
// entity-service's CLAUDE.md), so the specs sign in as the persona that
// holds the seat — one captured session per role, minted by
// tests/e2e/auth/generate-session.spec.ts (see auth/README.md):
//
//   crApprover          jane.doe@example.com        internal, the requester persona
//   crInternalApprover  alice.perera@example.com    internal, peer/CAB approver
//
// Customers (the seed's dave.mendis / erin.jayawardena, contacts of project 401)
// do NOT sign in to the CSM portal: they answer Customer Approval / Customer
// Review in the customer portal. So no spec here drives a customer through the
// CSM UI. Where the customer's answer matters, it is applied server-side
// (utils/customerPortalDecision.ts on the real stack, `customerDecides` on the
// fake API) and the spec asserts what the CSM page then SHOWS, as an internal user.
//
// Runs only against the local stack (E2E_NO_WEBSERVER=1, see
// package.json's "test:e2e:cr-lifecycle"). A test whose persona has no
// captured session is skipped, not failed.
//

import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import type { Browser, Locator, Page } from "@playwright/test";
import { test, expect, withRole, hasSession, openContextAs, type TimecardRole } from "../../fixtures/test";
import { ChangeRequestCreatePage } from "../../pages/ChangeRequestCreatePage";
import { ChangeRequestDetailPage } from "../../pages/ChangeRequestDetailPage";
import { decideAsCustomer, proposeAsCustomer } from "../../utils/customerPortalDecision";
import { EXAMPLE_CORP, OTHER_CORP, ok, plan, raise, realStackNamed, stateOf, staff, walkTo, type ApiResult, type Raised, type WalkTarget } from "../../utils/realStackApi";
import {
  FAKE_CAB,
  FAKE_CAB_COLLEAGUE,
  FAKE_CAB_GROUP,
  FAKE_CAB_NO_EMAIL,
  FAKE_CR_ID,
  FAKE_CREATOR,
  FAKE_DEPLOYMENTS,
  FAKE_DEPLOYMENT_PRODUCTS,
  FAKE_ECAB,
  FAKE_ECAB_COLLEAGUE,
  FAKE_ECAB_GROUP,
  FAKE_BETA_CONTACT,
  FAKE_CUST_ONE,
  FAKE_CUST_TWO,
  FAKE_OUTSIDER,
  FAKE_PEER,
  FAKE_PEER_COLLEAGUE,
  FAKE_PEER_GROUP,
  FAKE_PROJECT_CONTACTS,
  FAKE_PROJECTS,
  ACCEPT_CANNOT_COMBINE,
  ACCEPT_NEEDS_EXPECTED,
  ACCEPT_NEEDS_EXPECTED_WINDOW,
  ACCEPT_ONLY_AGREE,
  ACCEPT_PROPOSER_NOT_RECORDED,
  ACCEPT_WINDOW_HAS_NO_LENGTH,
  COUNTER_IS_THE_PROPOSAL,
  ON_HOLD_MESSAGE,
  PROPOSAL_NO_LONGER_WAITING,
  acceptNotInCustomerApproval,
  acceptTooFarAheadMessage,
  customerAnswerRefusal,
  customerProposedWhileOpenMessage,
  customerStageManualRefusal,
  emergencyNoCustomerConsentMessage,
  finalStateMessage,
  installFakeChangeRequestApi,
  NO_PROPOSAL_WAITING,
  proposalChangedMessage,
  proposalPassedMessage,
  windowChangedMessage,
  projectFrozenMessage,
  requirementCannotBeRemovedMessage,
  requirementGatePassedMessage,
  requirementNeedsProjectMessage,
  NO_RECORDED_PROPOSAL_TO_DECLINE,
  nobodyToAskMessage,
  REQUEST_APPROVAL_NEEDS_PROJECT,
  CANNOT_RETURN_TO_NEW,
  stateJumpMessage,
  type FakeChangeRequestApi,
  type FakeUser,
} from "../../utils/fakeChangeRequestApi";

const CR_NO_TEAM = "00000000-0000-0000-0000-000000001001";
const CR_WITH_TEAM = "00000000-0000-0000-0000-000000001002";
const CR_PENDING_APPROVAL = "00000000-0000-0000-0000-000000001003";
const CR_RESOLVED = "00000000-0000-0000-0000-000000001004";
const CR_IN_REVIEW = "00000000-0000-0000-0000-000000001202"; // CHG-FIXED-006 (Review, Customer Review ticked)
const CR_CUSTOMER_APPROVAL = "00000000-0000-0000-0000-000000001303"; // CHG-FIXED-007
const CR_CUSTOMER_REVIEW = "00000000-0000-0000-0000-000000001304"; // CHG-FIXED-008

/** The seed's personas, by the display name the Approvals table shows. */
const ALICE = "Alice Perera"; // internal — peer / CAB approver
const BOB = "Bob Fernando"; // internal
const CAROL = "Carol Silva"; // internal
const DAVE = "Dave Mendis"; // external — registered contact of project 401
const ERIN = "Erin Jayawardena"; // external — registered contact of project 401
const JANE = "Jane Doe"; // internal requester persona, in no approval group
const JOHN = "John Smith"; // customer who is (deliberately) a member of the assigned group

/** What the Edit dialog says (and the backend's refusal means) once approval was requested; the web app's own words. */
const PROJECT_FROZEN_REASON = "Fixed when approval was requested. Cancel and clone to change it.";
const REQUIREMENT_ADD_ONLY_REASON = "Once approval has been requested a customer requirement can be added but never removed.";
const REQUIREMENT_NEEDS_PROJECT_REASON = "Needs a Customer Project, which can no longer be set. Cancel and clone.";
const REQUIREMENT_ONCE_SAVED = "Once saved this can't be removed.";
const REQUEST_APPROVAL_NEEDS_PROJECT_REASON = "Select a Customer Project before requesting approval";
const REQUEST_APPROVAL_NEEDS_CONTACT_REASON = "Register a contact for the Customer Project before requesting approval";

/** Absolute path of the seed file, from the webapp dir the specs run in. */
const SEED_FILE = path.resolve(process.cwd(), "../../../scripts/csm-compose/seed-entity-service.sql");
const POSTGRES_CONTAINER = process.env.E2E_POSTGRES_CONTAINER ?? "csm-platform-postgres-1";

/** Runs SQL on the local docker-compose Postgres (psql in the container) and
 * resolves with what it printed (unaligned, tuples only). With `sql` on stdin
 * so the whole seed file fits however large it is. */
async function psqlOutput(sql: string): Promise<string> {
  return await new Promise<string>((resolve, reject) => {
    const child = spawn("docker", [
      "exec", "-i", POSTGRES_CONTAINER, "psql", "-U", "postgres", "-d", "csm_platform", "-v", "ON_ERROR_STOP=1", "-q", "-t", "-A", "-f", "-",
    ]);
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (d: Buffer) => (stdout += d.toString()));
    child.stderr.on("data", (d: Buffer) => (stderr += d.toString()));
    child.on("error", reject);
    child.on("close", (code) =>
      code === 0 ? resolve(stdout.trim()) : reject(new Error(`psql exited ${code}: ${stderr}`)),
    );
    child.stdin.end(sql);
  });
}

async function psql(sql: string): Promise<void> {
  await psqlOutput(sql);
}

/** Restores the CHG-FIXED-* fixtures (and the personas) to their starting
 * state by re-running the self-healing seed — see this file's top comment. */
async function resetFixtures(): Promise<void> {
  await psql(fs.readFileSync(SEED_FILE, "utf8"));
}

/** Runs `fn` as `role`'s persona in a second, independent browser context. */
async function asPersona<T>(browser: Browser, role: TimecardRole, fn: (page: Page) => Promise<T>): Promise<T> {
  test.skip(
    !hasSession(role),
    `No captured session for '${role}'. See tests/e2e/auth/README.md to mint ` +
      `tests/e2e/storageState/${role}.json (E2E_AUTH_EMAIL=… E2E_AUTH_ROLE=${role}).`,
  );
  const context = await openContextAs(browser, role);
  try {
    return await fn(await context.newPage());
  } finally {
    await context.close();
  }
}

/** Opens the change request and captures the very headers the signed-in page itself sends to the BFF, and the
 * BFF's base URL, so a spec can make the same call by hand. */
async function apiSession(page: Page, crId: string): Promise<{ headers: Record<string, string>; base: string }> {
  const detail = new ChangeRequestDetailPage(page);
  const [request] = await Promise.all([
    page.waitForRequest((r) => r.method() === "GET" && new RegExp(`/change-requests/${crId}/approvals`).test(r.url())),
    detail.goto(crId),
  ]);
  const all = await request.allHeaders();
  const headers: Record<string, string> = {};
  for (const [name, value] of Object.entries(all)) {
    if (name === "authorization" || name.startsWith("x-")) headers[name] = value;
  }
  return { headers, base: request.url().replace(/\/change-requests\/.*$/, "") };
}

/** POSTs the caller's decision on a change request straight to the BFF with the
 * very headers the signed-in page itself sends, to see the status the API
 * answers — what the Approve button would have produced had it been rendered. */
async function postDecision(page: Page, crId: string, decision: "approved" | "rejected") {
  const { headers, base } = await apiSession(page, crId);
  const response = await page.request.post(`${base}/change-requests/${crId}/approvals/decision`, {
    headers,
    data: { decision },
  });
  return { status: response.status(), body: await response.text() };
}

/**
 * PATCHes `{ state }` straight to the BFF, to see what the API answers. The specs only ever send what the backend
 * REFUSES (a manual scheduled / closed out of a customer gate, whoever asks; a Roll back while the customer's review is
 * live), so nothing changes; `message` is the answer's own `message`.
 */
async function patchState(page: Page, crId: string, state: string): Promise<{ status: number; message: string }> {
  const { headers, base } = await apiSession(page, crId);
  const response = await page.request.patch(`${base}/change-requests/${crId}`, {
    headers: { ...headers, "content-type": "application/json" },
    data: { state },
  });
  const body = await response.text();
  try {
    return { status: response.status(), message: (JSON.parse(body) as { message?: string }).message ?? body };
  } catch {
    return { status: response.status(), message: body };
  }
}

withRole(test, "crApprover");

// The seeded-fixture describes below need the local docker-compose stack and
// reset its Postgres rows first; scoped to this wrapper so the approval-flow
// describes at the bottom of the file (which run against an in-browser fake of
// the change-request API, see utils/fakeChangeRequestApi.ts) don't need docker.
test.describe("seeded fixtures (local stack)", () => {
test.beforeAll(async () => {
  await resetFixtures();
});

test.describe("change request lifecycle — compulsory team gate", () => {
  test("Request Approval is disabled with no assigned team, and states why", async ({ page }) => {
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_NO_TEAM);

    const blocked = page.getByLabel(/Request Approval: .*assigned team/i);
    await expect(blocked).toBeVisible();
    await expect(blocked.getByRole("button", { name: "Request Approval" })).toBeDisabled();
    await expect(detail.scheduleButton()).toHaveCount(0);
  });
});

test.describe("change request lifecycle — Assess-entry auto-provisioning", () => {
  test("Request Approval succeeds once a team is assigned, and provisions that team's INTERNAL members as approvers", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_WITH_TEAM);

    const requestApprovalButton = detail.requestApprovalButton();
    await expect(requestApprovalButton).toBeEnabled();

    const [response] = await Promise.all([
      page.waitForResponse(
        (r) => new RegExp(`/change-requests/${CR_WITH_TEAM}$`).test(r.url()) && r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      detail.requestApproval(),
    ]);
    expect(response.ok(), `Request Approval PATCH failed (${response.status()})`).toBeTruthy();

    await expect(requestApprovalButton).toBeHidden({ timeout: 15_000 });

    // The assigned team's active internal members (Alice, Bob, Carol — see
    // scripts/csm-compose/seed-entity-service.sql) are now Requested peer
    // approvers, with no manual provisioning step...
    await expect(detail.approverStatus(ALICE, "Peer Approval")).toHaveText("Requested");
    await expect(detail.approverStatus(BOB, "Peer Approval")).toHaveText("Requested");
    await expect(detail.approverStatus(CAROL, "Peer Approval")).toHaveText("Requested");
    // ...and nobody else: John Smith is a member of the same group but a
    // customer (EXTERNAL), who could not even find this change request in his
    // own list, so he is never provisioned; Jane Doe is out of the group.
    await expect(detail.approverRow(JOHN)).toHaveCount(0);
    await expect(detail.approverRow(JANE)).toHaveCount(0);
  });
});

test.describe("change request lifecycle — approve cascades to Authorize", () => {
  test("an internal approver sees the peer stage and approving it cascades to Authorize (CAB) and cancels the others", async ({
    browser,
  }) => {
    test.setTimeout(90_000);

    await asPersona(browser, "crInternalApprover", async (page) => {
      const detail = new ChangeRequestDetailPage(page);
      await detail.goto(CR_PENDING_APPROVAL);

      // Only the signed-in user's (Alice's) own pending row renders an
      // Approve button — Bob's and Carol's sibling rows have none. Rows are
      // scoped to the "Peer Approval" stage because, once Alice approves, the
      // backend adds a CAB Approval stage listing the same three people.
      const PEER = "Peer Approval";
      await expect(detail.approverStatus(ALICE, PEER)).toHaveText("Requested");
      await expect(detail.approverStatus(BOB, PEER)).toHaveText("Requested");
      await expect(detail.approverStatus(CAROL, PEER)).toHaveText("Requested");
      await expect(detail.approveButton(BOB, PEER)).toHaveCount(0);
      await expect(detail.approveButton(CAROL, PEER)).toHaveCount(0);

      const approveButton = detail.approveButton(ALICE, PEER);
      await expect(approveButton).toBeVisible();

      const [response] = await Promise.all([
        page.waitForResponse((r) => /\/change-requests\/[^/]+\/approvals?/.test(r.url()), { timeout: 15_000 }),
        approveButton.click(),
      ]);
      expect(response.ok(), `Approve decision failed (${response.status()})`).toBeTruthy();

      await expect(detail.approverStatus(ALICE, PEER)).toHaveText("Approved");
      await expect(detail.approverStatus(BOB, PEER)).toHaveText("Cancelled");
      await expect(detail.approverStatus(CAROL, PEER)).toHaveText("Cancelled");

      // Peer approval cascades to the CAB Approval stage (its own group, the
      // next stage), the CR is in Authorize, and there is no Schedule button.
      await expect(detail.currentStep()).toContainText("Authorize");
      await expect(detail.blockingReason()).toHaveText("Awaiting CAB Approval");
      await expect(detail.approverStatus(ALICE, "CAB Approval")).toHaveText("Requested");
      await expect(detail.approverStatus(BOB, "CAB Approval")).toHaveText("Requested");
      await expect(detail.approverStatus(CAROL, "CAB Approval")).toHaveText("Requested");
      await expect(detail.scheduleButton()).toHaveCount(0);

      // ...and CAB approval by another internal user schedules it.
      await detail.approveButton(ALICE, "CAB Approval").click();
      await expect(detail.approverStatus(ALICE, "CAB Approval")).toHaveText("Approved");
      await expect(detail.currentStep()).toContainText("Scheduled");
    });
  });
});

test.describe("change request lifecycle — terminal approval display", () => {
  test("an already-decided change request shows its resolved approval state with no pending actions", async ({
    page,
  }) => {
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_RESOLVED);

    await expect(detail.approverStatus(ALICE, "Peer Approval")).toHaveText("Approved");
    await expect(detail.approverStatus(BOB, "Peer Approval")).toHaveText("Cancelled");
    await expect(detail.approverStatus(CAROL, "Peer Approval")).toHaveText("Cancelled");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
  });
});

test.describe("change request lifecycle — opening an Assignment group", () => {
  test("an internal stage's group opens and lists the people its pool is drawn from, in name order", async ({ page }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_RESOLVED);

    await detail.groupLink(ALICE, "Example Corp ABT", "Peer Approval").click();
    const dialog = detail.groupDialog("Example Corp ABT");
    await expect(dialog).toBeVisible();
    // Alice, Bob and Carol are the group's active internal members (the
    // approvers it provisions). Jane Doe is in the *team* of that name, not in
    // the group, and John Smith is a customer: neither is a peer approver, so
    // neither is listed.
    await expect(dialog.getByRole("heading", { name: "Group Members (3)" })).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveText([
      new RegExp(`${ALICE}.*alice\\.perera@example\\.com`),
      new RegExp(`${BOB}.*bob\\.fernando@example\\.com`),
      new RegExp(`${CAROL}.*carol\\.silva@example\\.com`),
    ]);
    await expect(dialog.getByText(JANE)).toHaveCount(0);
    await expect(dialog.getByText(JOHN)).toHaveCount(0);

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("the Customer Approval stage opens the Customer Group: the project's registered contacts", async ({ page }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_CUSTOMER_APPROVAL);

    await detail.groupLink(DAVE, "Customer Group", "Customer Approval").click();
    const dialog = detail.groupDialog("Customer Group");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Group Members (2)" })).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveText([
      new RegExp(`${DAVE}.*dave\\.mendis@example\\.com`),
      new RegExp(`${ERIN}.*erin\\.jayawardena@example\\.com`),
    ]);
  });
});

// ---------------------------------------------------------------------------
// The customer stages (CHG-FIXED-007 Customer Approval, -008 Customer Review). The
// customer answers in the CUSTOMER portal, never in this one, so the CSM page is only
// ever asked to SHOW the stage (its contacts, Requested) and, once the customer has
// answered, the outcome: the deciding contact's row Approved / Rejected, the other
// contact's Cancelled, the change Scheduled / Closed / Canceled / Rollback. The answer is
// applied through entity-service with the contact's own token, as the customer portal does
// (utils/customerPortalDecision.ts); the signed-in CSM user (jane.doe) only looks.
// ---------------------------------------------------------------------------

test.describe("change request lifecycle — the customer stages and the customer's answer", () => {
  // The answers consume the fixtures; the seed puts them back.
  test.beforeEach(async () => {
    await resetFixtures();
  });

  const DAVE_EMAIL = "dave.mendis@example.com";
  const ERIN_EMAIL = "erin.jayawardena@example.com";

  const ANSWERS = [
    {
      crId: CR_CUSTOMER_APPROVAL,
      fixture: "CHG-FIXED-007",
      stage: "Customer Approval",
      decider: { name: DAVE, email: DAVE_EMAIL },
      other: ERIN,
      decision: "approved",
      rowStatus: "Approved",
      state: "SCHEDULED",
      shown: "Scheduled",
      // The stepper afterwards: Customer Approval done, Scheduled current (Customer Review is not ticked on this fixture).
      stages: "d d d d c p p p n p n",
      flags: { approval: true, review: false },
    },
    {
      crId: CR_CUSTOMER_APPROVAL,
      fixture: "CHG-FIXED-007",
      stage: "Customer Approval",
      decider: { name: ERIN, email: ERIN_EMAIL },
      other: DAVE,
      decision: "rejected",
      rowStatus: "Rejected",
      state: "CANCELED",
      shown: "Canceled",
      // Canceled is current. The customer's rejection proves where the change ended: New to Authorize are done,
      // Customer Approval is rejected and nothing after it was ever reached (Customer Review is not ticked here).
      stages: "d d d r n n n n n n c",
      flags: { approval: true, review: false },
    },
    {
      crId: CR_CUSTOMER_REVIEW,
      fixture: "CHG-FIXED-008",
      stage: "Customer Review",
      decider: { name: ERIN, email: ERIN_EMAIL },
      other: DAVE,
      decision: "approved",
      rowStatus: "Approved",
      state: "CLOSED",
      shown: "Closed",
      // Closed is current and the customer's review is done (Customer Approval is not ticked on this fixture).
      stages: "d d d d d d d d n c n",
      flags: { approval: false, review: true },
    },
    {
      crId: CR_CUSTOMER_REVIEW,
      fixture: "CHG-FIXED-008",
      stage: "Customer Review",
      decider: { name: DAVE, email: DAVE_EMAIL },
      other: ERIN,
      decision: "rejected",
      rowStatus: "Rejected",
      state: "ROLLBACK",
      shown: "Rollback",
      // Rollback is current; the customer's rejection of the review is why, so Customer Review reads rejected.
      stages: "d d d d d d d r c n n",
      flags: { approval: false, review: true },
    },
  ] as const;

  for (const a of ANSWERS) {
    test(`${a.fixture} (${a.stage}): once the customer ${a.decision === "approved" ? "approves" : "rejects"} it, the Approvals tab shows ${a.decider.name} ${a.rowStatus}, ${a.other} Cancelled and the change ${a.shown}`, async ({
      page,
    }) => {
      test.setTimeout(90_000);

      // Waiting on the customer: both contacts' rows are Requested, and nobody in the CSM
      // portal has anything to decide (the signed-in user is no approver of this stage).
      const detail = new ChangeRequestDetailPage(page);
      await detail.goto(a.crId);
      await expect(detail.approverStatus(DAVE, a.stage)).toHaveText("Requested");
      await expect(detail.approverStatus(ERIN, a.stage)).toHaveText("Requested");
      await expect(detail.approveButton()).toHaveCount(0);
      await expect(detail.rejectButton()).toHaveCount(0);
      await expect(detail.currentStep()).toContainText(a.stage);

      // The customer answers in the customer portal.
      const answer = await decideAsCustomer(a.crId, a.decider.email, a.decision);
      expect(answer.status, answer.body).toBe(200);

      // The CSM page shows the outcome.
      await page.reload();
      await expect(detail.approverStatus(a.decider.name, a.stage)).toHaveText(a.rowStatus);
      await expect(detail.approverStatus(a.other, a.stage)).toHaveText("Cancelled");
      if (a.decision === "approved") {
        await expect(detail.currentStep()).toContainText(a.shown);
      } else {
        await expect(page.locator(".MuiChip-label", { hasText: new RegExp(`^${a.shown}$`) }).first()).toBeVisible();
      }
      await expect(detail.approveButton()).toHaveCount(0);
      await expect(detail.rejectButton()).toHaveCount(0);
      await expectStages(detail, a.stages, a.flags);
      expect(await psqlOutput(`SELECT state::text FROM change_request WHERE id = '${a.crId}';`)).toBe(a.state);
      // The rows as stored (by name): the decision is the row's state (APPROVED / REJECTED), the other contact's is CANCELLED.
      const rows = [`${a.decider.name}:${a.decision.toUpperCase()}`, `${a.other}:CANCELLED`].sort().join(", ");
      expect(await stageRows(a.crId, a.stage)).toBe(rows);
    });
  }

  test("an internal user who is not a contact of the project cannot answer the customer stages", async ({ browser }) => {
    test.setTimeout(90_000);

    await asPersona(browser, "crInternalApprover", async (page) => {
      const detail = new ChangeRequestDetailPage(page);
      for (const [crId, stage] of [
        [CR_CUSTOMER_APPROVAL, "Customer Approval"],
        [CR_CUSTOMER_REVIEW, "Customer Review"],
      ] as const) {
        await detail.goto(crId);
        await expect(detail.approverStatus(DAVE, stage)).toHaveText("Requested");
        await expect(detail.approveButton()).toHaveCount(0);
        await expect(detail.rejectButton()).toHaveCount(0);
      }

      // And the API says so, with the reason.
      const { status, body } = await postDecision(page, CR_CUSTOMER_APPROVAL, "approved");
      expect(status, body).toBe(403);
      expect(body).toContain("only members of the customer group");
      await detail.goto(CR_CUSTOMER_APPROVAL);
      await expect(detail.approverStatus(DAVE, "Customer Approval")).toHaveText("Requested");
      await expect(detail.currentStep()).toContainText("Customer Approval");
    });
  });
});

// ---------------------------------------------------------------------------
// An approval is only actionable while the change is in its stage's state, on the real
// stack (BFF -> entity-service -> Postgres). The reported bug: a reviewer kept Approve /
// Reject on the Review stage of a change that was already Closed (and during Customer
// Review). CHG-FIXED-002 is walked for real -- Request Approval, peer and CAB approval,
// implementation, Review (its Review stage is provisioned by the code under test) -- with
// Customer Review ticked, then moved on; a "legacy" stale row is planted directly (no code
// path writes one any more), refused with the BFF-passed 409, and repaired by migration 0193.
// ---------------------------------------------------------------------------

const MIGRATION_0193 = path.resolve(process.cwd(), "../../../entity-service/migrations/0193_change_request_cancel_stale_approvals.sql");

/** "Alice Perera:REQUESTED, ..." -- every approver row of the CR's stage with this label, by user name (state is UPPER_SNAKE_CASE, migration 0138). */
async function stageRows(crId: string, label: string): Promise<string> {
  return await psqlOutput(`
    SELECT COALESCE(string_agg(u.name || ':' || asa.state, ', ' ORDER BY u.name), '')
    FROM approval_stage_approver asa
    JOIN approval_stage ast ON ast.id = asa.stage_id
    JOIN "user" u ON u.id = asa.approver_user_id
    WHERE ast.work_item_id = '${crId}' AND ast.checkpoint_label = '${label}';`);
}

async function requestedRows(crId: string): Promise<number> {
  return Number(await psqlOutput(`SELECT COUNT(*) FROM approval_stage_approver WHERE work_item_id = '${crId}' AND state = 'REQUESTED';`));
}

test.describe("change request lifecycle — a Review approver's controls follow the state (real stack)", () => {
  test.beforeEach(async () => {
    await resetFixtures();
  });
  test.afterAll(async () => {
    await resetFixtures();
  });

  test("Review -> Customer Review -> Closed: the reviewers lose Approve / Reject when the change leaves Review, the customer answers (in the customer portal), nothing stays requested", async ({
    page,
    browser,
  }) => {
    test.setTimeout(240_000);
    // CHG-FIXED-002 (New, assigned group 901, project 401) with Customer Review ticked.
    await psql(`UPDATE change_request SET customer_review_required = true WHERE id = '${CR_WITH_TEAM}';`);

    // The requester asks for approval.
    const jane = new ChangeRequestDetailPage(page);
    await jane.goto(CR_WITH_TEAM);
    await jane.requestApproval();
    await expect(jane.approverStatus(ALICE, "Peer Approval")).toHaveText("Requested");

    await asPersona(browser, "crInternalApprover", async (alice) => {
      const detail = new ChangeRequestDetailPage(alice);
      await detail.goto(CR_WITH_TEAM);
      await detail.approveButton(ALICE, "Peer Approval").click();
      await expect(detail.currentStep()).toContainText("Authorize");
      await detail.approveButton(ALICE, "CAB Approval").click();
      await expect(detail.currentStep()).toContainText("Scheduled");
      await alice.getByRole("button", { name: "Start implementation" }).click();
      await expect(detail.currentStep()).toContainText("Implement");
      await alice.getByRole("button", { name: "Mark implemented" }).click();
      await expect(detail.currentStep()).toContainText("Review");

      // Review: provisioned for the assigned group's internal members; only the signed-in
      // user's own row has controls.
      for (const who of [ALICE, BOB, CAROL]) await expect(detail.approverStatus(who, "Review")).toHaveText("Requested");
      await expect(detail.approveButton(ALICE, "Review")).toBeEnabled();
      await expect(detail.rejectButton(ALICE, "Review")).toBeEnabled();
      await expect(detail.approveButton(CAROL, "Review")).toHaveCount(0);
      expect(await stageRows(CR_WITH_TEAM, "Review")).toBe("Alice Perera:REQUESTED, Bob Fernando:REQUESTED, Carol Silva:REQUESTED");

      // Moving on to the customer's review cancels every reviewer's row, Carol's included
      // (the row the report was about): nobody can approve or reject the Review stage now.
      await detail.sendForCustomerReviewButton().click();
      await expect(detail.currentStep()).toContainText("Customer Review");
      for (const who of [ALICE, BOB, CAROL]) await expect(detail.approverStatus(who, "Review")).toHaveText("Cancelled");
      await expect(detail.approveButton()).toHaveCount(0);
      await expect(detail.rejectButton()).toHaveCount(0);
      expect(await stageRows(CR_WITH_TEAM, "Review")).toBe("Alice Perera:CANCELLED, Bob Fernando:CANCELLED, Carol Silva:CANCELLED");
      await expect(detail.approverStatus(DAVE, "Customer Review")).toHaveText("Requested");
      await expect(detail.approverStatus(ERIN, "Customer Review")).toHaveText("Requested");
      // Even forcing the decision through the API: there is nothing pending to decide.
      const forced = await postDecision(alice, CR_WITH_TEAM, "approved");
      expect(forced.status, forced.body).toBe(403);
      expect(forced.body).toContain("only members of the customer group");
    });

    // The customer answers in the customer portal (applied the same way); the change closes,
    // and the CSM page shows it.
    const answer = await decideAsCustomer(CR_WITH_TEAM, "dave.mendis@example.com", "approved");
    expect(answer.status, answer.body).toBe(200);
    await jane.goto(CR_WITH_TEAM);
    await expect(jane.currentStep()).toContainText("Closed");
    await expect(jane.approverStatus(DAVE, "Customer Review")).toHaveText("Approved");
    await expect(jane.approverStatus(ERIN, "Customer Review")).toHaveText("Cancelled");

    // Closed: no approver row is requested anywhere, and the reviewer has no controls.
    expect(await requestedRows(CR_WITH_TEAM)).toBe(0);
    expect(await psqlOutput(`SELECT state::text FROM change_request WHERE id = '${CR_WITH_TEAM}';`)).toBe("CLOSED");
    await asPersona(browser, "crInternalApprover", async (alice) => {
      const detail = new ChangeRequestDetailPage(alice);
      await detail.goto(CR_WITH_TEAM);
      await expect(detail.currentStep()).toContainText("Closed");
      await expect(detail.approverStatus(CAROL, "Review")).toHaveText("Cancelled");
      await expect(detail.approveButton()).toHaveCount(0);
      await expect(detail.rejectButton()).toHaveCount(0);
    });
  });

  test("CHG-FIXED-006 (in Review, Customer Review ticked): sending it for customer review cancels the reviewers' rows, Carol's included", async ({ browser }) => {
    test.setTimeout(120_000);
    // The fixture is seeded in Review with no Review stage (nobody walked it there), so give it
    // what entering Review provisions: the assigned group's internal members, requested.
    const reviewStage = "00000000-0000-0000-0000-000000001921";
    await psql(`
      INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label)
        VALUES ('${reviewStage}', now(), now(), 'e2e', 'e2e', '${CR_IN_REVIEW}', '00000000-0000-0000-0000-000000000901', 'requested', 'Review')
        ON CONFLICT (id) DO NOTHING;
      INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state) VALUES
        ('00000000-0000-0000-0000-000000001922', now(), now(), 'e2e', 'e2e', '${reviewStage}', '${CR_IN_REVIEW}', '00000000-0000-0000-0000-000000000011', 'REQUESTED'),
        ('00000000-0000-0000-0000-000000001923', now(), now(), 'e2e', 'e2e', '${reviewStage}', '${CR_IN_REVIEW}', '00000000-0000-0000-0000-000000000012', 'REQUESTED'),
        ('00000000-0000-0000-0000-000000001924', now(), now(), 'e2e', 'e2e', '${reviewStage}', '${CR_IN_REVIEW}', '00000000-0000-0000-0000-000000000013', 'REQUESTED')
        ON CONFLICT (id) DO UPDATE SET state = 'REQUESTED';`);

    await asPersona(browser, "crInternalApprover", async (alice) => {
      const detail = new ChangeRequestDetailPage(alice);
      await detail.goto(CR_IN_REVIEW);
      await expect(detail.currentStep()).toContainText("Review");
      await expect(detail.approveButton(ALICE, "Review")).toBeEnabled();
      expect(await stageRows(CR_IN_REVIEW, "Review")).toBe("Alice Perera:REQUESTED, Bob Fernando:REQUESTED, Carol Silva:REQUESTED");

      await detail.sendForCustomerReviewButton().click();
      await expect(detail.currentStep()).toContainText("Customer Review");
      for (const who of [ALICE, BOB, CAROL]) await expect(detail.approverStatus(who, "Review")).toHaveText("Cancelled");
      await expect(detail.approveButton()).toHaveCount(0);
      await expect(detail.rejectButton()).toHaveCount(0);
      expect(await stageRows(CR_IN_REVIEW, "Review")).toBe("Alice Perera:CANCELLED, Bob Fernando:CANCELLED, Carol Silva:CANCELLED");
      // The customer contacts are asked instead.
      expect(await stageRows(CR_IN_REVIEW, "Customer Review")).toBe("Dave Mendis:REQUESTED, Erin Jayawardena:REQUESTED");
    });
  });

  test("a legacy REQUESTED Review row on a Closed change reads canDecide=false, the API refuses it with a 409 and migration 0193 repairs it", async ({ browser }) => {
    test.setTimeout(150_000);
    // The reported shape: a change that is Closed, with a Review stage whose reviewers
    // are still requested (what the database held before the fix). Planted directly.
    const reviewStage = "00000000-0000-0000-0000-000000001911";
    await psql(`
      UPDATE change_request SET state = 'CLOSED' WHERE id = '${CR_PENDING_APPROVAL}';
      -- its Peer stage was decided long ago (alice approved, the others cancelled)
      UPDATE approval_stage_approver SET state = CASE approver_user_id WHEN '00000000-0000-0000-0000-000000000011' THEN 'APPROVED' ELSE 'CANCELLED' END
        WHERE stage_id = '00000000-0000-0000-0000-000000001005';
      INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label)
        VALUES ('${reviewStage}', now() + interval '1 minute', now(), 'e2e', 'e2e', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000901', 'requested', 'Review')
        ON CONFLICT (id) DO NOTHING;
      INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state) VALUES
        ('00000000-0000-0000-0000-000000001912', now(), now(), 'e2e', 'e2e', '${reviewStage}', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000011', 'REQUESTED'),
        ('00000000-0000-0000-0000-000000001913', now(), now(), 'e2e', 'e2e', '${reviewStage}', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000013', 'REQUESTED')
        ON CONFLICT (id) DO UPDATE SET state = 'REQUESTED';`);

    await asPersona(browser, "crInternalApprover", async (alice) => {
      const detail = new ChangeRequestDetailPage(alice);
      await detail.goto(CR_PENDING_APPROVAL);
      await expect(detail.currentStep()).toContainText("Closed");
      // The row still reads Requested -- it is what the database holds -- but the API says
      // canDecide=false, so the controls are disabled (with the existing explanation).
      await expect(detail.approverStatus(ALICE, "Review")).toHaveText("Requested");
      await expect(detail.approveButton(ALICE, "Review")).toBeDisabled();
      await expect(detail.rejectButton(ALICE, "Review")).toBeDisabled();
      await expect(alice.getByLabel(/you aren't able to approve or reject this stage/i).first()).toBeVisible();

      // Forced through the API anyway: refused, with the readable 409 (passed through by the BFF).
      for (const decision of ["approved", "rejected"] as const) {
        const { status, body } = await postDecision(alice, CR_PENDING_APPROVAL, decision);
        expect(status, body).toBe(409);
        expect(body).toContain(
          "this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review",
        );
      }
      // Nothing changed.
      expect(await stageRows(CR_PENDING_APPROVAL, "Review")).toBe("Alice Perera:REQUESTED, Carol Silva:REQUESTED");
      expect(await psqlOutput(`SELECT state::text FROM change_request WHERE id = '${CR_PENDING_APPROVAL}';`)).toBe("CLOSED");
    });

    // The data fix: cancels the stale rows of the Closed change (both reviewers') and a labelled
    // stage's row on a change that is in another state (CHG-FIXED-004 sits in Authorize, so a
    // Peer Approval row still requested on it is stale), and leaves a live approval alone
    // (CHG-FIXED-008 is in Customer Review with its Customer Review stage requested).
    await psql(`
      INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
        VALUES ('00000000-0000-0000-0000-000000001914', now(), now(), 'e2e', 'e2e', '00000000-0000-0000-0000-000000001008', '${CR_RESOLVED}', '00000000-0000-0000-0000-000000000013', 'REQUESTED')
        ON CONFLICT (id) DO UPDATE SET state = 'REQUESTED';`);
    expect(await requestedRows(CR_RESOLVED)).toBe(1);
    const live = await requestedRows(CR_CUSTOMER_REVIEW);
    expect(live).toBe(2);
    await psql(fs.readFileSync(MIGRATION_0193, "utf8"));
    expect(await requestedRows(CR_PENDING_APPROVAL)).toBe(0);
    expect(await stageRows(CR_PENDING_APPROVAL, "Review")).toBe("Alice Perera:CANCELLED, Carol Silva:CANCELLED");
    expect(await requestedRows(CR_RESOLVED)).toBe(0);
    expect(await requestedRows(CR_CUSTOMER_REVIEW)).toBe(live);
    // Idempotent.
    await psql(fs.readFileSync(MIGRATION_0193, "utf8"));
    expect(await stageRows(CR_PENDING_APPROVAL, "Review")).toBe("Alice Perera:CANCELLED, Carol Silva:CANCELLED");

    await asPersona(browser, "crInternalApprover", async (alice) => {
      const detail = new ChangeRequestDetailPage(alice);
      await detail.goto(CR_PENDING_APPROVAL);
      await expect(detail.approverStatus(ALICE, "Review")).toHaveText("Cancelled");
      await expect(detail.approveButton()).toHaveCount(0);
      await expect(detail.rejectButton()).toHaveCount(0);
    });
  });
});

// ---------------------------------------------------------------------------
// No bypass, and the stepper, while the customer is being asked, on the real stack: CHG-FIXED-007 sits in
// Customer Approval and CHG-FIXED-008 in Customer Review, each with the project's contacts (Dave and Erin) still
// REQUESTED. The customer answers in the customer portal, and staff never answer for them: this only LOOKS at the
// page (the stepper, the action bar: nothing named Bypass, no Close at Customer Review) and asks the API for the
// manual answer, which the backend refuses -- whoever asks, asked or not. Nothing is changed.
// ---------------------------------------------------------------------------

test.describe("change request lifecycle — no bypass, and the stepper, while the customer is asked (real stack)", () => {
  // The describes before this one consume the fixtures; the seed puts them back.
  test.beforeAll(async () => {
    await resetFixtures();
  });

  /** Roll back at Customer Review: a failed review is the customer's to give, so it is held back while they are asked. */
  const PENDING_ROLLBACK = new RegExp(
    `^Customer review is pending from (${DAVE}, ${ERIN}|${ERIN}, ${DAVE})\\. A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here\\.$`,
  );

  test("CHG-FIXED-007 (Customer Approval, Dave and Erin asked): Re-schedule and Cancel change are all staff have, there is no Bypass entry, and the API refuses {state: scheduled}", async ({ page }) => {
    test.setTimeout(90_000);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_CUSTOMER_APPROVAL);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval"); // the approvals are loaded

    // The stepper: Customer Approval is ticked on this fixture and current; Customer Review is not ticked, so not plotted.
    await expectStages(detail, "d d d c p p p p n p n", { approval: true, review: false });
    // The bar: Re-schedule beside Change state (the main button); the menu holds Cancel change and nothing else.
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });

    // The backend refuses the manual answer with its own words, and nothing moved.
    const refused = await patchState(page, CR_CUSTOMER_APPROVAL, "scheduled");
    expect(refused.status, refused.message).toBe(400);
    expect(refused.message).toBe(customerAnswerRefusal("scheduled"));
    expect(await psqlOutput(`SELECT state::text FROM change_request WHERE id = '${CR_CUSTOMER_APPROVAL}';`)).toBe("CUSTOMER_APPROVAL");
    expect(await requestedRows(CR_CUSTOMER_APPROVAL)).toBe(2);
  });

  test("CHG-FIXED-008 (Customer Review, Dave and Erin asked): Roll back is a disabled Change state entry naming them, there is no Close and no Bypass, and the API refuses {state: closed} and {state: rollback}", async ({ page }) => {
    test.setTimeout(90_000);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_CUSTOMER_REVIEW);
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");

    // Customer Review is ticked and current; Customer Approval is not ticked on this fixture, so not plotted.
    await expectStages(detail, "d d d d d d d c n p n", { approval: false, review: true });
    // The bar: no button at all (the customer's review is theirs to give: no Close); the backend withdraws Roll back while the
    // review is pending too (a failed review is the customer's to give), so it is listed disabled, with why.
    await expectActionBar(detail, { primary: null, reschedule: false, menu: [/^Roll back/, "Cancel change"] });
    await expect(detail.closeButton()).toHaveCount(0);
    await detail.openChangeStateMenu();
    await expect(detail.rollbackMenuItem()).toBeDisabled();
    await expect(detail.rollbackMenuItem().getByText(PENDING_ROLLBACK)).toBeVisible();
    await detail.closeChangeStateMenu();

    const closed = await patchState(page, CR_CUSTOMER_REVIEW, "closed");
    expect([closed.status, closed.message], "closed").toEqual([400, customerAnswerRefusal("closed", true)]);
    const rolledBack = await patchState(page, CR_CUSTOMER_REVIEW, "rollback");
    expect([rolledBack.status, rolledBack.message], "rollback").toEqual([400, customerStageManualRefusal("rollback")]);
    expect(await psqlOutput(`SELECT state::text FROM change_request WHERE id = '${CR_CUSTOMER_REVIEW}';`)).toBe("CUSTOMER_REVIEW");
    expect(await requestedRows(CR_CUSTOMER_REVIEW)).toBe(2);
  });
});
});

//
// Approval-flow lifecycle per change type, against the in-browser fake in
// utils/fakeChangeRequestApi.ts (no records created, no seeded fixtures, no
// docker). Visible state is asserted after every step:
//
//   Normal    New -> Request Approval -> Assess [Peer Approval]
//                 -> Authorize [CAB Approval] -> (auto) Scheduled
//                 -> Implement -> Review -> Closed
//   Emergency New -> Request Approval -> Authorize [one CAB Approval stage: no
//                 Peer, no Assess, no ECAB (the previous system has none), no customer steps]
//                 -> (auto) Scheduled -> Implement -> Review -> Closed
//   Standard  New -> Request Approval -> (auto) Scheduled, no approvals
//
// With "Customer Approval" ticked, the Normal and Standard routes above stop at Customer
// Approval before Scheduled until the customer answers (in the customer portal);
// with "Customer Review" ticked, Review offers "Send for customer review" instead
// of "Close", then Customer Review waits for the customer's answer too (there is
// no Close button there). Staff never record a customer's approval or review:
// nothing in the page words answering for the customer (no "Bypass ...").
//
// Also asserts at every step that there is no "Schedule" button and no
// "Move to Assess" label, and that the CR's creator can Cancel but never
// Approve/Reject. Each "switch user" is a page reload with the faked
// `/users/me` identity changed. The browser is still signed in with the
// captured session so the portal boots normally.
//

async function openDetail(detail: ChangeRequestDetailPage): Promise<void> {
  await detail.goto(FAKE_CR_ID);
}

async function expectNoManualSchedule(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.scheduleButton()).toHaveCount(0);
  await expect(detail.page.getByText(/move to assess/i)).toHaveCount(0);
}

/**
 * Customer Approval / Customer Review appear on the stepper only when ticked;
 * Rollback, Closed and Canceled are always there (the customer portal's order).
 * Each stage's text is its label plus a visually-hidden ", <status>".
 */
async function expectCustomerStepsOnLine(
  detail: ChangeRequestDetailPage,
  flags: { approval: boolean; review: boolean },
): Promise<void> {
  const expected = ["New", "Assess", "Authorize"];
  if (flags.approval) expected.push("Customer Approval");
  expected.push("Scheduled", "Implement", "Review");
  if (flags.review) expected.push("Customer Review");
  expected.push("Rollback", "Closed", "Canceled");
  await expect(detail.stepLabels()).toHaveText(expected.map((label) => new RegExp(`^${label}, `)));
}

/** The eleven stages of the customer portal's workflow, in its order (what the stepper plots, left to right). */
const STAGE_LABELS = [
  "New",
  "Assess",
  "Authorize",
  "Customer Approval",
  "Scheduled",
  "Implement",
  "Review",
  "Customer Review",
  "Rollback",
  "Closed",
  "Canceled",
] as const;

/** One letter per stage status, and the words the stepper reads them as (visually hidden, after the label). */
const STAGE_STATUS_WORDS = {
  d: "done",
  c: "current",
  p: "upcoming",
  n: "not taken",
  u: "history not recorded",
  r: "rejected by the customer",
} as const;

/**
 * What the stepper must read, stage by stage ("New, done", "Review, current", ...), for the eleven
 * `columns` of the table (space-separated letters: d done, c current, p upcoming, n not taken, u history not
 * recorded, r rejected by the customer), with Customer Approval / Customer Review left off the line when their checkbox is off.
 */
function expectedStages(columns: string, flags: { approval: boolean; review: boolean }): string[] {
  const letters = columns.split(" ") as Array<keyof typeof STAGE_STATUS_WORDS>;
  expect(letters, `eleven columns in "${columns}"`).toHaveLength(STAGE_LABELS.length);
  return STAGE_LABELS.flatMap((label, i) => {
    if ((label === "Customer Approval" && !flags.approval) || (label === "Customer Review" && !flags.review)) return [];
    return [`${label}, ${STAGE_STATUS_WORDS[letters[i]!]}`];
  });
}

/** The stepper plots exactly the stages of `columns`, each with the status the table gives it. */
async function expectStages(
  detail: ChangeRequestDetailPage,
  columns: string,
  flags: { approval: boolean; review: boolean },
): Promise<void> {
  await expect(detail.stepLabels()).toHaveText(expectedStages(columns, flags));
  // Exactly one stage is the current one (the state), or none while the state is none of the eleven.
  await expect(detail.currentStep()).toHaveCount(columns.includes("c") ? 1 : 0);
}

/**
 * Staff never record a customer's approval or review, so nothing in the page words an engineer answering for the
 * customer ("Bypass customer approval" / "Bypass customer review", the retired "Record customer approval"): not as
 * text, not as a button, not as a "Change state" entry (enabled or disabled). The menu is opened to look, when there is one.
 */
async function expectNoBypass(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.answerForCustomerWording()).toHaveCount(0);
  if ((await detail.changeStateButton().count()) === 0) return;
  await detail.openChangeStateMenu();
  await expect(detail.answerForCustomerWording()).toHaveCount(0);
  await detail.closeChangeStateMenu();
}

test.describe("change request approval flow — Normal", () => {
  for (const approval of [false, true]) {
    for (const review of [false, true]) {
      test(`Normal, customer approval ${approval ? "on" : "off"}, customer review ${review ? "on" : "off"}: every step shows the right state and actions`, async ({
        page,
      }) => {
        test.setTimeout(120_000);
        // A customer step needs a Customer Project to ask (Request Approval is refused without one, and the project can no
        // longer be set after New); Acme's contacts are asked at each customer gate and answer in the customer portal
        // (applied server-side), since staff have no way to answer for them.
        const api = await installFakeChangeRequestApi(
          page,
          "normal",
          FAKE_CREATOR,
          { customerApprovalRequired: approval, customerReviewRequired: review },
          approval || review ? ON_ACME : {},
        );
        const detail = new ChangeRequestDetailPage(page);

        // New: the creator requests approval. The flags are shown read-only.
        await openDetail(detail);
        await expect(detail.currentStep()).toContainText("New");
        await expect(detail.flagValue("Customer approval required")).toHaveText(approval ? "Yes" : "No");
        await expect(detail.flagValue("Customer review required")).toHaveText(review ? "Yes" : "No");
        await expectCustomerStepsOnLine(detail, { approval, review });
        await expectNoManualSchedule(detail);
        await detail.requestApproval();

        // Peer Approval is pending; the creator can't decide but can still cancel.
        await expect(detail.currentStep()).toContainText("Assess");
        await expect(detail.blockingReason()).toHaveText("Awaiting Peer Approval");
        await expect(detail.approverStage("Pat Peer")).toHaveText("Peer Approval");
        await expect(detail.approveButton()).toHaveCount(0);
        await expect(detail.rejectButton()).toHaveCount(0);
        await expect(detail.creatorApprovalNotice()).toBeVisible();
        await detail.changeStateButton().click();
        await expect(detail.cancelChangeMenuItem()).toBeEnabled();
        await page.keyboard.press("Escape");
        await expectNoManualSchedule(detail);

        // A peer approves; CAB Approval is the next, separate stage.
        api.setViewer(FAKE_PEER);
        await page.reload();
        await expect(detail.approveButton("Pat Peer")).toBeVisible();
        await detail.approve("Pat Peer");
        await expect(detail.currentStep()).toContainText("Authorize");
        await expect(detail.blockingReason()).toHaveText("Awaiting CAB Approval");
        await expect(detail.approverStage("Cam Cab")).toHaveText("CAB Approval");
        await expect(detail.approverStage("Pat Peer")).toHaveText("Peer Approval");
        await expect(detail.approverStatus("Pat Peer")).toHaveText("Approved");
        await expectNoManualSchedule(detail);

        // A CAB member approves; the page refreshes itself to Customer
        // Approval when that box is ticked, else straight to Scheduled.
        api.setViewer(FAKE_CAB);
        await page.reload();
        await detail.approve("Cam Cab");
        expect(api.requests().some((r) => r === `POST /change-requests/${FAKE_CR_ID}/approvals/decision`)).toBe(true);

        api.setViewer(FAKE_CREATOR);
        await page.reload();
        if (approval) {
          await expect(detail.currentStep()).toContainText("Customer Approval");
          await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
          await expect(page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
          // The customer is asked, and only the customer's own answer moves the change on: staff have Re-schedule and Cancel change.
          await expectOnlyCancelActionable(detail, "approval");
          await expectNoManualSchedule(detail);
          const writesBefore = api.requests().filter(isWrite).length;
          await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
          // The answer came from the customer portal, not from this page: nothing was sent from it.
          expect(api.requests().filter(isWrite).length).toBe(writesBefore);
        } else {
          await expectNoBypass(detail);
        }

        // Scheduled: nothing awaited, no Schedule button.
        await expect(detail.currentStep()).toContainText("Scheduled");
        await expect(detail.blockingReason()).toHaveCount(0);
        await expectNoBypass(detail);
        await expectNoManualSchedule(detail);

        // The engineer-driven tail.
        await page.getByRole("button", { name: "Start implementation" }).click();
        await expect(detail.currentStep()).toContainText("Implement");
        await page.getByRole("button", { name: "Mark implemented" }).click();
        await expect(detail.currentStep()).toContainText("Review");
        if (review) {
          // Review offers only "Send for customer review" -- no Close.
          await expect(detail.sendForCustomerReviewButton()).toBeVisible();
          await expect(detail.closeButton()).toHaveCount(0);
          await detail.changeStateButton().click();
          await expect(page.getByRole("menuitem", { name: "Close", exact: true })).toHaveCount(0);
          await page.keyboard.press("Escape");
          await detail.sendForCustomerReviewButton().click();
          await expect(detail.currentStep()).toContainText("Customer Review");
          await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
          await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
          // Customer Review has no Close button, and none in the menu either: the review is the customer's to give.
          await expect(detail.closeButton()).toHaveCount(0);
          await expectOnlyCancelActionable(detail, "review");
          await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
        } else {
          // Review offers Close and no customer review.
          await expect(detail.closeButton()).toBeVisible();
          await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
          await detail.closeButton().click();
        }
        await expect(detail.currentStep()).toContainText("Closed");
        await expect(detail.blockingReason()).toHaveCount(0);
        await expectNoManualSchedule(detail);
        expect(api.state()).toBe("closed");
      });
    }
  }

  test("a non-creator approver sees Approve and Reject, with no creator notice", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");

    api.setViewer(FAKE_PEER);
    await page.reload();
    await expect(detail.approveButton("Pat Peer")).toBeEnabled();
    await expect(detail.rejectButton("Pat Peer")).toBeEnabled();
    await expect(detail.creatorApprovalNotice()).toHaveCount(0);
  });
});

test.describe("change request approval flow — Emergency", () => {
  test("Request Approval -> one CAB Approval (no Peer, no ECAB) -> auto Scheduled -> Implement -> Review -> Closed, with Assess never taken", async ({ page }) => {
    test.setTimeout(90_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    const none = { approval: false, review: false };

    await openDetail(detail);
    // The line: New, Assess (not taken, like Rollback / Canceled), Authorize, ...; no customer step is on it.
    await expectStages(detail, "c n p p p p p p n p n", none);
    await detail.requestApproval();

    await expect(detail.currentStep()).toContainText("Authorize");
    await expectStages(detail, "d n c p p p p p n p n", none);
    await expect(detail.blockingReason()).toHaveText("Awaiting CAB Approval");
    await expect(detail.approverStage("Cam Cab")).toHaveText("CAB Approval");
    await expect(page.getByRole("cell", { name: "Peer Approval", exact: true })).toHaveCount(0);
    await expect(page.getByText(/ECAB/)).toHaveCount(0);
    expect(api.stages().map((st) => st.stage)).toEqual(["CAB Approval"]);
    await expect(detail.approveButton()).toHaveCount(0); // creator
    await expectNoManualSchedule(detail);

    api.setViewer(FAKE_CAB);
    await page.reload();
    await detail.approve("Cam Cab");
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expectStages(detail, "d n d p c p p p n p n", none);
    await expectNoManualSchedule(detail);

    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    // Review closes it: an Emergency change never goes to a customer review.
    await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
    await detail.closeButton().click();
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
  });

  test("the customer's part reads Not applicable on the Approval tab, from New to Scheduled", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR, {}, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    const labels = ["Customer approval required", "Customer review required", "Customer approved", "Customer reviewed"];

    await openDetail(detail);
    for (const label of labels) await expect(detail.flagValue(label)).toHaveText("Not applicable");
    await detail.requestApproval();
    api.setViewer(FAKE_CAB);
    await page.reload();
    await detail.approve("Cam Cab");
    await expect(detail.currentStep()).toContainText("Scheduled");
    for (const label of labels) await expect(detail.flagValue(label)).toHaveText("Not applicable");
  });
});

// Retired: "Emergency with Customer Approval" (ECAB approval stops at Customer Approval; the customer's answer schedules it).
// An Emergency change acts without the customer's consent: even on a project with registered contacts nobody is asked.
test.describe("change request approval flow — an Emergency change never reaches a customer state", () => {
  test("CAB approval schedules it straight away on a project with registered contacts, and no customer stage or request is ever made", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR, {}, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.blockingReason()).toHaveText("Awaiting CAB Approval");
    api.setViewer(FAKE_CAB);
    await page.reload();
    await detail.approve("Cam Cab");

    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(page.getByRole("cell", { name: "Customer Approval", exact: true })).toHaveCount(0);
    expect(api.stages().map((st) => st.stage)).toEqual(["CAB Approval"]);
    await expectNoBypass(detail);
    await expectNoManualSchedule(detail);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
  });

  test("a project nobody on which can be asked does not hold Request Approval back: there is nobody to ask for", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR, {}, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(detail.requestApprovalButton()).toBeEnabled();
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Authorize");
    expect(api.state()).toBe("authorize");
  });

  test("the API refuses a customer box on an Emergency change in the backend's words (create and PATCH), and moves nothing", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR, {}, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    for (const [field, other] of [
      ["customerApprovalRequired", "customerReviewRequired"],
      ["customerReviewRequired", "customerApprovalRequired"],
    ] as const) {
      const refused = await patchFromPage(page, { [field]: true });
      expect(refused, field).toEqual({ status: 400, message: emergencyNoCustomerConsentMessage([field]) });
      expect(api.flags()[other], field).toBe(false);
    }
    expect(api.flags()).toEqual({ customerApprovalRequired: false, customerReviewRequired: false });
    expect(api.state()).toBe("new");
    // A box it already holds is a no-op, accepted: a client that sends the whole form back is not punished.
    expect((await patchFromPage(page, { customerApprovalRequired: false, customerReviewRequired: false })).status).toBe(200);
  });

  test("an OLDER Emergency change with a live ECAB stage still shows it, its asked approver can still decide it, and the CAB approval schedules it", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR);
    api.startOlderEmergencyAtAuthorize();
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.blockingReason()).toHaveText("Awaiting ECAB Approval");
    await expect(detail.approverStage("Eli Ecab")).toHaveText("ECAB Approval");
    await expect(detail.approveButton()).toHaveCount(0); // creator

    // Cam Cab is a CAB member, but the stage asked Eli Ecab: a decision needs the caller's own REQUESTED row.
    api.setViewer(FAKE_CAB);
    await page.reload();
    await expect(detail.approveButton()).toHaveCount(0);

    api.setViewer(FAKE_ECAB);
    await page.reload();
    await detail.approve("Eli Ecab");
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(detail.approverStatus("Eli Ecab")).toHaveText("Approved");
    expect(api.state()).toBe("scheduled");
  });
});

test.describe("change request approval flow — Standard with Customer Approval", () => {
  test("Request Approval goes to Customer Approval (not Scheduled), and only the customer's answer schedules it", async ({
    page,
  }) => {
    const api = await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await detail.requestApproval();

    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByText(/no approval stages recorded/i)).toHaveCount(0); // the customer's stage is the one stage it has
    await expect(page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
    await expectNoManualSchedule(detail);
    await expectOnlyCancelActionable(detail, "approval");

    await customerAnswers(page, api, FAKE_CUST_TWO, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
  });
});

test.describe("change request approval flow — editing the customer checkboxes", () => {
  test("both are editable before their gate, are sent via PATCH, and show on the Approval tab afterwards", async ({
    page,
  }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(detail.flagValue("Customer approval required")).toHaveText("No");

    await detail.openEditDialog();
    await expect(detail.editCustomerApprovalCheckbox()).not.toBeChecked();
    await expect(detail.editCustomerApprovalCheckbox()).toBeEnabled();
    await expect(detail.editCustomerReviewCheckbox()).toBeEnabled();
    await detail.editCustomerApprovalCheckbox().check();
    await detail.editCustomerReviewCheckbox().check();
    const [request] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    expect(request.postDataJSON()).toEqual({ customerApprovalRequired: true, customerReviewRequired: true });
    await expect(detail.editDialog()).toHaveCount(0);

    expect(api.flags()).toEqual({ customerApprovalRequired: true, customerReviewRequired: true });
    await expect(detail.flagValue("Customer approval required")).toHaveText("Yes");
    await expect(detail.flagValue("Customer review required")).toHaveText("Yes");
    await expectCustomerStepsOnLine(detail, { approval: true, review: true });
    await expectStages(detail, "c p p p p p p p n p n", { approval: true, review: true });
  });

  test("Customer Approval is disabled with an explanation once the CR is scheduled; Customer Review can still be added (the project is stored)", async ({
    page,
  }) => {
    await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR, {}, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Scheduled");

    await detail.openEditDialog();
    await expect(detail.editCustomerApprovalCheckbox()).toBeDisabled();
    await expect(detail.editDialog().getByText(/locked/i).first()).toBeVisible();
    await expect(detail.editCustomerReviewCheckbox()).toBeEnabled();
    await expect(detail.editDialog().getByText(REQUIREMENT_ONCE_SAVED)).toBeVisible();
  });

  test("Customer Review is disabled once the CR has reached customer review", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR, { customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    api.setState("customer_review");
    await page.reload();
    await expect(detail.currentStep()).toContainText("Customer Review");

    await detail.openEditDialog();
    await expect(detail.editCustomerReviewCheckbox()).toBeDisabled();
    await expect(detail.editCustomerApprovalCheckbox()).toBeDisabled();
  });

  test("shows the backend's refusal when the gate passed while the dialog was open (400)", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.openEditDialog();
    await detail.editCustomerApprovalCheckbox().check();

    // The CR moves on behind the open dialog's back; the backend refuses.
    api.setState("scheduled");
    await detail.saveEdit();
    await expect(detail.editDialog().getByRole("alert")).toContainText(
      requirementGatePassedMessage("customerApprovalRequired", "scheduled"),
    );
    await expect(detail.editDialog()).toBeVisible();
    expect(api.flags().customerApprovalRequired).toBe(false);
  });
});

/** What the Edit dialog says about one customer requirement's box: enabled (and the line under it), or disabled with the reason. */
type BoxRule = { enabled: true; says?: string } | { enabled: false; says: string };
const GATE_PASSED = (box: "approval" | "review") => `Locked: the change request has already reached the customer ${box} step or later.`;
const OPEN = (): BoxRule => ({ enabled: true });
const ADDABLE: BoxRule = { enabled: true, says: REQUIREMENT_ONCE_SAVED };
const ADD_ONLY: BoxRule = { enabled: false, says: REQUIREMENT_ADD_ONLY_REASON };
const NEEDS_PROJECT: BoxRule = { enabled: false, says: REQUIREMENT_NEEDS_PROJECT_REASON };
const PASSED = (box: "approval" | "review"): BoxRule => ({ enabled: false, says: GATE_PASSED(box) });

// The Edit dialog in EVERY state, against the fake of the backend (`api.setState` puts the change in the state out of band,
// the dialog is opened on that stored state): the table of the lock, row by row, as the product owner decided it and as the
// Vitest table (`changeRequests.test.ts`) and the Go truth table state it. The server stays the authority (see "the customer
// requirements lock (real stack)"), the dialog says it up front.
//   state | stored boxes | project | the Customer Project | the Customer Approval box | the Customer Review box
const DIALOG_TABLE: Array<{
  state: string;
  approval: boolean;
  review: boolean;
  project: boolean;
  projectEditable: boolean;
  approvalBox: BoxRule;
  reviewBox: BoxRule;
}> = [
  { state: "new", approval: false, review: false, project: false, projectEditable: true, approvalBox: OPEN(), reviewBox: OPEN() },
  { state: "new", approval: true, review: true, project: true, projectEditable: true, approvalBox: OPEN(), reviewBox: OPEN() },
  { state: "assess", approval: true, review: false, project: true, projectEditable: false, approvalBox: ADD_ONLY, reviewBox: ADDABLE },
  { state: "assess", approval: false, review: false, project: false, projectEditable: false, approvalBox: NEEDS_PROJECT, reviewBox: NEEDS_PROJECT },
  { state: "authorize", approval: false, review: false, project: true, projectEditable: false, approvalBox: ADDABLE, reviewBox: ADDABLE },
  { state: "authorize", approval: true, review: true, project: true, projectEditable: false, approvalBox: ADD_ONLY, reviewBox: ADD_ONLY },
  { state: "customer_approval", approval: true, review: false, project: true, projectEditable: false, approvalBox: ADD_ONLY, reviewBox: ADDABLE },
  { state: "scheduled", approval: false, review: false, project: true, projectEditable: false, approvalBox: PASSED("approval"), reviewBox: ADDABLE },
  { state: "scheduled", approval: false, review: false, project: false, projectEditable: false, approvalBox: PASSED("approval"), reviewBox: NEEDS_PROJECT },
  { state: "implement", approval: true, review: false, project: true, projectEditable: false, approvalBox: ADD_ONLY, reviewBox: ADDABLE },
  { state: "review", approval: false, review: false, project: true, projectEditable: false, approvalBox: PASSED("approval"), reviewBox: ADDABLE },
  { state: "review", approval: false, review: true, project: true, projectEditable: false, approvalBox: PASSED("approval"), reviewBox: ADD_ONLY },
  { state: "customer_review", approval: true, review: true, project: true, projectEditable: false, approvalBox: ADD_ONLY, reviewBox: ADD_ONLY },
  { state: "customer_review", approval: false, review: false, project: true, projectEditable: false, approvalBox: PASSED("approval"), reviewBox: PASSED("review") },
  { state: "closed", approval: false, review: false, project: true, projectEditable: false, approvalBox: PASSED("approval"), reviewBox: PASSED("review") },
  { state: "canceled", approval: true, review: false, project: true, projectEditable: false, approvalBox: ADD_ONLY, reviewBox: PASSED("review") },
];

test.describe("change request approval flow — the Edit dialog follows the lock in every state", () => {
  for (const row of DIALOG_TABLE) {
    const boxes = `approval ${row.approval ? "ticked" : "unticked"}, review ${row.review ? "ticked" : "unticked"}, ${row.project ? "a project" : "no project"}`;
    test(`${row.state} (${boxes}): the project is ${row.projectEditable ? "editable" : "read-only with its reason"}, the approval box ${row.approvalBox.enabled ? "open" : "read-only"}, the review box ${row.reviewBox.enabled ? "open" : "read-only"}`, async ({ page }) => {
      const api = await installFakeChangeRequestApi(
        page,
        "normal",
        FAKE_CREATOR,
        { customerApprovalRequired: row.approval, customerReviewRequired: row.review },
        row.project ? ON_ACME : {},
      );
      const detail = new ChangeRequestDetailPage(page);
      await openDetail(detail);
      if (row.state !== "new") {
        api.setState(row.state);
        await page.reload();
      }
      await detail.openEditDialog();

      if (row.projectEditable) {
        await expect(detail.editProjectField()).toBeEnabled();
        await expect(detail.editDialog().getByText(PROJECT_FROZEN_REASON)).toHaveCount(0);
      } else {
        await expect(detail.editProjectField()).toBeDisabled();
        await expect(detail.editDialog().getByText(PROJECT_FROZEN_REASON)).toBeVisible();
      }
      for (const [box, rule] of [["approval", row.approvalBox], ["review", row.reviewBox]] as const) {
        const checkbox = box === "approval" ? detail.editCustomerApprovalCheckbox() : detail.editCustomerReviewCheckbox();
        const stored = box === "approval" ? row.approval : row.review;
        if (stored) await expect(checkbox, `${box} box ticked`).toBeChecked();
        else await expect(checkbox, `${box} box unticked`).not.toBeChecked();
        if (rule.enabled) await expect(checkbox, `${box} box`).toBeEnabled();
        else await expect(checkbox, `${box} box`).toBeDisabled();
        const line = detail.editDialog().locator(`#cr-edit-customer-${box}-desc`);
        if (rule.says) await expect(line, `${box} box line`).toContainText(rule.says);
        else for (const forbidden of [REQUIREMENT_ADD_ONLY_REASON, REQUIREMENT_NEEDS_PROJECT_REASON, REQUIREMENT_ONCE_SAVED, "Locked:"]) await expect(line).not.toContainText(forbidden);
      }
    });
  }

  test("a client that resends the whole form is not punished: the stored project and boxes are accepted, and saving changes nothing it should not", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    expect(await patchFromPage(page, { projectId: ACME.id, customerApprovalRequired: true, customerReviewRequired: false })).toMatchObject({ status: 200 });
    expect(api.scope().projectId).toBe(ACME.id);
    // The same refusals the real backend gives, in its words, from the fake: a swap, an untick, back to New.
    expect(await patchFromPage(page, { projectId: BETA.id })).toEqual({ status: 400, message: projectFrozenMessage("assess") });
    expect(await patchFromPage(page, { customerApprovalRequired: false })).toEqual({ status: 400, message: requirementCannotBeRemovedMessage("customerApprovalRequired", "assess") });
    expect(await patchFromPage(page, { state: "new" })).toEqual({ status: 400, message: CANNOT_RETURN_TO_NEW });
    expect(api.flags()).toEqual({ customerApprovalRequired: true, customerReviewRequired: false });
  });

  test("no Customer Project: a box cannot be turned on after New, in the backend's words; and Request Approval with a box ticked and no project is refused in them too", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true });
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    // Disabled in the action bar, with why
    const blocked = page.getByLabel(new RegExp(`Request Approval: .*${REQUEST_APPROVAL_NEEDS_PROJECT_REASON}`, "i"));
    await expect(blocked.getByRole("button", { name: "Request Approval" })).toBeDisabled();
    expect(await patchFromPage(page, { state: "assess" })).toEqual({ status: 400, message: REQUEST_APPROVAL_NEEDS_PROJECT });
    expect(api.state()).toBe("new");
    // Un-ticking in the same PATCH, or picking a project in it, lets it through.
    expect(await patchFromPage(page, { state: "assess", customerApprovalRequired: false })).toMatchObject({ status: 200 });
    expect(await patchFromPage(page, { customerReviewRequired: true })).toEqual({ status: 400, message: requirementNeedsProjectMessage("customerReviewRequired") });
  });
});

test.describe("change request approval flow — Standard", () => {
  test("Request Approval goes straight to Scheduled, with no approval stages", async ({ page }) => {
    await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);

    await openDetail(detail);
    await detail.requestApproval();

    await expect(detail.currentStep()).toContainText("Scheduled");
    await expect(page.getByText(/no approval stages recorded/i)).toBeVisible();
    await expect(detail.blockingReason()).toHaveCount(0);
    await expectNoManualSchedule(detail);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Customer Project / Deployments / Deployment products (and the read-only Customer Group) on a
// change request, end to end against the in-browser fake backend: create ->
// detail Overview -> edit (cascade, whole-scope PATCH, backend refusal) ->
// approvals -> locked once implementation starts.
// ---------------------------------------------------------------------------

const ACME = FAKE_PROJECTS[0]!;
const BETA = FAKE_PROJECTS[1]!;
const [ACME_PROD, ACME_STG, BETA_DEV] = FAKE_DEPLOYMENTS;
const GAMMA = FAKE_PROJECTS[2]!;
const ACME_CONTACTS = FAKE_PROJECT_CONTACTS[ACME.id]!.map((u) => u.name);
const BETA_CONTACTS = FAKE_PROJECT_CONTACTS[BETA.id]!.map((u) => u.name);
const productsOf = (deploymentId: string): string[] =>
  FAKE_DEPLOYMENT_PRODUCTS.filter((p) => p.deploymentId === deploymentId).map((p) => p.name);

test.describe("change request lifecycle — project and deployments (mocked backend)", () => {
  test("Normal change: create with project + deployments, see them on the detail page, edit the scope, approve, and have it locked once implementing", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const create = new ChangeRequestCreatePage(page);
    const detail = new ChangeRequestDetailPage(page);

    // 1. Create a Normal change with a project, two deployments and a category.
    await create.goto();
    await create.selectType("Normal");
    await create.subjectField().fill("[E2E] project + deployments lifecycle (mocked)");
    await create.selectProject(ACME.name);
    await create.selectDeployments([ACME_PROD!.name, ACME_STG!.name]);
    await create.selectCategory("DevOps");
    await create.createButton().click();
    await expect(page).toHaveURL(new RegExp(`/operations/change-requests/${FAKE_CR_ID}$`));
    await expect(detail.lifecycleStepper()).toBeVisible();

    // 2. The detail Overview shows every one of them.
    await expect(detail.overviewCell("Customer Project")).toContainText(ACME.name);
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name, ACME_STG!.name]);
    await expect(detail.page.getByText("Environments", { exact: true })).toHaveCount(0);
    await expect(detail.overviewChips("Deployment products")).toHaveText([
      ...productsOf(ACME_PROD!.id),
      ...productsOf(ACME_STG!.id),
    ]);
    // The Customer Group is the project's registered contacts, derived and read-only.
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    await expect(detail.overviewCell("Category")).toContainText("DevOps");
    await expect(detail.currentStep()).toContainText("New");

    // 3. Request Approval, then edit the scope while it is still editable (Assess).
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toHaveValue(ACME.name);
    await expect(detail.editChipsOf(detail.editDeploymentsField())).toHaveText([ACME_PROD!.name, ACME_STG!.name]);
    await detail.editToggleOptions(detail.editDeploymentsField(), [ACME_STG!.name]); // drop Staging
    await expect(detail.editChipsOf(detail.editDeploymentProductsField())).toHaveText(productsOf(ACME_PROD!.id));
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(ACME_CONTACTS);
    const [patch] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    // The whole scope goes out together, with the exact wire names.
    expect(patch.postDataJSON()).toEqual({
      projectId: ACME.id,
      deploymentIds: [ACME_PROD!.id],
      deploymentProductIds: FAKE_DEPLOYMENT_PRODUCTS.filter((p) => p.deploymentId === ACME_PROD!.id).map((p) => p.id),
    });
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name]);
    await expect(detail.overviewChips("Deployment products")).toHaveText(productsOf(ACME_PROD!.id));
    expect(api.scope().deploymentIds).toEqual([ACME_PROD!.id]);

    // 4. Peer and CAB approve; the creator starts implementation.
    api.setViewer(FAKE_PEER);
    await page.reload();
    await detail.approve("Pat Peer");
    await expect(detail.currentStep()).toContainText("Authorize");
    api.setViewer(FAKE_CAB);
    await page.reload();
    await detail.approve("Cam Cab");
    api.setViewer(FAKE_CREATOR);
    await page.reload();
    await expect(detail.currentStep()).toContainText("Scheduled");

    // Still editable while Scheduled ...
    await detail.openEditDialog();
    await expect(detail.editDeploymentsField()).toBeEnabled();
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();

    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");

    // 5. ... and locked, with the reason, from Implement on. The detail still shows it.
    await detail.openEditDialog();
    await expect(detail.editDialog().getByText(/can't be changed once implementation has started/i)).toBeVisible();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editDeploymentsField()).toBeDisabled();
    await expect(detail.saveButton()).toBeDisabled();
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name]);
  });

  test("editing: changing the project clears the dependents and sends the new project with empty lists", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, {
      projectId: ACME.id,
      deploymentIds: [ACME_PROD!.id],
    });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await expect(detail.overviewChips("Deployments")).toHaveText([ACME_PROD!.name]);

    await detail.openEditDialog();
    // A saved project can be swapped but not cleared.
    await detail.editProjectField().click();
    await page.getByRole("option", { name: BETA.name }).click();
    await expect(detail.editChipsOf(detail.editDeploymentsField())).toHaveCount(0);
    await expect(detail.editChipsOf(detail.editDeploymentProductsField())).toHaveCount(0);
    // The read-only Customer Group follows the project: customer B's contacts replace customer A's.
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(BETA_CONTACTS);
    const [patch] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    expect(patch.postDataJSON()).toEqual({
      projectId: BETA.id,
      deploymentIds: [],
      deploymentProductIds: [],
    });
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewCell("Customer Project")).toContainText(BETA.name);
    await expect(detail.overviewCell("Deployments")).toContainText("—");
    await expect(detail.overviewChips("Customer group")).toHaveText(BETA_CONTACTS);
    expect(api.scope()).toMatchObject({ projectId: BETA.id, deploymentIds: [] });
  });

  test("editing: picking deployments of a new project derives the products", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await expect(detail.overviewCell("Customer Project")).toContainText("—");

    await detail.openEditDialog();
    await expect(detail.editDeploymentsField()).toBeDisabled();
    await detail.editProjectField().click();
    await page.getByRole("option", { name: BETA.name }).click();
    await detail.editToggleOptions(detail.editDeploymentsField(), [BETA_DEV!.name]);
    await expect(detail.editChipsOf(detail.editDeploymentProductsField())).toHaveText(productsOf(BETA_DEV!.id));
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewCell("Customer Project")).toContainText(BETA.name);
    await expect(detail.overviewChips("Deployments")).toHaveText([BETA_DEV!.name]);
  });

  test("editing: the category is sent on its own when only it changed", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, { category: "other" });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await expect(detail.overviewCell("Category")).toContainText("Other");

    await detail.openEditDialog();
    await expect(detail.editCategoryField()).toHaveText("Other");
    await detail.editCategoryField().click();
    await page.getByRole("option", { name: "Hotfix Release - Cloud", exact: true }).click();
    const [patch] = await Promise.all([
      page.waitForRequest((r) => r.method() === "PATCH" && r.url().endsWith(`/change-requests/${FAKE_CR_ID}`)),
      detail.saveEdit(),
    ]);
    expect(patch.postDataJSON()).toEqual({ category: "hotfix_release_cloud" });
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.overviewCell("Category")).toContainText("Hotfix Release - Cloud");
  });

  test("editing: shows the backend's refusal verbatim when a chosen deployment was deactivated behind the dialog", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, { projectId: ACME.id, deploymentIds: [ACME_PROD!.id] });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    await detail.openEditDialog();
    await detail.editToggleOptions(detail.editDeploymentsField(), [ACME_STG!.name]);

    api.retireDeployment(ACME_STG!.id);
    await detail.saveEdit();
    await expect(detail.editDialog().getByRole("alert")).toContainText(
      `deploymentIds: deployment ${ACME_STG!.name} is not an active deployment of the selected project`,
    );
    await expect(detail.editDialog()).toBeVisible();
    expect(api.scope().deploymentIds).toEqual([ACME_PROD!.id]);
  });

  test("the detail Overview shows a dash for each field a change request has none of", async ({ page }) => {
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(FAKE_CR_ID);
    for (const label of ["Customer Project", "Deployments", "Deployment products", "Customer group", "Category"]) {
      await expect(detail.overviewCell(label), label).toContainText("—");
    }
  });
});

//
// Customer group: the people a customer-gated change request is directed to --
// the registered contacts of its Customer Project, derived and read-only.
// Against the same fake (see its header for the exact contract): with a project
// whose contacts include someone eligible, entering Customer Approval / Customer
// Review provisions a stage for them; while it is live only Cancel is offered.
// Their answer is given in the customer portal, not here: the specs apply it
// server-side (`api.customerDecides`) and assert what the CSM page then shows
// (approve -> Scheduled / Closed, reject -> Canceled / Rollback; the contact's row
// Approved / Rejected, the others' Cancelled). With no project, a project without
// registered contacts, or none of them eligible, Request Approval is REFUSED while a
// customer box is ticked (see "Request Approval is refused when nobody can be asked"), so
// a change raised here never reaches a gate with nobody to answer. What is left is an
// OLDER change already at a gate (before that rule, its contacts gone since, or migrated
// from the previous system with no request): no stage exists, nobody is asked, and staff never record a
// customer's approval or review, so it waits at the gate with what staff always have there
// (Re-schedule, Roll back at Customer Review, Cancel change) until the customer can be
// asked. Those specs start at the gate (`openOlderChangeAt`).
//

const NO_CUSTOMER_GROUP_TEXT =
  /^No registered customer contacts are assigned to this change request's project, so no customer approvers were assigned\./;
/**
 * The note for a project that HAS registered contacts of whom none has a request waiting (only the requester, contacts
 * no longer active, or a change that reached the gate with no request at all), and what staff are left with per gate.
 */
const NOBODY_ASKED_TEXT = /^Nobody is being asked to answer at this step\./;
const CANCEL_ONLY_WAY_OUT = /Cancel change is the only way out/;
const ROLL_BACK_OR_CANCEL_WAYS_OUT = /Roll back or Cancel change are the only ways out/;

/** A change request on the Acme project: its customer group is Mia and Max. */
const ON_ACME = { projectId: ACME.id };

/**
 * A change request on a project with NO registered contacts (Gamma). Request Approval is REFUSED for it while a customer box
 * is ticked (see "Request Approval is refused when nobody can be asked"), so no flow through the page puts a customer step
 * there with nobody to ask; what is left is an OLDER change that reached a gate before that rule, one whose contacts left
 * the project afterwards, or one migrated from the previous system with no request. Such a change starts at its gate
 * (`openOlderChangeAt`), and nobody can answer for the customer there either: there is no manual path.
 */
const NO_CONTACTS = { projectId: GAMMA.id };

/** The stepper's label of each state `FakeChangeRequestApi.startAtState` takes. */
const OLDER_STATE_LABEL = { review: "Review", customer_approval: "Customer Approval", customer_review: "Customer Review" } as const;

/**
 * Opens an OLDER change request: already in `state` with nobody asked (see `FakeChangeRequestApi.startAtState`), the creator
 * signed in. Request Approval is refused for a project nobody on which can be asked, so the dead end that remains -- a change
 * already at a customer gate with nobody to answer, where only Cancel (and Re-schedule, or Roll back) are left -- starts here.
 */
async function openOlderChangeAt(
  api: FakeChangeRequestApi,
  detail: ChangeRequestDetailPage,
  state: keyof typeof OLDER_STATE_LABEL,
): Promise<void> {
  api.startAtState(state);
  await openDetail(detail);
  await expect(detail.currentStep()).toContainText(OLDER_STATE_LABEL[state]);
}

/**
 * The customer's request is pending, so only the customer moves the change on. At Customer Approval staff have Re-schedule
 * (beside Change state) and, in the menu, Cancel change; at Customer Review the menu lists Roll back DISABLED (a failed review
 * is the customer's to give too, see the action bar describes) next to an enabled Cancel change. No Close, no forward button,
 * nothing named Bypass -- not even a disabled entry.
 */
async function expectOnlyCancelActionable(detail: ChangeRequestDetailPage, gate: "approval" | "review"): Promise<void> {
  await expect(detail.answerForCustomerWording()).toHaveCount(0);
  await expect(detail.closeButton()).toHaveCount(0);
  await expect(detail.page.getByRole("button", { name: "Start implementation" })).toHaveCount(0);
  await detail.openChangeStateMenu();
  await expect(detail.menuItems()).toHaveCount(gate === "review" ? 2 : 1);
  await expect(detail.answerForCustomerWording()).toHaveCount(0);
  if (gate === "review") await expect(detail.rollbackMenuItem()).toBeDisabled();
  await expect(detail.cancelChangeMenuItem()).toBeEnabled();
  await detail.closeChangeStateMenu();
}

async function switchTo(page: import("@playwright/test").Page, api: FakeChangeRequestApi, user: FakeUser): Promise<void> {
  api.setViewer(user);
  await page.reload();
}

/**
 * The customer's answer: given in the customer portal (customers do not sign in to the
 * CSM portal), applied server-side, then the CSM page -- as `viewer`, an internal user --
 * is reloaded to show the outcome.
 */
async function customerAnswers(
  page: import("@playwright/test").Page,
  api: FakeChangeRequestApi,
  contact: FakeUser,
  decision: "approved" | "rejected",
  viewer: FakeUser = FAKE_CREATOR,
): Promise<void> {
  api.customerDecides(contact, decision);
  await switchTo(page, api, viewer);
}

/** Drives a fresh Normal CR through Peer and CAB approval (the creator
 * requests, Pat Peer and Cam Cab approve), leaving the viewer as Cam Cab. */
async function approveInternally(page: import("@playwright/test").Page, api: FakeChangeRequestApi, detail: ChangeRequestDetailPage): Promise<void> {
  await openDetail(detail);
  await detail.requestApproval();
  await expect(detail.currentStep()).toContainText("Assess");
  await switchTo(page, api, FAKE_PEER);
  await detail.approve("Pat Peer");
  await expect(detail.currentStep()).toContainText("Authorize");
  await switchTo(page, api, FAKE_CAB);
  await detail.approve("Cam Cab");
}

test.describe("change request approval flow — customer group (the project's registered contacts)", () => {
  test("Normal with Customer Approval and Customer Review on a project with registered contacts: every step shows the right state, stage rows and buttons for the creator and a non-contact, and the customer's answers (applied server-side) show up", async ({
    page,
  }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(
      page,
      "normal",
      FAKE_CREATOR,
      { customerApprovalRequired: true, customerReviewRequired: true },
      ON_ACME,
    );
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    // Customer Approval, creator: the group's stage is provisioned; Cancel only.
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    // The Overview lists the same people as the read-only Customer Group.
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    for (const member of [FAKE_CUST_ONE, FAKE_CUST_TWO]) {
      await expect(detail.approverRow(member.name, "Customer Approval")).toBeVisible();
      await expect(detail.approverStatus(member.name, "Customer Approval")).toHaveText("Requested");
      await expect(detail.approverRow(member.name, "Customer Approval")).toContainText("Customer Group");
    }
    await expect(detail.approverStatus("Pat Peer", "Peer Approval")).toHaveText("Approved");
    await expect(detail.approverStatus("Cam Cab", "CAB Approval")).toHaveText("Approved");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    await expectOnlyCancelActionable(detail, "approval");
    await expectNoManualSchedule(detail);

    // Customer Approval, non-member: sees the rows, no Approve/Reject, no manual path.
    await switchTo(page, api, FAKE_OUTSIDER);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.approverRow(FAKE_CUST_ONE.name, "Customer Approval")).toBeVisible();
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    await expectNoBypass(detail);

    // Customer Approval, a member answers in the customer portal (not in this page): the CSM
    // page, reloaded, shows Scheduled, the member's row Approved and the other contact's Cancelled.
    await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
    expect(api.state()).toBe("scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Approved");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Approval")).toHaveText("Cancelled");
    await expect(detail.approveButton()).toHaveCount(0);
    await expectNoManualSchedule(detail);

    // The engineer-driven tail up to Review.
    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    await expect(detail.sendForCustomerReviewButton()).toBeVisible();
    await expect(detail.closeButton()).toHaveCount(0);

    // Customer Review: a stage for the same group; Cancel only.
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Review")).toHaveText("Requested");
    await expect(detail.approverRow(FAKE_CUST_TWO.name, "Customer Review")).toContainText("Customer Group");
    await expect(detail.approveButton()).toHaveCount(0); // creator
    await expectOnlyCancelActionable(detail, "review");

    // Customer Review, non-member: nothing to decide.
    await switchTo(page, api, FAKE_OUTSIDER);
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);

    // Customer Review, the other member approves in the customer portal -> Closed.
    await customerAnswers(page, api, FAKE_CUST_TWO, "approved");
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Review")).toHaveText("Approved");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Review")).toHaveText("Cancelled");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0);
    await expectNoManualSchedule(detail);
  });

  test("a contact rejecting the Customer Approval (in the customer portal) cancels the change request, and the CSM page shows it", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await customerAnswers(page, api, FAKE_CUST_TWO, "rejected");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Approval")).toHaveText("Rejected");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Cancelled");
    await expect(page.locator(".MuiChip-label", { hasText: /^Canceled$/ }).first()).toBeVisible();
    expect(api.state()).toBe("canceled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0);
    // Canceled is the current stage. The customer's rejection proves the change ended at Customer Approval: Authorize and
    // before are passed, Customer Approval is rejected and the stages after it were never reached (not "history not recorded").
    await expectStages(detail, "d d d r n n n n n n c", { approval: true, review: false });
    await expect(detail.stage("Customer Approval")).toHaveText("Customer Approval, rejected by the customer");
  });

  test("a contact rejecting the Customer Review (in the customer portal) moves the change request to Rollback (terminal, no actions left), and the CSM page shows it", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");

    await customerAnswers(page, api, FAKE_CUST_ONE, "rejected");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Review")).toHaveText("Rejected");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Review")).toHaveText("Cancelled");
    expect(api.state()).toBe("rollback");
    await expect(page.locator(".MuiChip-label", { hasText: /^Rollback$/ }).first()).toBeVisible();
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0);
    // The customer's rejection of the review is why the change was rolled back: Customer Review reads rejected, Rollback is current.
    await expectStages(detail, "d d d d d d d r c n n", { approval: false, review: true });
    await expect(detail.stage("Customer Review")).toHaveText("Customer Review, rejected by the customer");
  });

  test("an older change at Customer Approval on a project with no registered contacts: no customer stage, the Approval tab explains why, and nobody can record the approval: Re-schedule and Cancel change are what is left", async ({
    page,
  }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();
    // ...and the note says plainly what is left: Cancel change is the only way out of Customer Approval.
    await expect(page.getByText(CANCEL_ONLY_WAY_OUT)).toBeVisible();
    await expect(page.getByRole("cell", { name: "Customer Approval", exact: true })).toHaveCount(0);
    // Nobody was asked and nobody can answer for the customer: Re-schedule beside Change state, whose menu holds Cancel change.
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });
    await expectNoBypass(detail);

    // The backend says the same in words, asked or not, and nothing moved (no work note either).
    expect(await patchStateFromPage(page, "scheduled")).toEqual({ status: 400, message: customerAnswerRefusal("scheduled") });
    expect(api.state()).toBe("customer_approval");
    expect(api.journal()).toEqual([]);
    await expect(detail.currentStep()).toContainText("Customer Approval");
  });

  test("an older change at Customer Review on a project with no registered contacts shows the helper, Roll back and Cancel change are on offer, there is no Close and no Bypass", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();
    await expect(page.getByText(ROLL_BACK_OR_CANCEL_WAYS_OUT)).toBeVisible();
    await expect(page.getByRole("cell", { name: "Customer Review", exact: true })).toHaveCount(0);
    await expect(detail.closeButton()).toHaveCount(0);
    // Nobody is asked, so Roll back is enabled (nothing is pending to hold it back); there is no Close to be had.
    await expectActionBar(detail, { primary: null, reschedule: false, menu: ["Roll back", "Cancel change"] });
    await detail.openChangeStateMenu();
    await expect(detail.rollbackMenuItem()).toBeEnabled();
    await detail.closeChangeStateMenu();
    await expectNoBypass(detail);

    expect(await patchStateFromPage(page, "closed")).toEqual({ status: 400, message: customerAnswerRefusal("closed") });
    expect(api.state()).toBe("customer_review");
    expect(api.journal()).toEqual([]);
  });

  test("an older change at Customer Approval on a project without registered contacts has no stage and the helper; the project cannot be swapped for one that has contacts (it is fixed once approval was requested), so the change waits until a contact registers on THAT project", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();
    await expect(detail.overviewChips("Customer group")).toHaveCount(0);
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });

    // The Edit dialog shows the project read-only, with why: it cannot be swapped for one that has contacts.
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editDialog().getByText(PROJECT_FROZEN_REASON)).toBeVisible();
    await expect(detail.editDialog().getByText(ACME.name)).toHaveCount(0);
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();
    await expect(detail.editDialog()).toHaveCount(0);
    // ... and the backend refuses it with its own readable 400 when it is asked anyway.
    expect(await patchFromPage(page, { projectId: ACME.id })).toEqual({ status: 400, message: projectFrozenMessage("customer_approval") });
    expect(api.scope().projectId).toBe(GAMMA.id);
    expect(api.stages().filter((st) => st.stage === "Customer Approval")).toEqual([]);

    // A contact registers on the SAME project: the next write that touches the state or the project asks them.
    api.setProjectContacts(GAMMA.id, [FAKE_CUST_ONE]);
    expect(await patchFromPage(page, { projectId: GAMMA.id }), "the stored project resent is an accepted no-op").toMatchObject({ status: 200 });
    await page.reload();
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expectOnlyCancelActionable(detail, "approval");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
  });

  test("the Customer Project cannot be changed while the customer is asked: the dialog shows it read-only with the reason, the backend refuses a swap in words, and the contacts asked are still the same", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editDialog().getByText(PROJECT_FROZEN_REASON)).toBeVisible();
    await expect(detail.editChipsOf(detail.editCustomerGroupField())).toHaveText(ACME_CONTACTS);
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();

    // The swap, sent anyway: a readable 400 and nothing moved -- the stage still asks Acme's contacts.
    expect(await patchFromPage(page, { projectId: BETA.id })).toEqual({ status: 400, message: projectFrozenMessage("customer_approval") });
    await page.reload();
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    expect(api.stages().filter((st) => st.stage === "Customer Approval").map((st) => st.status)).toEqual(["REQUESTED"]);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    expect(api.scope().projectId).toBe(ACME.id);

    // Customer B's contact has no pending request here; customer A's contact answers (in the customer portal).
    expect(() => api.customerDecides(FAKE_BETA_CONTACT, "approved")).toThrow(/no pending customer approval/);
    expect(api.state()).toBe("customer_approval");
    await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
  });

  test("isolation: a change request of customer A is put only to customer A's contacts, never to customer B's, who cannot answer it", async ({ page }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_OUTSIDER);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.overviewChips("Customer group")).toHaveText(ACME_CONTACTS);
    await expect(page.getByText(FAKE_BETA_CONTACT.name, { exact: true })).toHaveCount(0);
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
    expect(api.stages().find((st) => st.stage === "Customer Approval")?.approvers.map((a) => a.name)).toEqual(ACME_CONTACTS);
    // Customer B's contact has no pending row on this change request: nothing moves.
    expect(() => api.customerDecides(FAKE_BETA_CONTACT, "approved")).toThrow(/no pending customer approval/);
    expect(api.state()).toBe("customer_approval");
  });

  test("an older change at Customer Approval on a project whose only contact is the creator has no stage: nobody is asked, the note says so (not that no contacts are registered), and still no way to record the approval", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, { projectId: GAMMA.id });
    api.setProjectContacts(GAMMA.id, [FAKE_CREATOR]);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval");
    await expect(page.getByRole("cell", { name: "Customer Approval", exact: true })).toHaveCount(0);
    // The project HAS a registered contact, so "no registered contacts" would be false: the note says nobody has a
    // request waiting, and that Cancel change is the only way out of Customer Approval.
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expect(page.getByText(NOBODY_ASKED_TEXT)).toBeVisible();
    await expect(page.getByText(CANCEL_ONLY_WAY_OUT)).toBeVisible();
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });
    await expectNoBypass(detail);
  });

  test("an older change at Customer Review with nobody eligible (the requester is the project's only contact) shows the same note, naming Roll back or Cancel change as the ways out", async ({ page }) => {
    test.setTimeout(120_000);
    // Deactivated contacts leave the same empty set (the backend asks only active contacts, and the fake only the
    // non-creators), which is all the page can see: the unit tests of the page cover that wording separately.
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, { projectId: GAMMA.id });
    api.setProjectContacts(GAMMA.id, [FAKE_CREATOR]);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_review");
    await expect(page.getByText(NOBODY_ASKED_TEXT)).toBeVisible();
    await expect(page.getByText(ROLL_BACK_OR_CANCEL_WAYS_OUT)).toBeVisible();
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expect(detail.closeButton()).toHaveCount(0);
    await expectActionBar(detail, { primary: null, reschedule: false, menu: ["Roll back", "Cancel change"] });
    await expectNoBypass(detail);
  });

  test("a customer request that is waiting on somebody shows no note at either gate", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await expect(page.getByText(NOBODY_ASKED_TEXT)).toHaveCount(0);
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
  });
});

// ---------------------------------------------------------------------------
// Request Approval is REFUSED when a customer box is ticked and nobody on the Customer Project can be asked (no registered
// contact other than the requester). Such a change would reach Customer Approval / Customer Review with nobody to answer,
// and with no staff action that answers for the customer (no Bypass) it could only be cancelled (or rolled back from
// Review). This extends the rule that refuses Request Approval with a box ticked and no Customer Project. The page says it up
// front where it knows (the project has no registered contact at all: the button is disabled with the reason); where it cannot
// tell (the requester is the only contact) the request goes out and the backend's own words show in the error banner. Ticking a
// box on after New is refused the same way, and its words show in the Edit dialog. Against the in-browser fake of the backend.
// ---------------------------------------------------------------------------

test.describe("change request approval flow — Request Approval is refused when nobody can be asked (mocked backend)", () => {
  /** The disabled Request Approval's focusable wrapper, found by the reason it carries (`Request Approval: <reason>`). */
  const blocked = (page: Page): Locator => page.getByLabel(`Request Approval: ${REQUEST_APPROVAL_NEEDS_CONTACT_REASON}`);

  const TICKED: Array<[type: "normal" | "standard", boxes: string, flags: { customerApprovalRequired?: boolean; customerReviewRequired?: boolean }]> = [
    ["normal", "Customer Approval", { customerApprovalRequired: true }],
    ["normal", "Customer Review", { customerReviewRequired: true }],
    ["standard", "both boxes", { customerApprovalRequired: true, customerReviewRequired: true }],
  ];
  for (const [type, boxes, flags] of TICKED) {
    test(`${type} change, ${boxes} ticked, a project with no registered contact: Request Approval is disabled with the reason, the API refuses it in the backend's words, and nothing moves`, async ({ page }) => {
      test.setTimeout(120_000);
      const api = await installFakeChangeRequestApi(page, type, FAKE_CREATOR, flags, NO_CONTACTS);
      const detail = new ChangeRequestDetailPage(page);
      await openDetail(detail);
      await expect(detail.currentStep()).toContainText("New");

      await expect(blocked(page)).toBeVisible();
      await expect(blocked(page).getByRole("button", { name: "Request Approval" })).toBeDisabled();
      await blocked(page).hover(); // the reason is the button's tooltip
      await expect(page.getByRole("tooltip")).toContainText(REQUEST_APPROVAL_NEEDS_CONTACT_REASON);
      // Cancel change is still there behind the menu: a change that cannot be sent is not stuck.
      await detail.openChangeStateMenu();
      await expect(detail.menuItems()).toHaveText(["Cancel change"]);
      await detail.closeChangeStateMenu();

      // The backend is the authority: asked anyway it says why in words, and moves nothing.
      const words = nobodyToAskMessage(!!flags.customerApprovalRequired, !!flags.customerReviewRequired);
      expect(await patchStateFromPage(page, "assess")).toEqual({ status: 400, message: words });
      expect(api.state()).toBe("new");
      expect(api.stages()).toEqual([]);
      expect(api.journal()).toEqual([]);
      await page.reload();
      await expect(detail.currentStep()).toContainText("New");
      await expect(blocked(page)).toBeVisible();
    });
  }

  test("with no customer box ticked there is nobody to ask for: Request Approval is enabled on a project with no registered contact, and goes through", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(blocked(page)).toHaveCount(0);
    await expect(detail.requestApprovalButton()).toBeEnabled();
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    expect(api.state()).toBe("assess");
  });

  test("a contact registers on the project: the block lifts, Request Approval goes through, and the customer is asked at the gate", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(blocked(page)).toBeVisible();

    api.setProjectContacts(GAMMA.id, [FAKE_CUST_ONE]);
    await page.reload();
    await expect(blocked(page)).toHaveCount(0);
    await expect(detail.requestApprovalButton()).toBeEnabled();
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    await switchTo(page, api, FAKE_PEER);
    await detail.approve("Pat Peer");
    await switchTo(page, api, FAKE_CAB);
    await detail.approve("Cam Cab");
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expect(page.getByText(NOBODY_ASKED_TEXT)).toHaveCount(0);
  });

  test("unticking the box in the Edit dialog (allowed in New) lifts the block: the change then goes through with nobody to ask for", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(blocked(page)).toBeVisible();

    await detail.openEditDialog();
    await expect(detail.editCustomerApprovalCheckbox()).toBeChecked();
    await expect(detail.editCustomerApprovalCheckbox()).toBeEnabled();
    await detail.editCustomerApprovalCheckbox().uncheck();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    expect(api.flags()).toEqual({ customerApprovalRequired: false, customerReviewRequired: false });
    await expect(blocked(page)).toHaveCount(0);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
  });

  test("a project whose only contact is the requester: the page cannot tell, so the button is enabled; the backend's refusal shows in the error banner, the change stays in New, and a second contact lets it through", async ({ page }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    api.setProjectContacts(GAMMA.id, [FAKE_CREATOR]);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    // The project HAS a registered contact: nothing to say up front.
    await expect(blocked(page)).toHaveCount(0);
    await expect(detail.requestApprovalButton()).toBeEnabled();

    await detail.requestApproval();
    // The backend's own words, verbatim, in the same place every other refused transition shows.
    await expect(page.getByRole("alert").filter({ hasText: nobodyToAskMessage(true, false) })).toBeVisible();
    expect(api.state()).toBe("new");
    expect(api.stages()).toEqual([]);
    await expect(detail.currentStep()).toContainText("New");
    await expect(detail.requestApprovalButton()).toBeEnabled();

    // A second contact registers: the same button now goes through.
    api.setProjectContacts(GAMMA.id, [FAKE_CREATOR, FAKE_CUST_ONE]);
    await page.reload();
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    expect(api.state()).toBe("assess");
  });

  test("after New, ticking a box on for a project nobody on which can be asked is refused too: the dialog shows the backend's words and the box is not saved", async ({ page }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    // No box ticked, so Request Approval is open on a project with no contact: the change leaves New...
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");

    // ...and the unticked Customer Approval box can still be added until its gate, which the page does not pre-empt:
    // the project is set, and whether anybody on it can be asked is the backend's call.
    await detail.openEditDialog();
    await expect(detail.editCustomerApprovalCheckbox()).toBeEnabled();
    await detail.editCustomerApprovalCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog().getByRole("alert")).toContainText(nobodyToAskMessage(true, false));
    await expect(detail.editDialog()).toBeVisible();
    expect(api.flags()).toEqual({ customerApprovalRequired: false, customerReviewRequired: false });
    // The refusal is the same at the API, naming whichever box is turned on (or both).
    expect(await patchFromPage(page, { customerReviewRequired: true })).toEqual({ status: 400, message: nobodyToAskMessage(false, true) });
    expect(await patchFromPage(page, { customerApprovalRequired: true, customerReviewRequired: true })).toEqual({ status: 400, message: nobodyToAskMessage(true, true) });
    // Once a contact registers, the same save is accepted.
    api.setProjectContacts(GAMMA.id, [FAKE_CUST_ONE]);
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    expect(api.flags()).toEqual({ customerApprovalRequired: true, customerReviewRequired: false });
  });
});

// ---------------------------------------------------------------------------
// Roll back -- the failed-review off-ramp. Offered, next to the forward move and
// Cancel change, from Review and Customer Review only; destructive (menu-only),
// and it needs a stated reason, which is posted as an internal note before the PATCH.
// Rollback is final: the stepper's Rollback stage is the current one and no
// actions are left. Runs against the in-browser fake of the backend contract.
// ---------------------------------------------------------------------------

/** Roll back is not offered: either no overflow menu at all, or none of its entries is "Roll back". */
async function expectNoRollbackOffered(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.page.getByRole("button", { name: "Roll back", exact: true })).toHaveCount(0);
  if ((await detail.changeStateButton().count()) === 0) return;
  await detail.changeStateButton().click();
  await expect(detail.page.getByRole("menuitem").first()).toBeVisible();
  await expect(detail.rollbackMenuItem()).toHaveCount(0);
  await detail.page.keyboard.press("Escape");
}

/** The Rollback stage is the current one on the stepper (no other is), the state chip says so, nothing is awaited. */
async function expectRolledBack(page: import("@playwright/test").Page, detail: ChangeRequestDetailPage, api: FakeChangeRequestApi): Promise<void> {
  await expect(detail.reasonDialog()).toHaveCount(0);
  await expect(detail.currentStep()).toHaveCount(1);
  await expect(detail.currentStep()).toContainText("Rollback");
  // The stepper: Rollback is the current stage; a rolled-back change got past Review, and never reached Closed or Canceled.
  await expect(detail.stage("Rollback")).toHaveText("Rollback, current");
  for (const passed of ["New", "Assess", "Authorize", "Scheduled", "Implement", "Review"]) {
    await expect(detail.stage(passed), passed).toHaveText(`${passed}, done`);
  }
  await expect(detail.stage("Closed")).toHaveText("Closed, not taken");
  await expect(detail.stage("Canceled")).toHaveText("Canceled, not taken");
  await expect(page.locator(".MuiChip-label", { hasText: /^Rollback$/ }).first()).toBeVisible();
  await expect(detail.blockingReason()).toHaveCount(0);
  await expect(detail.changeStateButton()).toHaveCount(0);
  await expect(detail.approveButton()).toHaveCount(0);
  expect(api.state()).toBe("rollback");
  // Nobody is left pending on a rolled-back change.
  for (const st of api.stages()) {
    for (const a of st.approvers) expect(a.status, `${st.stage}/${a.name}`).not.toBe("REQUESTED");
  }
}

/** Opens Roll back from the overflow menu, shows reason is required, then confirms with `reason`. */
async function rollBackWithReason(page: import("@playwright/test").Page, detail: ChangeRequestDetailPage, reason: string): Promise<void> {
  await detail.changeStateButton().click();
  await detail.rollbackMenuItem().click();
  const dialog = detail.reasonDialog();
  await expect(dialog.getByRole("heading", { name: "Roll back this change?" })).toBeVisible();
  // A reason is required: the confirm action stays disabled until one is typed.
  await expect(dialog.getByRole("button", { name: "Roll back", exact: true })).toBeDisabled();
  await dialog.getByLabel("Reason").fill(reason);
  await expect(dialog.getByRole("button", { name: "Roll back", exact: true })).toBeEnabled();
  await dialog.getByRole("button", { name: "Roll back", exact: true }).click();
  await expect(detail.currentStep()).toContainText("Rollback");
}

//
// Assignment group: each approver row's Assignment group is a link that opens the
// group (ServiceNow's group page) and lists its members; for the customer stages
// there is no group, so it lists the project's registered contacts. Against the
// same fake (`GET /groups/{id}`, `assignmentGroup` on every internal stage).
//

/** The group-page requests the fake has served so far. */
const groupRequests = (api: FakeChangeRequestApi): string[] => api.requests().filter((r) => r.startsWith("GET /groups/"));

test.describe("change request approval flow — opening an Assignment group", () => {
  test("Peer row: the group opens with its details and members; Escape closes it and returns focus to the link", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    await expect(detail.approverStage("Pat Peer")).toHaveText("Peer Approval");
    await expect(detail.approverRow("Pat Peer", "Peer Approval")).toContainText(FAKE_PEER_GROUP.name);

    // Nothing is fetched until the link is used.
    expect(groupRequests(api)).toEqual([]);

    // Reachable and operable from the keyboard alone.
    const link = detail.groupLink("Pat Peer", FAKE_PEER_GROUP.name, "Peer Approval");
    await link.focus();
    await page.keyboard.press("Enter");

    const dialog = detail.groupDialog(FAKE_PEER_GROUP.name);
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("Mona Manager")).toBeVisible();
    await expect(dialog.getByRole("link", { name: "example-corp-abt@example.com" })).toBeVisible();
    await expect(dialog.getByText("Builds and supports the Example Corp account.")).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Group Members (2)" })).toBeVisible();
    const members = dialog.getByRole("listitem");
    await expect(members).toHaveCount(2);
    await expect(members.nth(0)).toContainText(FAKE_PEER.name);
    await expect(members.nth(0)).toContainText(FAKE_PEER.email);
    await expect(members.nth(0)).toContainText("Lead");
    await expect(members.nth(1)).toContainText(FAKE_PEER_COLLEAGUE.name);
    await expect(members.nth(1)).not.toContainText("Lead");
    expect(groupRequests(api)).toEqual([`GET /groups/${FAKE_PEER_GROUP.id}`]);

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(link).toBeFocused();
  });

  test("CAB row: opens the CAB Approval group's own members (not the peer group's)", async ({ page }) => {
    test.setTimeout(90_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await switchTo(page, api, FAKE_PEER);
    await detail.approve("Pat Peer");
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.approverStage("Cam Cab")).toHaveText("CAB Approval");

    await detail.groupLink("Cam Cab", FAKE_CAB_GROUP.name, "CAB Approval").click();
    const dialog = detail.groupDialog(FAKE_CAB_GROUP.name);
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Group Members (3)" })).toBeVisible();
    const members = dialog.getByRole("listitem");
    await expect(members).toHaveText([
      new RegExp(`${FAKE_CAB.name}.*${FAKE_CAB.email}`),
      new RegExp(`${FAKE_CAB_COLLEAGUE.name}.*${FAKE_CAB_COLLEAGUE.email}`),
      new RegExp(`^${FAKE_CAB_NO_EMAIL.name}$`), // no email on file: just the name
    ]);
    // A group with no description, email or manager shows only its members.
    await expect(dialog.getByText("Manager", { exact: true })).toHaveCount(0);
    await expect(dialog.getByText("Group email", { exact: true })).toHaveCount(0);
    await expect(dialog.getByText("Description", { exact: true })).toHaveCount(0);
    expect(groupRequests(api)).toEqual([`GET /groups/${FAKE_CAB_GROUP.id}`]);

    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(dialog).toBeHidden();
    // The Peer row's own group is still its own: a different request, a different page.
    await detail.groupLink("Pat Peer", FAKE_PEER_GROUP.name, "Peer Approval").click();
    await expect(detail.groupDialog(FAKE_PEER_GROUP.name).getByRole("heading", { name: "Group Members (2)" })).toBeVisible();
    expect(groupRequests(api)).toEqual([`GET /groups/${FAKE_CAB_GROUP.id}`, `GET /groups/${FAKE_PEER_GROUP.id}`]);
  });

  test("Emergency: the CAB Approval row opens the CAB group (its one stage is in the existing group)", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.approverStage(FAKE_CAB.name)).toHaveText("CAB Approval");

    await detail.groupLink(FAKE_CAB.name, FAKE_CAB_GROUP.name, "CAB Approval").click();
    const dialog = detail.groupDialog(FAKE_CAB_GROUP.name);
    await expect(dialog.getByRole("heading", { name: "Group Members (3)" })).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveText([
      new RegExp(FAKE_CAB.name),
      new RegExp(FAKE_CAB_COLLEAGUE.name),
      new RegExp(FAKE_CAB_NO_EMAIL.name),
    ]);
    expect(groupRequests(api)).toEqual([`GET /groups/${FAKE_CAB_GROUP.id}`]);
  });

  test("an OLDER Emergency change's ECAB Approval row still opens its (unused) ECAB group", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR);
    api.startOlderEmergencyAtAuthorize();
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expect(detail.approverStage(FAKE_ECAB.name)).toHaveText("ECAB Approval");

    await detail.groupLink(FAKE_ECAB.name, FAKE_ECAB_GROUP.name, "ECAB Approval").click();
    const dialog = detail.groupDialog(FAKE_ECAB_GROUP.name);
    await expect(dialog.getByText("Emergency Change Advisory Board.")).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Group Members (2)" })).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveText([
      new RegExp(`${FAKE_ECAB.name}.*Lead`),
      new RegExp(FAKE_ECAB_COLLEAGUE.name),
    ]);
    expect(groupRequests(api)).toEqual([`GET /groups/${FAKE_ECAB_GROUP.id}`]);
  });

  test("Customer Approval row: opens the Customer Group listing the project's registered contacts, with no group request", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");

    await detail.groupLink(FAKE_CUST_ONE.name, "Customer Group", "Customer Approval").click();
    const dialog = detail.groupDialog("Customer Group");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "Group Members (2)" })).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveText([
      new RegExp(`${FAKE_CUST_ONE.name}.*${FAKE_CUST_ONE.email}`),
      new RegExp(`${FAKE_CUST_TWO.name}.*${FAKE_CUST_TWO.email}`),
    ]);
    // These are the contacts the page already has; there is no group to fetch.
    expect(groupRequests(api)).toEqual([]);

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("a group that cannot be loaded shows an error with Try again, leaves the approvals table alone, and recovers", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await expect(detail.approverStage("Pat Peer")).toHaveText("Peer Approval");

    api.failGroups(500);
    await detail.groupLink("Pat Peer", FAKE_PEER_GROUP.name, "Peer Approval").click();
    const dialog = detail.groupDialog(FAKE_PEER_GROUP.name);
    await expect(dialog.getByText("Could not load this group's members.")).toBeVisible();
    await expect(dialog.getByRole("listitem")).toHaveCount(0);

    api.failGroups(null);
    await dialog.getByRole("button", { name: "Try again" }).click();
    await expect(dialog.getByRole("heading", { name: "Group Members (2)" })).toBeVisible();
    await expect(dialog.getByText("Could not load this group's members.")).toHaveCount(0);

    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(dialog).toBeHidden();
    await expect(detail.approverRow("Pat Peer", "Peer Approval")).toBeVisible();
  });

  test("opening and closing a group leaves Approve / Reject working", async ({ page }) => {
    test.setTimeout(60_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await detail.requestApproval();
    await switchTo(page, api, FAKE_PEER);
    await expect(detail.approveButton("Pat Peer")).toBeEnabled();
    await expect(detail.rejectButton("Pat Peer")).toBeEnabled();

    await detail.groupLink("Pat Peer", FAKE_PEER_GROUP.name, "Peer Approval").click();
    await expect(detail.groupDialog(FAKE_PEER_GROUP.name)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(detail.groupDialog(FAKE_PEER_GROUP.name)).toBeHidden();

    await expect(detail.approveButton("Pat Peer")).toBeEnabled();
    await expect(detail.rejectButton("Pat Peer")).toBeEnabled();
    await detail.approve("Pat Peer");
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.approverStatus("Pat Peer", "Peer Approval")).toHaveText("Approved");
  });
});

test.describe("change request approval flow — Roll back", () => {
  for (const review of [true, false]) {
    test(`Normal, customer review ${review ? "on" : "off"}: New -> ... -> Review -> Roll back (reason required), state shown after every step`, async ({
      page,
    }) => {
      test.setTimeout(180_000);
      const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: review }, review ? ON_ACME : {});
      const detail = new ChangeRequestDetailPage(page);

      // Roll back is never on offer on the way to Review.
      await openDetail(detail);
      await expect(detail.currentStep()).toContainText("New");
      await expectNoRollbackOffered(detail);
      await detail.requestApproval();
      await expect(detail.currentStep()).toContainText("Assess");
      await expectNoRollbackOffered(detail);
      await switchTo(page, api, FAKE_PEER);
      await detail.approve("Pat Peer");
      await expect(detail.currentStep()).toContainText("Authorize");
      await expectNoRollbackOffered(detail);
      await switchTo(page, api, FAKE_CAB);
      await detail.approve("Cam Cab");
      await switchTo(page, api, FAKE_CREATOR);
      await expect(detail.currentStep()).toContainText("Scheduled");
      await expectNoRollbackOffered(detail);
      await page.getByRole("button", { name: "Start implementation" }).click();
      await expect(detail.currentStep()).toContainText("Implement");
      await expectNoRollbackOffered(detail);
      await page.getByRole("button", { name: "Mark implemented" }).click();

      // Review: the forward move is the primary button, Roll back sits in the menu with Cancel change.
      await expect(detail.currentStep()).toContainText("Review");
      if (review) {
        await expect(detail.sendForCustomerReviewButton()).toBeVisible();
        await expect(detail.closeButton()).toHaveCount(0);
      } else {
        await expect(detail.closeButton()).toBeVisible();
        await expect(detail.sendForCustomerReviewButton()).toHaveCount(0);
      }
      await detail.changeStateButton().click();
      await expect(detail.page.getByRole("menuitem")).toHaveText(["Roll back", "Cancel change"]);
      await detail.page.keyboard.press("Escape");

      // Backing out of the dialog leaves the change untouched.
      await detail.changeStateButton().click();
      await detail.rollbackMenuItem().click();
      await detail.reasonDialog().getByRole("button", { name: "Go back", exact: true }).click();
      await expect(detail.reasonDialog()).toHaveCount(0);
      await expect(detail.currentStep()).toContainText("Review");
      expect(api.state()).toBe("review");

      await rollBackWithReason(page, detail, "Post-deployment smoke test failed.");
      await expectRolledBack(page, detail, api);
      // Customer Review is on the line when ticked, off it when not. The project's contacts would have been asked on entering it,
      // and there is no stage for them: it was not entered, so it reads "not taken" (an older change whose project had nobody to
      // ask leaves no such proof: see "Rollback: the stage turns current").
      if (review) await expect(detail.stage("Customer Review")).toHaveText("Customer Review, not taken");
      else await expect(detail.stage("Customer Review")).toHaveCount(0);
      // The reason was recorded as an internal note before the state moved.
      expect(api.journal()).toContainEqual({ kind: "comment", text: "Post-deployment smoke test failed." });
      const calls = api.requests();
      expect(calls.indexOf(`POST /change-requests/${FAKE_CR_ID}/comments`)).toBeGreaterThan(-1);
      expect(calls.indexOf(`POST /change-requests/${FAKE_CR_ID}/comments`)).toBeLessThan(
        calls.lastIndexOf(`PATCH /change-requests/${FAKE_CR_ID}`),
      );
      const patchBodies = api.requestBodies().filter((b) => b.request.startsWith("PATCH"));
      expect(patchBodies[patchBodies.length - 1]?.body).toEqual({ state: "rollback" });

      // Still rolled back after a reload: a terminal state with no way out.
      await page.reload();
      await expectRolledBack(page, detail, api);
    });
  }

  test("an older Normal change at Customer Review with nobody asked: Roll back is not held back (nothing is pending), and takes a reason", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_review");

    // Customer Review without a group: no button at all (the customer's review is theirs to give, so no Close);
    // the menu lists Roll back, then Cancel change.
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expect(detail.closeButton()).toHaveCount(0);
    await detail.openChangeStateMenu();
    await expect(detail.menuItems()).toHaveText(["Roll back", "Cancel change"]);
    await detail.closeChangeStateMenu();

    await rollBackWithReason(page, detail, "The customer rejected the result.");
    await expectRolledBack(page, detail, api);
    // Rolled back from Customer Review, which had no contacts to ask and so left no stage: the line cannot say it was
    // entered, and does not claim it was skipped.
    await expectStages(detail, "d d d d d d d u c n n", { approval: false, review: true });
    expect(api.journal()).toContainEqual({ kind: "comment", text: "The customer rejected the result." });
  });

  test("Customer Review with a customer group: Roll back is listed disabled, with why, while the group's review is pending", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(
      page,
      "normal",
      FAKE_CREATOR,
      { customerReviewRequired: true },
      ON_ACME,
    );
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);

    await switchTo(page, api, FAKE_CREATOR);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await page.getByRole("button", { name: "Mark implemented" }).click();
    // Review still offers Roll back (the internal review can fail).
    await detail.changeStateButton().click();
    await expect(detail.rollbackMenuItem()).toBeVisible();
    await detail.page.keyboard.press("Escape");
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
    // Nothing is clickable but Cancel: Roll back is there to be seen (the failed review is the customer's to give), disabled.
    await detail.openChangeStateMenu();
    await expect(detail.rollbackMenuItem()).toBeDisabled();
    await expect(detail.rollbackMenuItem().getByText(PENDING_REVIEW_ROLLBACK_REASON)).toBeVisible();
    await expect(detail.rollbackMenuItem()).toHaveAccessibleName(`Roll back: ${PENDING_REVIEW_ROLLBACK_REASON}`);
    await detail.rollbackMenuItem().focus();
    await page.keyboard.press("Enter");
    await expect(detail.reasonDialog()).toHaveCount(0);
    await detail.closeChangeStateMenu();
    expect(await patchStateFromPage(page, "rollback")).toEqual({ status: 400, message: customerStageManualRefusal("rollback") });
    expect(api.state()).toBe("customer_review");
    await expectOnlyCancelActionable(detail, "review");
  });

  test("Standard: Roll back is offered from Review too, and nowhere before it", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "standard", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expectNoRollbackOffered(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Scheduled");
    await expectNoRollbackOffered(detail);
    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");
    await expectNoRollbackOffered(detail);
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    await rollBackWithReason(page, detail, "Backout after failed verification.");
    await expectRolledBack(page, detail, api);
  });
});

// ---------------------------------------------------------------------------
// An approval is only actionable while the change is in its stage's state. Reported
// bug: an internal reviewer kept Approve / Reject on the Review stage of a change that
// was already Closed (and while it waited at Customer Review for the customer). Deciding
// Review changes no state -- a human moves the change on -- and nothing cancelled the
// Review stage's other approvers when it left Review. The fake backend does what the
// real one does now: every PATCH / decision ends by cancelling the REQUESTED rows of the
// stages the change has left (all of them once it is closed / canceled / rollback), and
// canDecide is false on a REQUESTED row of a stage the change has left.
// ---------------------------------------------------------------------------

/** Cancel change from the overflow menu, with the reason the dialog insists on. */
async function cancelChangeWithReason(page: Page, detail: ChangeRequestDetailPage, reason: string): Promise<void> {
  await detail.changeStateButton().click();
  await detail.cancelChangeMenuItem().click();
  const dialog = detail.reasonDialog();
  await dialog.getByLabel("Reason").fill(reason);
  await dialog.getByRole("button", { name: "Cancel change", exact: true }).click();
  await expect(detail.reasonDialog()).toHaveCount(0);
}

/** Drives a fresh Normal CR (Peer, CAB, implementation) to Review, leaving the creator signed in. */
async function driveToReview(
  page: Page,
  api: FakeChangeRequestApi,
  detail: ChangeRequestDetailPage,
): Promise<void> {
  await approveInternally(page, api, detail);
  await switchTo(page, api, FAKE_CREATOR);
  await page.getByRole("button", { name: "Start implementation" }).click();
  await expect(detail.currentStep()).toContainText("Implement");
  await page.getByRole("button", { name: "Mark implemented" }).click();
  await expect(detail.currentStep()).toContainText("Review");
}

/**
 * Nobody can approve or reject anything: no controls render at all. The page has to be there first: right after a
 * `switchTo()` reload the controls are absent only because nothing has rendered yet, and a test that ends on such
 * an assertion finishes while the page is still loading (its `/users/me` and approvals requests in flight).
 */
async function expectNoDecisionControls(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.lifecycleStepper()).toBeVisible();
  await detail.page.waitForLoadState("networkidle");
  await expect(detail.approveButton()).toHaveCount(0);
  await expect(detail.rejectButton()).toHaveCount(0);
}

/** Every approver row of every stage, flattened -- nothing may still be REQUESTED on a finished change. */
const requestedRows = (api: FakeChangeRequestApi): string[] =>
  api.stages().flatMap((st) => st.approvers.filter((a) => a.status === "REQUESTED").map((a) => `${st.stage}/${a.name}`));

const REVIEWERS = [FAKE_PEER, FAKE_PEER_COLLEAGUE] as const;

test.describe("change request approval flow — a Review approver's controls follow the change request's state", () => {
  test("Review: the assigned group's members can decide their own row, nobody else can", async ({ page }) => {
    test.setTimeout(150_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);

    // Provisioned for the assigned group's internal members, the creator excluded.
    expect(api.stages().map((st) => st.stage)).toEqual(["Peer Approval", "CAB Approval", "Review"]);
    expect(api.stages()[2]!.approvers).toEqual([
      { name: FAKE_PEER.name, status: "REQUESTED" },
      { name: FAKE_PEER_COLLEAGUE.name, status: "REQUESTED" },
    ]);
    await expectNoDecisionControls(detail); // the creator

    for (const reviewer of REVIEWERS) {
      await switchTo(page, api, reviewer);
      await expect(detail.currentStep()).toContainText("Review");
      await expect(detail.approverStatus(reviewer.name, "Review")).toHaveText("Requested");
      await expect(detail.approveButton(reviewer.name, "Review")).toBeEnabled();
      await expect(detail.rejectButton(reviewer.name, "Review")).toBeEnabled();
      await expect(detail.approveButton()).toHaveCount(1); // their own row only
    }
    // A CAB member decided long ago and is not in the assigned group.
    await switchTo(page, api, FAKE_CAB);
    await expectNoDecisionControls(detail);
  });

  for (const moveOn of ["customer_review", "closed", "rollback", "canceled"] as const) {
    test(`Review -> ${moveOn}: the reviewers can no longer approve or reject`, async ({ page }) => {
      test.setTimeout(240_000);
      const api = await installFakeChangeRequestApi(
        page,
        "normal",
        FAKE_CREATOR,
        { customerReviewRequired: moveOn === "customer_review" },
        moveOn === "customer_review" ? ON_ACME : {},
      );
      const detail = new ChangeRequestDetailPage(page);
      await driveToReview(page, api, detail);

      // In Review the first reviewer can decide.
      await switchTo(page, api, FAKE_PEER);
      await expect(detail.approveButton("Pat Peer", "Review")).toBeEnabled();

      // The creator moves the change on.
      await switchTo(page, api, FAKE_CREATOR);
      if (moveOn === "customer_review") {
        await detail.sendForCustomerReviewButton().click();
        await expect(detail.currentStep()).toContainText("Customer Review");
      } else if (moveOn === "closed") {
        await detail.closeButton().click();
        await expect(detail.currentStep()).toContainText("Closed");
      } else if (moveOn === "rollback") {
        await rollBackWithReason(page, detail, "Smoke test failed.");
        await expectRolledBack(page, detail, api);
      } else {
        await cancelChangeWithReason(page, detail, "No longer needed.");
        await expect(page.locator(".MuiChip-label", { hasText: /^Canceled$/ }).first()).toBeVisible();
      }
      expect(api.state()).toBe(moveOn);
      // The Review rows were cancelled on the way -- every one of them.
      expect(api.stages()[2]!.approvers.map((a) => a.status)).toEqual(["CANCELLED", "CANCELLED"]);
      if (moveOn !== "customer_review") expect(requestedRows(api)).toEqual([]);

      // Both reviewers now see their rows as Cancelled, with no Approve / Reject (also after a reload).
      for (const reviewer of REVIEWERS) {
        await switchTo(page, api, reviewer);
        await expect(detail.approverStatus(reviewer.name, "Review")).toHaveText("Cancelled");
        await expectNoDecisionControls(detail);
      }
    });
  }

  test("Review -> Customer Review -> the customer approves (in the customer portal) -> Closed: nothing is requested at the end", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
    expect(requestedRows(api)).toEqual(["Customer Review/Mia Member", "Customer Review/Max Member"]);

    // The reviewers have nothing to decide while the customer answers...
    await switchTo(page, api, FAKE_PEER);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");
    await expectNoDecisionControls(detail);
    // ...the customer does, in the customer portal; the CSM page shows the outcome.
    await customerAnswers(page, api, FAKE_CUST_ONE, "approved", FAKE_PEER);
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
    expect(requestedRows(api)).toEqual([]);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Review")).toHaveText("Approved");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Review")).toHaveText("Cancelled");
    for (const user of [FAKE_PEER, FAKE_PEER_COLLEAGUE]) {
      await switchTo(page, api, user);
      await expectNoDecisionControls(detail);
    }
  });

  test("a Review decision records the answer and leaves the change in Review; the other member's row is cancelled", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);

    await switchTo(page, api, FAKE_PEER);
    await detail.approve("Pat Peer", "Review");
    await expect(detail.approverStatus("Pat Peer", "Review")).toHaveText("Approved");
    await expect(detail.approverStatus("Quinn Peer", "Review")).toHaveText("Cancelled");
    expect(api.state()).toBe("review"); // a human moves it on
    await expectNoDecisionControls(detail);
    await switchTo(page, api, FAKE_PEER_COLLEAGUE);
    await expectNoDecisionControls(detail);
    await switchTo(page, api, FAKE_CREATOR);
    await detail.closeButton().click();
    await expect(detail.currentStep()).toContainText("Closed");
    expect(requestedRows(api)).toEqual([]);
  });

  test("a legacy row left REQUESTED after the change moved on reads canDecide=false: the controls are disabled, with the reason", async ({ page }) => {
    test.setTimeout(180_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    const decisions = (): string[] => api.requests().filter((r) => r.endsWith("/approvals/decision"));
    const decisionsBefore = decisions().length; // the Peer and CAB approvals that got it here
    // The change moved on without the sweep (a row written before it existed).
    api.setState("closed");

    await switchTo(page, api, FAKE_PEER);
    await expect(detail.currentStep()).toContainText("Closed");
    await expect(detail.approverStatus("Pat Peer", "Review")).toHaveText("Requested");
    await expect(detail.approveButton("Pat Peer", "Review")).toBeDisabled();
    await expect(detail.rejectButton("Pat Peer", "Review")).toBeDisabled();
    await expect(page.getByLabel(/you aren't able to approve or reject this stage/i)).toBeVisible();
    expect(decisions()).toHaveLength(decisionsBefore); // nothing was submitted from the disabled controls
  });
});

// ---------------------------------------------------------------------------
// Re-schedule and the customer's proposed time -- the diagram's Time Change loop, in the previous system's own
// mechanism. "authorize" is the wire name of the loop, but the change NEVER leaves Customer Approval and never goes
// back through CAB: the change itself has not changed.
//
//  - Re-schedule (an outlined button, Customer Approval only): a dialog (current window prefilled, at least one end
//    must change, optional reason); the customer is asked to approve the new time again, in a fresh request.
//  - A customer's PROPOSED time waits in Customer Approval, the planned window untouched, until WSO2 answers: a banner
//    shows it beside the planned time, with "Accept proposed time" (the change goes straight to Scheduled, no CAB, no new
//    customer request) and "Propose a different time" (the customer is asked again; keeping the current time declines).
//
// Runs against the in-browser fake of the backend contract.
// ---------------------------------------------------------------------------

/** Re-schedule is not offered: no such button and no such menu entry. */
async function expectNoRescheduleOffered(detail: ChangeRequestDetailPage): Promise<void> {
  await expect(detail.rescheduleButton()).toHaveCount(0);
  if ((await detail.changeStateButton().count()) === 0) return;
  await detail.changeStateButton().click();
  await expect(detail.page.getByRole("menuitem").first()).toBeVisible();
  await expect(detail.page.getByRole("menuitem", { name: /re-schedule|authorize/i })).toHaveCount(0);
  await detail.page.keyboard.press("Escape");
}

// Typed in the signed-in user's time zone; the PATCH carries UTC, so the specs assert
// the stored window moved to the week after (any zone offset keeps it on 2030-03-0[78]).
const NEXT_WEEK_START = { month: 3, day: 8, year: 2030, hour12: 12, minute: 0, pm: true };
const NEXT_WEEK_END = { month: 3, day: 8, year: 2030, hour12: 2, minute: 0, pm: true };
const ORIGINAL_WINDOW = { start: "2030-03-01 09:00:00", end: "2030-03-01 11:00:00" };
const MOVED_TO_NEXT_WEEK = /^2030-03-0[78] \d{2}:\d{2}:00$/;

/** The stage names the fake holds, in order (a Re-schedule must never add a CAB one). */
const stageNames = (api: FakeChangeRequestApi): string[] => api.stages().map((st) => st.stage);

test.describe("change request approval flow — Re-schedule", () => {
  test("Normal with a customer group: Customer Approval -> Re-schedule -> still Customer Approval, the customer asked again (no Authorize, no CAB) -> a member approves (in the customer portal) -> Scheduled, state shown after every step", async ({
    page,
  }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(
      page,
      "normal",
      FAKE_CREATOR,
      { customerApprovalRequired: true },
      ON_ACME,
    );
    const detail = new ChangeRequestDetailPage(page);

    // Re-schedule is never on offer before Customer Approval.
    await openDetail(detail);
    await expectNoRescheduleOffered(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    await expectNoRescheduleOffered(detail);
    await switchTo(page, api, FAKE_PEER);
    await detail.approve("Pat Peer");
    await expect(detail.currentStep()).toContainText("Authorize");
    await expectNoRescheduleOffered(detail);
    await switchTo(page, api, FAKE_CAB);
    await detail.approve("Cam Cab");

    // Customer Approval: the customer group is asked; Re-schedule and Cancel are the creator's actions.
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.rescheduleButton()).toBeVisible();
    await expectOnlyCancelActionable(detail, "approval");
    const stagesBefore = stageNames(api);

    // The dialog starts on the current window, says the customer is asked (no CAB), and will not submit without a change.
    await detail.rescheduleButton().click();
    await expect(detail.rescheduleDialog()).toBeVisible();
    await expect(detail.rescheduleDialog().getByText(/The customer is asked to approve it\. No further internal approval is needed/)).toBeVisible();
    await expect(detail.rescheduleSubmit()).toBeDisabled();
    await expect(detail.rescheduleDialog().getByText("Change the planned start or end to re-schedule.")).toBeVisible();
    await detail.fillRescheduleWindow("Planned start", NEXT_WEEK_START);
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await expect(detail.rescheduleSubmit()).toBeEnabled();
    await detail.rescheduleDialog().getByLabel("Reason (optional)").fill("Customer freeze next week.");
    await detail.rescheduleSubmit().click();

    // Still Customer Approval, waiting on the customer: the old request is a record, the fresh one is live, no CAB was opened.
    await expect(detail.rescheduleDialog()).toHaveCount(0);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    expect(api.state()).toBe("customer_approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.rescheduleButton()).toBeVisible();
    await expectNoBypass(detail);
    expect(stageNames(api)).toEqual([...stagesBefore, "Customer Approval"]);
    expect(api.stages()[2].approvers.map((a) => a.status)).toEqual(["CANCELLED", "CANCELLED"]);
    expect(api.stages()[3].approvers.map((a) => a.status)).toEqual(["REQUESTED", "REQUESTED"]);
    expect(api.planned().start).toMatch(MOVED_TO_NEXT_WEEK);
    expect(api.planned().end).toMatch(MOVED_TO_NEXT_WEEK);
    expect(api.journal()).toContainEqual({ kind: "comment", text: "Customer freeze next week." });
    const patches = api.requestBodies().filter((b) => b.request.startsWith("PATCH"));
    expect(patches[patches.length - 1]?.body).toEqual({
      state: "authorize",
      plannedStartOn: api.planned().start,
      plannedEndOn: api.planned().end,
    });
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval").nth(1)).toHaveText("Requested");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval").first()).toHaveText("Cancelled");
    // Nobody at WSO2 has anything to approve: there is no CAB stage to decide.
    await expect(detail.approveButton()).toHaveCount(0);

    // The customer answers in the customer portal; the CSM page shows Scheduled.
    await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
    expect(api.state()).toBe("scheduled");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expectNoRescheduleOffered(detail);
  });

  test("an older Normal change at Customer Approval with nobody to ask: Re-schedule is the outlined button beside Change state, but it is REFUSED in the words Request Approval uses, writes nothing and can be tried again; an unchanged window is blocked in the dialog", async ({
    page,
  }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval");
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });
    await expect(detail.rescheduleButton()).toBeVisible();

    // No change -> the submit stays disabled.
    await detail.rescheduleButton().click();
    await expect(detail.rescheduleSubmit()).toBeDisabled();

    // Nobody can be asked: the backend's refusal is the one Request Approval gives, shown verbatim; nothing was written.
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await expect(detail.rescheduleSubmit()).toBeEnabled();
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog().getByRole("alert")).toHaveText(nobodyToAskMessage(true, false));
    expect(api.state()).toBe("customer_approval");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    expect(api.stages().map((st) => st.stage)).toEqual(["Peer Approval", "CAB Approval"]);
    expect(api.journal()).toEqual([]);
    // Repeatable: the same refusal, the same nothing.
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog().getByRole("alert")).toHaveText(nobodyToAskMessage(true, false));
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    await detail.rescheduleDialog().getByRole("button", { name: "Close", exact: true }).click();
    await expect(detail.rescheduleDialog()).toHaveCount(0);

    // The backend's other 400 is shown too: the change moved on behind the dialog's back.
    await detail.rescheduleButton().click();
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    api.setState("scheduled");
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog().getByRole("alert")).toContainText('state "authorize" cannot be set manually');
    await expect(detail.rescheduleDialog().getByRole("alert")).toContainText(
      "it can only be set by hand to re-schedule a change from customer_approval",
    );
    expect(api.state()).toBe("scheduled");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
  });

  test("a contact registers on the project: the same older change can then be re-scheduled, and the customer is asked in a fresh request", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval");
    api.setProjectContacts(GAMMA.id, [FAKE_CUST_ONE]);
    await page.reload();
    await detail.rescheduleButton().click();
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog()).toHaveCount(0);
    expect(api.state()).toBe("customer_approval");
    expect(api.stages().map((st) => st.stage)).toEqual(["Peer Approval", "CAB Approval", "Customer Approval"]);
    expect(api.stages()[2].approvers.map((a) => a.status)).toEqual(["REQUESTED"]);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
  });

  test("Emergency (an OLDER row sitting in Customer Approval, its approval stage still named ECAB): Re-schedule stays in Customer Approval too: no CAB, no Authorize", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "emergency", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    // The shape an earlier version left: the ECAB stage settled, the change waiting on the customer (nothing creates this now).
    api.startAtState("customer_approval");
    api.syncCustomers(); // the customer's request, as the change got it when it reached the gate
    await openDetail(detail);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.approverStage(FAKE_ECAB.name)).toHaveText("ECAB Approval");

    await detail.rescheduleButton().click();
    await expect(detail.rescheduleDialog().getByText(/CAB/)).toHaveCount(0);
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog()).toHaveCount(0);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    expect(api.state()).toBe("customer_approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    // The customer's superseded request stays as a record before the fresh one; no CAB stage was added.
    expect(stageNames(api)).toEqual(["ECAB Approval", "Customer Approval", "Customer Approval"]);
    await expect(detail.approveButton()).toHaveCount(0);
  });

  test("Standard with a customer group: Re-schedule stays in Customer Approval and asks the customer again", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(
      page,
      "standard",
      FAKE_CREATOR,
      { customerApprovalRequired: true },
      ON_ACME,
    );
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expectNoRescheduleOffered(detail);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.rescheduleButton()).toBeVisible();

    await detail.rescheduleButton().click();
    await expect(detail.rescheduleDialog().getByText(/The customer is asked to approve it/)).toBeVisible();
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog()).toHaveCount(0);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    expect(api.state()).toBe("customer_approval");
    expect(api.planned().end).toMatch(MOVED_TO_NEXT_WEEK);
    expect(api.planned().start).toBe(ORIGINAL_WINDOW.start);
    expect(api.stages().map((s) => s.stage)).toEqual(["Customer Approval", "Customer Approval"]);
    expect(api.stages()[0].approvers.map((a) => a.status)).toEqual(["CANCELLED", "CANCELLED"]);
    expect(api.stages()[1].approvers.map((a) => a.status)).toEqual(["REQUESTED", "REQUESTED"]);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.rescheduleButton()).toBeVisible();
  });
});

// A time the customer proposed: the previous system's own `customer_updated_on` / confirmation pair. The proposal is made in the customer
// portal (`api.customerProposes`); the CSM page shows it and WSO2 answers it.
const PROPOSED_START = "2030-03-08T09:00:00Z"; // the planned window is 2030-03-01 09:00 - 11:00 (2 hours)
/** The planned window a page opened on the original plan shows (what an Accept names as the plan it replaces). */
const SHOWN_WINDOW = { expectedPlannedStartOn: ORIGINAL_WINDOW.start, expectedPlannedEndOn: ORIGINAL_WINDOW.end };
const PROPOSED_WINDOW_TEXT = /Mar 8, 2030, \d{1,2}:\d{2} [AP]M to Mar 8, 2030, \d{1,2}:\d{2} [AP]M/;

/** Drives a Normal change (the customer's approval on) to Customer Approval and has Mia propose a time; the creator is signed in. */
async function proposalWaiting(page: Page, api: FakeChangeRequestApi, detail: ChangeRequestDetailPage, proposerKnown = true): Promise<void> {
  await driveToCustomerApproval(page, api, detail);
  if (proposerKnown) api.customerProposes(FAKE_CUST_ONE, PROPOSED_START);
  else api.seedProposal({ startOn: PROPOSED_START, proposerKnown: false });
  await switchTo(page, api, FAKE_CREATOR);
  await expect(detail.proposalBanner()).toBeVisible();
}

/**
 * A picture of the page for review (previews only, never committed): written to E2E_SHOT_DIR as `<name>-<scheme>.png`, 1440 wide,
 * in the colour scheme E2E_SHOT_SCHEME names (light, or dark by default). A no-op without the directory.
 */
async function proposalPicture(page: Page, name: string): Promise<void> {
  const dir = process.env.E2E_SHOT_DIR?.trim();
  if (!dir) return;
  const scheme = process.env.E2E_SHOT_SCHEME === "light" ? "light" : "dark";
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.emulateMedia({ colorScheme: scheme });
  fs.mkdirSync(dir, { recursive: true });
  await page.waitForTimeout(600); // a dialog fades in, a theme repaints: wait it out
  await page.screenshot({ path: path.join(dir, `${name}-${scheme}.png`) });
}

test.describe("change request approval flow — a customer's proposed time (mocked backend)", () => {
  test.describe.configure({ timeout: 240_000 });

  test("the proposal waits in Customer Approval, planned window untouched: the banner shows both windows and who proposed it, the header says the change waits for WSO2, and nothing else was written", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    const stagesBefore = JSON.stringify(api.stages());
    await expect(detail.proposalBanner()).toHaveCount(0); // nothing proposed yet

    api.customerProposes(FAKE_CUST_ONE, PROPOSED_START);
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.proposalBanner()).toBeVisible();
    await expect(detail.currentStep()).toContainText("Customer Approval");
    expect(api.state()).toBe("customer_approval");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW); // not overwritten before WSO2 agrees
    expect(JSON.stringify(api.stages())).toBe(stagesBefore); // no stage, no approver row touched
    expect(api.proposal()).toEqual({ customerUpdatedOn: PROPOSED_START, confirmation: null });
    // The header waits for WSO2, not for the customer.
    await expect(detail.proposalWaitingReason()).toHaveText("Waiting for WSO2 to respond to the customer's proposed time");
    await expect(detail.customerProposalWaitingReason()).toBeVisible();
    await expect(detail.neutralProposalWaitingReason()).toHaveCount(0);
    await expect(detail.customerProposalBanner()).toBeVisible(); // the proposer is on record, so the banner says the customer proposed it
    await expect(detail.neutralProposalBanner()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveCount(0);
    // The banner: planned beside proposed, the proposer, Accept the one primary action.
    const banner = detail.proposalBanner();
    await expect(banner).toContainText("Planned now");
    await expect(banner).toContainText("Proposed by the customer");
    await expect(banner).toContainText(PROPOSED_WINDOW_TEXT);
    await expect(banner).toContainText("Same length as the planned window (2 hours)");
    await expect(banner).toContainText("Proposed by Mia Member (mia.member@acme.example)");
    await proposalPicture(page, "01-banner-proposer-known");
    await expect(detail.acceptProposedTimeButton()).toHaveClass(/MuiButton-contained/);
    await expect(detail.proposeDifferentTimeButton()).toHaveClass(/MuiButton-outlined/);
    // The bar's own outlined action is the counter now: no Re-schedule, no bypass, Cancel is all the menu holds.
    await expect(detail.rescheduleButton()).toHaveCount(0);
    await expect(detail.page.getByRole("button", { name: "Propose a different time" })).toHaveCount(2); // the bar's and the banner's
    await expectOnlyCancelActionable(detail, "approval");
    // The customer's own request is still live: only they can approve the current time.
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
  });

  test("ACCEPT: one confirmation, the proposal becomes the planned window and the change is Scheduled; no CAB, no new customer request, and nothing is stamped as the customer's approval", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    const stageNamesBefore = stageNames(api);

    await detail.acceptProposedTimeButton().click();
    await expect(detail.acceptDialog()).toBeVisible();
    await expect(detail.acceptDialog()).toContainText("The change will be scheduled for");
    await expect(detail.acceptDialog()).toContainText("The customer sees that you accepted it and is not asked again. No CAB approval is needed.");
    await expect(detail.acceptDialog().getByRole("checkbox")).toHaveCount(0); // there is never a confirmation to tick
    await proposalPicture(page, "02-accept-dialog-proposer-known");
    expect(api.state()).toBe("customer_approval"); // nothing is sent before the engineer confirms
    await detail.acceptDialogConfirm().click();

    await expect(detail.acceptDialog()).toHaveCount(0);
    await expect(detail.currentStep()).toContainText("Scheduled");
    expect(api.state()).toBe("scheduled");
    // Exactly the Accept contract: the answer, the proposal it accepts and the window the page showed; never a `state`.
    const patches = api.requestBodies().filter((b) => b.request.startsWith("PATCH"));
    expect(patches[patches.length - 1]?.body).toEqual({
      confirmCustomerUpdatedDate: "agree",
      expectedCustomerUpdatedOn: PROPOSED_START,
      expectedPlannedStartOn: ORIGINAL_WINDOW.start,
      expectedPlannedEndOn: ORIGINAL_WINDOW.end,
    });
    // The window is the proposal with the planned length (2 hours) kept; the answer is Agree.
    expect(api.planned()).toEqual({ start: "2030-03-08 09:00:00", end: "2030-03-08 11:00:00" });
    expect(api.proposal()).toEqual({ customerUpdatedOn: PROPOSED_START, confirmation: "agree" });
    // No CAB and no second customer request: the stages are what they were, and nobody is asked any more.
    expect(stageNames(api)).toEqual(stageNamesBefore);
    expect(api.stages().flatMap((st) => st.approvers).filter((a) => a.status === "REQUESTED")).toEqual([]);
    // The customer's approval is not stamped (no staff action records it): the cell says what happened, not a misleading No.
    expect(api.customerApproved()).toBe(false);
    await expect(detail.overviewCell("Customer approved")).toContainText("Proposed time accepted");
    await expect(detail.overviewCell("Customer approved")).not.toHaveText(/\bNo\b/);
    await proposalPicture(page, "06-after-accept-scheduled");
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.proposalWaitingReason()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveCount(0);
    await expectNoBypass(detail);
    // SRE details show WSO2's answer.
    await detail.page.getByRole("tab", { name: "Plan" }).click();
    await expect(detail.overviewCell("WSO2 answer to the customer's time")).toContainText("Agree");
  });

  test("COUNTER: a different time asks the customer again (no CAB), answers the proposal Disagree and the banner is gone; the loop repeats with the customer's next proposal", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    const stageNamesBefore = stageNames(api);

    await detail.proposeDifferentTimeButton().click();
    await expect(detail.counterDialog()).toBeVisible();
    await expect(detail.counterDialog()).toContainText("No CAB approval is needed.");
    // Prefilled with the PLANNED window; the submit says what it does.
    await expect(detail.counterSubmit("Decline proposed time")).toBeEnabled();
    await proposalPicture(page, "03-counter-dialog-decline");
    await detail.fillRescheduleWindow("Planned start", NEXT_WEEK_START, detail.counterDialog());
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END, detail.counterDialog());
    await detail.counterSubmit("Propose this time").click();
    await expect(detail.counterDialog()).toHaveCount(0);

    expect(api.state()).toBe("customer_approval");
    expect(api.proposal()).toEqual({ customerUpdatedOn: PROPOSED_START, confirmation: "disagree" });
    expect(api.planned().start).toMatch(MOVED_TO_NEXT_WEEK);
    // The customer is asked again in a fresh request, nothing else opened.
    expect(stageNames(api)).toEqual([...stageNamesBefore, "Customer Approval"]);
    expect(api.stages()[stageNamesBefore.length].approvers.map((a) => a.status)).toEqual(["REQUESTED", "REQUESTED"]);
    const patches = api.requestBodies().filter((b) => b.request.startsWith("PATCH"));
    expect(patches[patches.length - 1]?.body).toMatchObject({ state: "authorize", expectedCustomerUpdatedOn: PROPOSED_START });
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.rescheduleButton()).toBeVisible();

    // The customer proposes again (their next proposal clears the standing answer): the banner is back.
    api.customerProposes(FAKE_CUST_TWO, "2030-03-20T10:00:00Z");
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.proposalBanner()).toBeVisible();
    await expect(detail.proposalBanner()).toContainText("Proposed by Max Member (max.member@acme.example)");
    expect(api.proposal().confirmation).toBeNull();
  });

  test("DECLINE (keep the current time): only the answer is written; the customer keeps their live request and nothing new is opened", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    const stagesBefore = JSON.stringify(api.stages());

    await detail.proposeDifferentTimeButton().click();
    await expect(detail.counterDialog()).toContainText("The current time stays, so the proposal is declined.");
    await detail.counterSubmit("Decline proposed time").click();
    await expect(detail.counterDialog()).toHaveCount(0);
    expect(api.proposal()).toEqual({ customerUpdatedOn: PROPOSED_START, confirmation: "disagree" });
    expect(api.state()).toBe("customer_approval");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    expect(JSON.stringify(api.stages())).toBe(stagesBefore);
    const patches = api.requestBodies().filter((b) => b.request.startsWith("PATCH"));
    expect(patches[patches.length - 1]?.body).toEqual({
      state: "authorize",
      expectedCustomerUpdatedOn: PROPOSED_START,
      expectedPlannedStartOn: ORIGINAL_WINDOW.start,
      expectedPlannedEndOn: ORIGINAL_WINDOW.end,
    });
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
  });

  test("the very time the customer proposed is not a counter: the dialog says to use Accept, and the API refuses it in words", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    await detail.proposeDifferentTimeButton().click();
    // 09:00 - 11:00 UTC on 8 March, typed in the viewer's own time zone as the pickers show it.
    const zone = await detail.page.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone);
    expect(zone).toBeTruthy();
    expect(await patchFromPage(page, { state: "authorize", plannedStartOn: "2030-03-08 09:00:00", plannedEndOn: "2030-03-08 11:00:00", expectedCustomerUpdatedOn: PROPOSED_START })).toEqual({
      status: 400,
      message: COUNTER_IS_THE_PROPOSAL,
    });
    expect(api.proposal().confirmation).toBeNull();
  });

  test("nobody is recorded as having proposed the stored time (a date WSO2 users write too, or one left over from an earlier round): the banner says so, Accept is disabled with the reason and refused by the API, and Propose a different time is a plain Re-schedule that writes no answer", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail, false);
    const stagesBefore = api.stages().map((st) => st.stage);
    // Nobody is on record, so nothing says the customer proposed it and nothing waits for an answer: a stored time, an info banner,
    // and the header still says the customer is being asked.
    await expect(detail.neutralProposalBanner()).toBeVisible();
    await expect(detail.customerProposalBanner()).toHaveCount(0);
    await expect(detail.neutralProposalBanner()).toContainText("A time is stored (");
    await expect(detail.neutralProposalBanner()).toContainText(") but nobody is recorded as having proposed it.");
    await expect(detail.neutralProposalBanner()).toContainText("Stored time");
    await expect(detail.neutralProposalBanner()).not.toContainText("Proposed by");
    await expect(detail.neutralProposalBanner()).not.toContainText("The customer proposed");
    await expect(detail.proposalWaitingReason()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await proposalPicture(page, "04-banner-proposer-not-recorded");

    // Accept is disabled, with the backend's own reason on a focusable wrapper; there is nothing to confirm and no dialog to open.
    await expect(detail.acceptProposedTimeButton()).toBeDisabled();
    const reason = `${ACCEPT_PROPOSER_NOT_RECORDED.charAt(0).toUpperCase()}${ACCEPT_PROPOSER_NOT_RECORDED.slice(1)}.`;
    await expect(detail.proposalBanner().getByLabel(`Accept proposed time: ${reason}`)).toHaveAttribute("tabindex", "0");
    await expect(detail.proposeDifferentTimeButton()).toBeEnabled();
    // The API refuses an Accept sent anyway, with its code in the body, and writes nothing.
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: PROPOSED_START, ...SHOWN_WINDOW })).toEqual({
      status: 409,
      message: ACCEPT_PROPOSER_NOT_RECORDED,
    });
    // ... and a request with no window is not a decline of a time nobody proposed: refused, never a silent Disagree.
    expect(await patchFromPage(page, { state: "authorize" })).toEqual({ status: 400, message: NO_RECORDED_PROPOSAL_TO_DECLINE });
    expect(api.state()).toBe("customer_approval");
    expect(api.proposal()).toEqual({ customerUpdatedOn: PROPOSED_START, confirmation: null });

    // Propose a different time: a plain Re-schedule that names the stored time. No decline, the window must change.
    await detail.proposeDifferentTimeButton().click();
    await expect(detail.counterDialog()).toContainText("A time is stored (");
    await expect(detail.counterDialog()).toContainText("There is no proposal to decline.");
    await expect(detail.counterDialog()).not.toContainText("The customer proposed");
    await expect(detail.counterSubmit("Propose this time")).toBeDisabled();
    await expect(detail.counterDialog().getByRole("button", { name: "Decline proposed time" })).toHaveCount(0);
    await detail.fillRescheduleWindow("Planned start", NEXT_WEEK_START, detail.counterDialog());
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END, detail.counterDialog());
    await detail.counterSubmit("Propose this time").click();
    await expect(detail.counterDialog()).toHaveCount(0);
    // The customer is asked again (a fresh request, no CAB) and NO answer was written against a date nobody proposed.
    expect(api.state()).toBe("customer_approval");
    expect(api.proposal().confirmation).toBeNull();
    // The window was typed in the signed-in user's zone and the PATCH carries UTC, so assert the day, never the hour.
    expect(api.planned().start).toMatch(MOVED_TO_NEXT_WEEK);
    expect(api.planned().end).toMatch(MOVED_TO_NEXT_WEEK);
    expect(api.stages().map((st) => st.stage)).toEqual([...stagesBefore, "Customer Approval"]);
  });

  test("a date that is not a proposal waiting for WSO2 shows no banner and no actions on it: a stale date equal to the plan, an answered one, an internal approval still being asked (an unknown approval group), a change in Authorize", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);

    // 1. A date left over from an earlier cycle: it equals the planned start, so there is nothing to answer.
    api.seedProposal({ startOn: "2030-03-01T09:00:00Z" });
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.rescheduleButton()).toBeVisible();
    // ... and a plain Re-schedule is not an answer to it: the API would have said so had a proposal been waiting.
    expect(await patchFromPage(page, { state: "authorize", expectedCustomerUpdatedOn: "2030-03-01T09:00:00Z" })).toEqual({ status: 409, message: PROPOSAL_NO_LONGER_WAITING });

    // 2. WSO2 already answered it (a standing Disagree), whoever wrote the date.
    api.seedProposal({ startOn: PROPOSED_START, confirmation: "disagree" });
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.proposalBanner()).toHaveCount(0);

    // 3. An approval is still being asked of somebody that is not the customer: an inconsistent row, or a group we have no name for.
    api.seedProposal({ startOn: PROPOSED_START });
    api.addInternalRequestedRow();
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: PROPOSED_START, ...SHOWN_WINDOW })).toEqual({ status: 409, message: NO_PROPOSAL_WAITING });

    // 4. A change in Authorize (an older row whose fresh CAB stage is live) with the same date on it: no banner, no actions.
    api.setState("authorize");
    await page.reload();
    await expect(detail.currentStep()).toContainText("Authorize");
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.rescheduleButton()).toHaveCount(0);
    await expect(detail.proposeDifferentTimeButton()).toHaveCount(0);
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: PROPOSED_START, ...SHOWN_WINDOW })).toEqual({
      status: 409,
      message: acceptNotInCustomerApproval("authorize"),
    });
    expect(api.state()).toBe("authorize");
  });

  test("Accept is disabled with its reason on hold, once the proposed time has passed, or when its window would end after the year 2100; the API refuses each in words", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);

    api.setOnHold(true);
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.acceptProposedTimeButton()).toBeDisabled();
    await expect(detail.page.getByLabel("Accept proposed time: This change request is on hold. Take it off hold first.")).toBeVisible();
    await expect(detail.proposeDifferentTimeButton()).toBeEnabled();
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: PROPOSED_START, ...SHOWN_WINDOW })).toEqual({ status: 400, message: ON_HOLD_MESSAGE });
    api.setOnHold(false);

    // A proposal that has since passed (seeded in the past: the customer portal refuses to make one).
    api.seedProposal({ startOn: "2020-01-01T09:00:00Z", proposerKnown: true });
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.acceptProposedTimeButton()).toBeDisabled();
    await expect(detail.page.getByLabel("Accept proposed time: The proposed time has passed. Propose a different time.")).toBeVisible();
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: "2020-01-01T09:00:00Z", ...SHOWN_WINDOW })).toEqual({
      status: 409,
      message: proposalPassedMessage("2020-01-01T09:00:00Z"),
    });
    expect(api.state()).toBe("customer_approval");
    expect(api.proposal().confirmation).toBeNull();

    // A date left far ahead (the previous system writes the column too; the customer portal refuses to make one): the planned 2-hour
    // length from it would end after the year 2100, a window no other path may write. Held back in the backend's words, and the
    // last window that fits is accepted.
    api.seedProposal({ startOn: "2100-12-31T22:30:00Z", proposerKnown: true });
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.acceptProposedTimeButton()).toBeDisabled();
    await expect(
      detail.page.getByLabel(/^Accept proposed time: The time the customer proposed \(2100-12-31T22:30:00Z\) is too far ahead to be accepted/),
    ).toBeVisible();
    await expect(detail.proposeDifferentTimeButton()).toBeEnabled();
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: "2100-12-31T22:30:00Z", ...SHOWN_WINDOW })).toEqual({
      status: 409,
      message: acceptTooFarAheadMessage("2100-12-31T22:30:00Z", "2101-01-01T00:30:00Z"),
    });
    expect(api.state()).toBe("customer_approval");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    api.seedProposal({ startOn: "2100-12-31T21:30:00Z", proposerKnown: true });
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.acceptProposedTimeButton()).toBeEnabled();
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: "2100-12-31T21:30:00Z", ...SHOWN_WINDOW })).toMatchObject({ status: 200 });
    expect(api.state()).toBe("scheduled");
  });

  test("a time the customer re-proposed behind an open confirmation is refused in the backend's words, the dialog keeps what the engineer was shown, and nothing is accepted", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    await detail.acceptProposedTimeButton().click();
    api.customerProposes(FAKE_CUST_TWO, "2030-03-20T10:00:00Z"); // behind the open dialog
    await detail.acceptDialogConfirm().click();
    await expect(detail.acceptDialog().getByRole("alert")).toHaveText(proposalChangedMessage("2030-03-20T10:00:00Z"));
    expect(api.state()).toBe("customer_approval");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    expect(api.proposal().confirmation).toBeNull();
    // That refusal names no code, but the proposal the dialog showed is not the one the page holds now once the change is read
    // again: asking again would be refused again, so Accept is held back and Close is the way on.
    await expect(detail.acceptDialogConfirm()).toBeDisabled();
    await detail.acceptDialog().getByRole("button", { name: "Close", exact: true }).click();
    await page.reload();
    await expect(detail.proposalBanner()).toContainText("Proposed by Max Member");
  });

  test("a window re-scheduled behind an open Accept confirmation: the refusal names its code, the dialog closes, the page says why with focus on the notice, shows the new window, and Accept then goes through", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    await detail.acceptProposedTimeButton().click();
    api.moveWindow("2030-03-02 09:00:00", "2030-03-02 11:00:00"); // behind the open dialog
    await detail.acceptDialogConfirm().click();
    await expect(detail.acceptDialog()).toHaveCount(0);
    const notice = detail.staleAnswerNotice();
    const moved = windowChangedMessage("2030-03-02T09:00:00Z to 2030-03-02T11:00:00Z");
    await expect(notice).toContainText(`${moved.charAt(0).toUpperCase()}${moved.slice(1)}.`); // the notice states it as a sentence
    await expect(notice).toBeFocused();
    // Nothing was accepted, and the page read the change again: the banner shows the planned window as it is now.
    expect(api.state()).toBe("customer_approval");
    expect(api.proposal().confirmation).toBeNull();
    await expect(detail.proposalBanner()).toContainText(/Mar 2, 2030, \d{1,2}:\d{2} [AP]M to Mar 2, 2030, \d{1,2}:\d{2} [AP]M/);
    // On the current state it goes through, and the notice goes with the next dialog.
    await detail.acceptProposedTimeButton().click();
    await expect(notice).toHaveCount(0);
    await detail.acceptDialogConfirm().click();
    await expect(detail.acceptDialog()).toHaveCount(0);
    expect(api.state()).toBe("scheduled");
  });

  test("a window re-scheduled behind an open counter: the dialog closes with no note posted, and opened again the reason is recorded exactly once", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    const reason = "Freeze that week.";
    const fillCounter = async (): Promise<void> => {
      await detail.fillRescheduleWindow("Planned start", NEXT_WEEK_START, detail.counterDialog());
      await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END, detail.counterDialog());
      await detail.counterDialog().getByLabel("Reason (optional)").fill(reason);
    };
    await detail.proposeDifferentTimeButton().click();
    await fillCounter();
    api.moveWindow("2030-03-02 09:00:00", "2030-03-02 11:00:00"); // behind the open dialog
    await detail.counterSubmit("Propose this time").click();
    await expect(detail.counterDialog()).toHaveCount(0);
    await expect(detail.staleAnswerNotice()).toBeFocused();
    expect(api.journal()).toEqual([]); // the refused attempt left no note behind
    expect(api.proposal().confirmation).toBeNull();

    await detail.proposeDifferentTimeButton().click();
    await expect(detail.counterDialog().getByLabel("Reason (optional)")).toHaveValue("");
    await fillCounter();
    await detail.counterSubmit("Propose this time").click();
    await expect(detail.counterDialog()).toHaveCount(0);
    expect(api.proposal().confirmation).toBe("disagree");
    expect(api.journal()).toEqual([{ kind: "comment", text: reason }]); // once, after the change went through
  });

  test("a plain Re-schedule opened before the customer proposed never answers a proposal it did not see: the API says so, in words", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    await detail.rescheduleButton().click();
    api.customerProposes(FAKE_CUST_ONE, PROPOSED_START); // behind the open dialog
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog().getByRole("alert")).toHaveText(customerProposedWhileOpenMessage(PROPOSED_START));
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    expect(api.proposal().confirmation).toBeNull();
  });

  test("every refusal of the two answers, in the backend's words, and none writes anything: the shape of Accept, the state, a stale window, a stale proposal", async ({ page }) => {
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await proposalWaiting(page, api, detail);
    const stagesBefore = JSON.stringify(api.stages());
    const refusal = (status: number, message: string) => ({ status, message });
    const shown = { expectedPlannedStartOn: ORIGINAL_WINDOW.start, expectedPlannedEndOn: ORIGINAL_WINDOW.end };
    const accept = (extra: Record<string, unknown>) =>
      patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: PROPOSED_START, ...shown, ...extra });

    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "disagree", expectedCustomerUpdatedOn: PROPOSED_START, ...shown })).toEqual(refusal(400, ACCEPT_ONLY_AGREE));
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", ...shown })).toEqual(refusal(400, ACCEPT_NEEDS_EXPECTED));
    // The window the page showed is required too: an Accept never lands on a plan its reader did not see.
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: PROPOSED_START })).toEqual(
      refusal(400, ACCEPT_NEEDS_EXPECTED_WINDOW),
    );
    expect(await accept({ subject: "something else" })).toEqual(refusal(400, ACCEPT_CANNOT_COMBINE));
    expect(await accept({ expectedPlannedStartOn: "2030-03-02 09:00:00" })).toEqual(
      refusal(409, windowChangedMessage("2030-03-01T09:00:00Z to 2030-03-01T11:00:00Z")),
    );
    expect(await patchFromPage(page, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: "2030-03-09T09:00:00Z", ...shown })).toEqual(
      refusal(409, proposalChangedMessage(PROPOSED_START)),
    );
    // A Re-schedule that names a proposal that is not the waiting one, and one that names none while one waits.
    expect(await patchFromPage(page, { state: "authorize", expectedCustomerUpdatedOn: "2030-03-09T09:00:00Z" })).toEqual(
      refusal(409, proposalChangedMessage(PROPOSED_START)),
    );
    expect(await patchFromPage(page, { state: "authorize", plannedEndOn: "2030-03-01 12:00:00" })).toEqual(
      refusal(409, customerProposedWhileOpenMessage(PROPOSED_START)),
    );
    // Neither answer is a manual `scheduled`: that stays refused whoever asks, a waiting proposal included.
    expect(await patchStateFromPage(page, "scheduled")).toEqual(refusal(400, customerAnswerRefusal("scheduled")));

    // Nothing was written by any of it.
    expect(api.state()).toBe("customer_approval");
    expect(api.planned()).toEqual(ORIGINAL_WINDOW);
    expect(api.proposal()).toEqual({ customerUpdatedOn: PROPOSED_START, confirmation: null });
    expect(JSON.stringify(api.stages())).toBe(stagesBefore);
    expect(ACCEPT_WINDOW_HAS_NO_LENGTH).toBeTruthy();
  });

  test("a legacy change in Customer Approval whose requirement box is false (a migrated row) is asked again by a Re-schedule: it stays in Customer Approval, no CAB, nothing but its own window and request changes; and only the customer's own answer schedules it", async ({ page }) => {
    // A change that sits in Customer Approval with its stored requirement false: how a migrated change looks. The registered
    // contacts of its project are asked (the backend provisions their stage on the next write).
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    api.setState("customer_approval");
    api.syncCustomers();
    await openDetail(detail);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    expect(api.flags()).toEqual({ customerApprovalRequired: false, customerReviewRequired: false });
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
    await expect(detail.rescheduleButton()).toBeVisible();

    await detail.rescheduleButton().click();
    await detail.fillRescheduleWindow("Planned start", NEXT_WEEK_START);
    await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END);
    await detail.rescheduleSubmit().click();
    await expect(detail.rescheduleDialog()).toHaveCount(0);
    expect(api.state()).toBe("customer_approval");
    expect(stageNames(api)).toEqual(["Customer Approval", "Customer Approval"]);
    expect(api.stages().map((st) => st.approvers.map((a) => a.status))).toEqual([
      ["CANCELLED", "CANCELLED"],
      ["REQUESTED", "REQUESTED"],
    ]);
    // A Re-schedule writes no requirement flag (a sync-owned shape it must not touch) and never the outcome stamp.
    expect(api.flags()).toEqual({ customerApprovalRequired: false, customerReviewRequired: false });
    expect(api.customerApproved()).toBe(false);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");

    await customerAnswers(page, api, FAKE_CUST_TWO, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
    expect(api.state()).toBe("scheduled");
  });
});

// ---------------------------------------------------------------------------
// The lifecycle, state by state: the stepper plots the customer portal's eleven stages (New,
// Assess, Authorize, Customer Approval, Scheduled, Implement, Review, Customer Review,
// Rollback, Closed, Canceled), horizontally, and each says its status in words (visually hidden
// ", done" / ", current" / ", upcoming" / ", not taken" / ", history not recorded" / ", rejected by the customer"); the action
// bar offers the forward move as the primary button, Re-schedule beside it, everything else
// behind "Change state" -- and never a way to answer for the customer (no "Bypass customer
// approval" / "Bypass customer review", as a button or in the menu). Against the in-browser fake of the backend.
// ---------------------------------------------------------------------------

/** What the page offers in one state: the stepper's columns and the action bar. */
interface Surface {
  /** The eleven stage statuses (see {@link expectedStages}). */
  stages: string;
  /** The label of the one primary button, or null when there is none. */
  primary: string | null;
  /** Whether the outlined "Re-schedule" button is there. */
  reschedule: boolean;
  /** The "Change state" menu entries, in order; empty = no menu (no Change state button at all). */
  menu: Array<string | RegExp>;
}

/** Every label the action bar can put on its primary button. */
const PRIMARY_LABELS = ["Request Approval", "Start implementation", "Mark implemented", "Send for customer review", "Close"];

/** The action bar offers exactly this: the primary button, Re-schedule, and the "Change state" menu's entries. */
async function expectActionBar(
  detail: ChangeRequestDetailPage,
  bar: Pick<Surface, "primary" | "reschedule"> & { menu: Array<string | RegExp> },
): Promise<void> {
  // Nothing words answering for the customer: no bypass, no "Record customer approval", anywhere on the page.
  await expect(detail.answerForCustomerWording()).toHaveCount(0);
  for (const label of PRIMARY_LABELS) {
    await expect(detail.page.getByRole("button", { name: label, exact: true }), label).toHaveCount(label === bar.primary ? 1 : 0);
  }
  await expect(detail.rescheduleButton()).toHaveCount(bar.reschedule ? 1 : 0);
  if (bar.menu.length === 0) {
    await expect(detail.changeStateButton()).toHaveCount(0);
    return;
  }
  // "Change state" is the main (contained) button when nothing else is, the outlined one beside a primary move.
  await expect(detail.changeStateButton()).toHaveClass(bar.primary ? /MuiButton-outlined/ : /MuiButton-contained/);
  await detail.openChangeStateMenu();
  await expect(detail.menuItems()).toHaveText(bar.menu);
  await expect(detail.answerForCustomerWording()).toHaveCount(0);
  await detail.closeChangeStateMenu();
}

/** The change is in `label`'s state, the stepper reads `surface.stages`, and the action bar offers `surface`. */
async function expectSurface(
  detail: ChangeRequestDetailPage,
  label: string,
  surface: Surface,
  flags: { approval: boolean; review: boolean },
): Promise<void> {
  await expect(detail.currentStep()).toContainText(label);
  await expectStages(detail, surface.stages, flags);
  await expectActionBar(detail, surface);
}

/** Drives a Normal CR with no project (nobody to ask) through to Customer Approval, the creator signed in. */
async function driveToCustomerApproval(page: Page, api: FakeChangeRequestApi, detail: ChangeRequestDetailPage): Promise<void> {
  await approveInternally(page, api, detail);
  await switchTo(page, api, FAKE_CREATOR);
  await expect(detail.currentStep()).toContainText("Customer Approval");
}

/** Drives a Normal CR (customer review on) through Review to Customer Review, the creator signed in. */
async function driveToCustomerReview(page: Page, api: FakeChangeRequestApi, detail: ChangeRequestDetailPage): Promise<void> {
  await driveToReview(page, api, detail);
  await detail.sendForCustomerReviewButton().click();
  await expect(detail.currentStep()).toContainText("Customer Review");
}

test.describe("change request lifecycle — the stepper and the action bar in every state (mocked backend)", () => {
  for (const customer of [true, false]) {
    test(`Normal, customer approval and customer review ${customer ? "on" : "off"}: every state shows its stage statuses and its actions`, async ({ page }) => {
      test.setTimeout(240_000);
      const api = await installFakeChangeRequestApi(
        page,
        "normal",
        FAKE_CREATOR,
        { customerApprovalRequired: customer, customerReviewRequired: customer },
        customer ? ON_ACME : {},
      );
      const detail = new ChangeRequestDetailPage(page);
      const flags = { approval: customer, review: customer };
      // Columns: New, Assess, Authorize, Customer Approval, Scheduled, Implement, Review, Customer Review,
      // Rollback, Closed, Canceled (d done, c current, p upcoming, n not taken, u history not recorded, r rejected by the customer).
      // Rollback and Canceled are exceptions: "not taken" until the change really ends in them.

      await openDetail(detail);
      await expectSurface(detail, "New", { stages: "c p p p p p p p n p n", primary: "Request Approval", reschedule: false, menu: ["Cancel change"] }, flags);

      await detail.requestApproval();
      await expectSurface(detail, "Assess", { stages: "d c p p p p p p n p n", primary: null, reschedule: false, menu: ["Cancel change"] }, flags);

      await switchTo(page, api, FAKE_PEER);
      await detail.approve("Pat Peer");
      await switchTo(page, api, FAKE_CREATOR);
      await expectSurface(detail, "Authorize", { stages: "d d c p p p p p n p n", primary: null, reschedule: false, menu: ["Cancel change"] }, flags);

      await switchTo(page, api, FAKE_CAB);
      await detail.approve("Cam Cab");
      await switchTo(page, api, FAKE_CREATOR);
      if (customer) {
        // The customer is asked (Acme's contacts): Re-schedule stays beside Change state, whose menu holds Cancel change only.
        await expectSurface(
          detail,
          "Customer Approval",
          { stages: "d d d c p p p p n p n", primary: null, reschedule: true, menu: ["Cancel change"] },
          flags,
        );
        // The customer approves in the customer portal; the page, reloaded, shows Scheduled.
        await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
      }
      await expectSurface(detail, "Scheduled", { stages: "d d d d c p p p n p n", primary: "Start implementation", reschedule: false, menu: ["Cancel change"] }, flags);

      await detail.page.getByRole("button", { name: "Start implementation" }).click();
      await expectSurface(detail, "Implement", { stages: "d d d d d c p p n p n", primary: "Mark implemented", reschedule: false, menu: ["Cancel change"] }, flags);

      await detail.page.getByRole("button", { name: "Mark implemented" }).click();
      await expectSurface(
        detail,
        "Review",
        {
          stages: "d d d d d d c p n p n",
          primary: customer ? "Send for customer review" : "Close",
          reschedule: false,
          menu: ["Roll back", "Cancel change"],
        },
        flags,
      );

      if (customer) {
        await detail.sendForCustomerReviewButton().click();
        // No Close button here, and none in the menu: the customer's review is theirs to give. Roll back is held back
        // (disabled, with why) while they are asked.
        await expectSurface(
          detail,
          "Customer Review",
          { stages: "d d d d d d d c n p n", primary: null, reschedule: false, menu: [/^Roll back/, "Cancel change"] },
          flags,
        );
        await customerAnswers(page, api, FAKE_CUST_TWO, "approved");
      } else {
        // A plain Close out of Review is an ordinary forward move: no dialog, no reason.
        await detail.closeButton().click();
        await expect(detail.reasonDialog()).toHaveCount(0);
      }
      await expectSurface(detail, "Closed", { stages: "d d d d d d d d n c n", primary: null, reschedule: false, menu: [] }, flags);
      expect(api.state()).toBe("closed");
      // Nobody recorded anything on the customer's behalf (the answers came from the customer portal), and a plain Close needs no reason.
      expect(api.journal()).toEqual([]);
    });
  }

  test("Rollback: the stage turns current, everything through Review stays done, Closed and Canceled are not taken (an older change whose project had nobody to ask: Customer Review leaves no proof either way)", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "review");
    // Before: Review is current and Rollback is a faint exception.
    await expectStages(detail, "d d d d d d c p n p n", { approval: false, review: true });
    await expect(detail.stage("Rollback")).toHaveText("Rollback, not taken");

    await rollBackWithReason(page, detail, "Smoke test failed after deployment.");
    // The project has no registered contacts, so entering Customer Review would have left no stage either:
    // whether it was entered cannot be told, and the line says so rather than guessing "not taken". (Request Approval is
    // refused for such a project now, so this is an older change: it starts in Review.)
    await expectStages(detail, "d d d d d d d u c n n", { approval: false, review: true });
    await expectRolledBack(page, detail, api);
    await page.reload();
    await expectStages(detail, "d d d d d d d u c n n", { approval: false, review: true });
  });

  test("Rollback from Review on a project with registered contacts: Customer Review was never entered, so it reads 'not taken'", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    await rollBackWithReason(page, detail, "Smoke test failed after deployment.");
    // The contacts would have been asked on entering Customer Review, and there is no stage for them: it was not entered.
    await expectStages(detail, "d d d d d d d n c n n", { approval: false, review: true });
    await expectRolledBack(page, detail, api);
    await page.reload();
    await expectStages(detail, "d d d d d d d n c n n", { approval: false, review: true });
  });

  test("Canceled: the stage turns current; with no approvals to prove it, earlier stages read 'history not recorded', never done or upcoming", async ({ page }) => {
    test.setTimeout(120_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await cancelChangeWithReason(page, detail, "Raised against the wrong environment.");
    await expect(detail.currentStep()).toContainText("Canceled");
    expect(api.state()).toBe("canceled");
    await expectStages(detail, "u u u u u u u u n n c", { approval: false, review: false });
    await expect(detail.stage("Rollback")).toHaveText("Rollback, not taken");
    await expect(detail.stage("Closed")).toHaveText("Closed, not taken");
    await expect(detail.changeStateButton()).toHaveCount(0);
  });

  test("Canceled in Review: what the approvals prove was passed is done, the rest is 'history not recorded'", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    await cancelChangeWithReason(page, detail, "Superseded by CHG-1234.");
    await expect(detail.currentStep()).toContainText("Canceled");
    expect(api.state()).toBe("canceled");
    // Peer and CAB approved and the Review stage was provisioned, so the change got as far as Implement.
    await expectStages(detail, "d d d d d d u u n n c", { approval: false, review: false });
  });

  test("Canceled at Customer Approval while the customer is asked: the stages before it are done, it and the rest are not recorded", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    await expectStages(detail, "d d d c p p p p n p n", { approval: true, review: false });

    await cancelChangeWithReason(page, detail, "The customer asked to postpone indefinitely.");
    await expect(detail.currentStep()).toContainText("Canceled");
    expect(api.state()).toBe("canceled");
    await expectStages(detail, "d d d u u u u u n n c", { approval: true, review: false });
  });

  test("Canceled in Review after the Review stage was approved: Review itself reads 'history not recorded', since approving it does not move the change on", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    await switchTo(page, api, FAKE_PEER);
    await detail.approve(FAKE_PEER.name, "Review");
    await expect(detail.approverStatus(FAKE_PEER.name, "Review")).toHaveText("Approved");
    await switchTo(page, api, FAKE_CREATOR);
    // Approved, and still in Review: the engineer has not moved it on.
    await expect(detail.currentStep()).toContainText("Review");
    expect(api.state()).toBe("review");
    await cancelChangeWithReason(page, detail, "Superseded by CHG-1234.");
    await expect(detail.currentStep()).toContainText("Canceled");
    // Review was never left, so it is not done; Peer and CAB approved and Review was entered, so Implement and before were passed.
    await expectStages(detail, "d d d d d d u u n n c", { approval: false, review: false });
    await expect(detail.stage("Review")).toHaveText("Review, history not recorded");
  });

  test("Canceled after the customer approved (in the customer portal): Customer Approval reads done (their approved stage and the recorded approval prove it), later stages are not recorded", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
    await cancelChangeWithReason(page, detail, "The window moved out of the quarter.");
    await expect(detail.currentStep()).toContainText("Canceled");
    expect(api.state()).toBe("canceled");
    // The customer's approval is on record (their APPROVED stage row and `hasCustomerApproved`): Customer Approval was
    // passed. The change may have been canceled at Scheduled itself, so that is not.
    await expectStages(detail, "d d d d u u u u n n c", { approval: true, review: false });
    await expect(detail.stage("Customer Approval")).toHaveText("Customer Approval, done");
  });

  test("a customer stage is left off the line while its checkbox is off, and joins it when ticked", async ({ page }) => {
    test.setTimeout(120_000);
    await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    await expectStages(detail, "c p p p p p p p n p n", { approval: false, review: false });
    await expect(detail.stage("Customer Approval")).toHaveCount(0);
    await expect(detail.stage("Customer Review")).toHaveCount(0);
    await expect(detail.stepLabels()).toHaveCount(9);

    await detail.openEditDialog();
    await detail.editCustomerApprovalCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    // Only the ticked one joins, in the customer portal's place for it.
    await expectStages(detail, "c p p p p p p p n p n", { approval: true, review: false });
    await expect(detail.stepLabels()).toHaveCount(10);

    await detail.openEditDialog();
    await detail.editCustomerReviewCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    await expectStages(detail, "c p p p p p p p n p n", { approval: true, review: true });
    await expect(detail.stepLabels()).toHaveCount(11);
  });
});

// ---------------------------------------------------------------------------
// The customer's answer is not the staff's to give. Staff never record a customer's approval or
// review: while a change is in Customer Approval / Customer Review it moves on only through the
// customer's own answer in the customer portal. So the "Change state" menu holds no "Bypass customer
// approval" / "Bypass customer review" -- not as a button, not as an entry, not even a disabled one --
// whether the customer's request is pending or nobody was asked. What staff keep: Re-schedule (Customer
// Approval), Roll back (Review, Customer Review; held back, disabled with why, while the customer's
// review is pending) and Cancel change.
// ---------------------------------------------------------------------------

/** Why Roll back is disabled at Customer Review while the customer is asked: a failed review is theirs to give. */
const PENDING_REVIEW_ROLLBACK_REASON =
  "Customer review is pending from Mia Member, Max Member. A failed review is theirs to give in the Customer Portal, so the change can't be rolled back from here.";

/** PATCHes `body` on the fake CR from inside the page (the page's own origin and routes), as the app's client would. */
async function patchFromPage(page: Page, body: Record<string, unknown>): Promise<{ status: number; message: string }> {
  return await page.evaluate(
    async ({ crId, data }) => {
      const base = (window as unknown as { config: { CSM_PORTAL_BACKEND_BASE_URL: string } }).config.CSM_PORTAL_BACKEND_BASE_URL;
      const response = await fetch(`${base}/change-requests/${crId}`, {
        method: "PATCH",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(data),
      });
      const parsed = (await response.json()) as { message?: string };
      return { status: response.status, message: parsed.message ?? "" };
    },
    { crId: FAKE_CR_ID, data: body },
  );
}

/** PATCHes `{ state }` on the fake CR from inside the page. */
async function patchStateFromPage(page: Page, state: string): Promise<{ status: number; message: string }> {
  return await patchFromPage(page, { state });
}

/** A request that changes something: a PATCH, or a posted comment (the reason recorded before a state change). */
const isWrite = (request: string): boolean => request.startsWith("PATCH ") || (request.startsWith("POST ") && request.endsWith("/comments"));

test.describe("change request action bar — nobody answers for the customer (mocked backend)", () => {
  test("Customer Approval, the customer's request pending: Re-schedule and Cancel change only; no Bypass anywhere, and the API refuses {state: scheduled}", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval"); // the approvals are loaded

    // The bar: Re-schedule beside Change state, which is the main button; the menu holds Cancel change and nothing else.
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });
    await expect(detail.rescheduleButton()).toHaveClass(/MuiButton-outlined/);
    await expectOnlyCancelActionable(detail, "approval");
    // Not even the explanation of a disabled entry: who is asked is shown in the Approvals tab, not in the menu.
    await expect(page.getByText(/is pending from/i)).toHaveCount(0);
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");

    // Nothing was sent, and the backend would refuse it with its own words if it were.
    expect(api.requestBodies().filter((b) => b.request.startsWith("PATCH ") && b.body?.state === "scheduled")).toEqual([]);
    expect(await patchStateFromPage(page, "scheduled")).toEqual({ status: 400, message: customerAnswerRefusal("scheduled") });
    expect(api.state()).toBe("customer_approval");
  });

  test("Customer Approval, nobody asked (an older change on a project with no registered contacts): the same bar -- Re-schedule beside Change state, whose menu holds Cancel change -- and the same refusal in words", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();

    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });
    await expect(detail.changeStateButton()).toHaveClass(/MuiButton-contained/);
    await expect(detail.rescheduleButton()).toHaveClass(/MuiButton-outlined/);
    await expectNoBypass(detail);

    // Nobody was asked, and still the backend will not record the approval for the customer.
    expect(await patchStateFromPage(page, "scheduled")).toEqual({ status: 400, message: customerAnswerRefusal("scheduled") });
    expect(api.state()).toBe("customer_approval");
    expect(api.journal()).toEqual([]);
  });

  test("Customer Review, the customer's request pending: Roll back is disabled and says why; there is no Close and no Bypass, and the API refuses {state: closed} and {state: rollback}", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerReview(page, api, detail);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Review");

    await expectActionBar(detail, { primary: null, reschedule: false, menu: [/^Roll back/, "Cancel change"] });
    await expect(detail.closeButton()).toHaveCount(0);
    await detail.openChangeStateMenu();
    // The backend withdraws Roll back while the review is pending (a failed review is the customer's to give), so it is
    // listed disabled with its own reason.
    await expect(detail.rollbackMenuItem()).toBeDisabled();
    await expect(detail.rollbackMenuItem().getByText(PENDING_REVIEW_ROLLBACK_REASON)).toBeVisible();
    await expect(detail.rollbackMenuItem()).toHaveAccessibleName(`Roll back: ${PENDING_REVIEW_ROLLBACK_REASON}`);
    // A disabled entry is still reachable by keyboard (so its reason can be read) and does nothing.
    await detail.rollbackMenuItem().focus();
    await expect(detail.rollbackMenuItem()).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(detail.reasonDialog()).toHaveCount(0);
    await detail.closeChangeStateMenu();

    // The backend refuses closing on the customer's behalf (whoever asks) and rolling back while they are asked, in its own words.
    expect(await patchStateFromPage(page, "closed")).toEqual({ status: 400, message: customerAnswerRefusal("closed", true) });
    expect(await patchStateFromPage(page, "rollback")).toEqual({ status: 400, message: customerStageManualRefusal("rollback") });
    expect(api.state()).toBe("customer_review");
    expect(api.journal()).toEqual([]);
  });

  test("Customer Review, nobody asked (an older change): Roll back is the first menu entry, before Cancel change, enabled; no button or entry closes the change, and the API refuses {state: closed}", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_review");
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toBeVisible();

    await expectActionBar(detail, { primary: null, reschedule: false, menu: ["Roll back", "Cancel change"] });
    await expect(detail.closeButton()).toHaveCount(0);
    await detail.openChangeStateMenu();
    await expect(detail.page.getByRole("menuitem", { name: /^close$/i })).toHaveCount(0);
    await expect(detail.rollbackMenuItem()).toBeEnabled();
    await detail.closeChangeStateMenu();
    await expectNoBypass(detail);

    expect(await patchStateFromPage(page, "closed")).toEqual({ status: 400, message: customerAnswerRefusal("closed") });
    expect(api.state()).toBe("customer_review");
  });

  test("there is no bypass anywhere else either: Review's plain Close is a button, and neither Review nor Scheduled lists anything like one", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR);
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    // Customer review is not required: Close is the primary move out of Review, and it is not the customer's answer.
    await expectActionBar(detail, { primary: "Close", reschedule: false, menu: ["Roll back", "Cancel change"] });
    await expectNoBypass(detail);
  });
});

// ---------------------------------------------------------------------------
// The only ways on out of a customer gate: the customer's own answer (applied server-side, as the customer
// portal does), Re-schedule, Roll back and Cancel change. The customer's answer stamps what the customer
// confirmed ("Customer approved" / "Customer reviewed"); a stray manual PATCH of it is refused from any
// caller and changes nothing, not even a work note.
// ---------------------------------------------------------------------------

test.describe("change request approval flow — the customer's own answer is the only way on (mocked backend)", () => {
  test("Customer Approval: the customer approves in the customer portal -> Scheduled, 'Customer approved' reads Yes, and nothing was sent from the CSM page", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    const flags = { approval: true, review: false };
    await expectStages(detail, "d d d c p p p p n p n", flags);
    await expect(detail.flagValue("Customer approved")).toHaveText("No");
    const before = api.requests().length;

    await customerAnswers(page, api, FAKE_CUST_TWO, "approved");
    await expect(detail.currentStep()).toContainText("Scheduled");
    expect(api.state()).toBe("scheduled");
    await expectStages(detail, "d d d d c p p p n p n", flags);
    await expect(detail.stage("Customer Approval")).toHaveText("Customer Approval, done");
    await expect(detail.approverStatus(FAKE_CUST_TWO.name, "Customer Approval")).toHaveText("Approved");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Cancelled");
    await expect(detail.flagValue("Customer approved")).toHaveText("Yes");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Start implementation" })).toBeVisible();
    await expectNoBypass(detail);
    // No PATCH and no work note went out from the page: the answer is not staff's to give or to annotate.
    expect(api.requests().slice(before).filter(isWrite)).toEqual([]);
    expect(api.journal()).toEqual([]);
  });

  test("Customer Review: the customer approves in the customer portal -> Closed, 'Customer reviewed' reads Yes, and nothing was sent from the CSM page", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerReview(page, api, detail);
    const flags = { approval: false, review: true };
    await expectStages(detail, "d d d d d d d c n p n", flags);
    await expect(detail.flagValue("Customer reviewed")).toHaveText("No");
    const before = api.requests().length;

    await customerAnswers(page, api, FAKE_CUST_ONE, "approved");
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
    await expectStages(detail, "d d d d d d d d n c n", flags);
    await expect(detail.stage("Customer Review")).toHaveText("Customer Review, done");
    await expect(detail.flagValue("Customer reviewed")).toHaveText("Yes");
    await expect(detail.blockingReason()).toHaveCount(0);
    await expect(detail.changeStateButton()).toHaveCount(0); // nothing is left to do
    expect(api.requests().slice(before).filter(isWrite)).toEqual([]);
    expect(api.journal()).toEqual([]);
  });

  test("a manual scheduled / closed sent anyway is refused with the backend's words, from a customer gate, asked or not, and changes nothing (no state, no stamp, no work note)", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true, customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_approval"); // an older change: nobody was asked

    expect(await patchStateFromPage(page, "scheduled")).toEqual({ status: 400, message: customerAnswerRefusal("scheduled") });
    expect(api.state()).toBe("customer_approval");
    await page.reload();
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.flagValue("Customer approved")).toHaveText("No");

    // The same out of Customer Review (the change put there out of band: nothing in the page gets it there without the
    // engineer-driven tail): closed is refused too.
    api.setState("customer_review");
    await page.reload();
    await expect(detail.currentStep()).toContainText("Customer Review");
    expect(await patchStateFromPage(page, "closed")).toEqual({ status: 400, message: customerAnswerRefusal("closed") });
    expect(api.state()).toBe("customer_review");
    await page.reload();
    await expect(detail.flagValue("Customer reviewed")).toHaveText("No");
    expect(api.journal()).toEqual([]);
  });

  test("Cancel change out of a customer gate still takes a reason, recorded as an internal note before the PATCH", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    await cancelChangeWithReason(page, detail, "The customer asked to postpone indefinitely.");
    await expect(detail.currentStep()).toContainText("Canceled");
    expect(api.state()).toBe("canceled");
    expect(api.journal()).toEqual([{ kind: "comment", text: "The customer asked to postpone indefinitely." }]);
  });

  test("the backend refuses a Roll back the page still thought possible (a customer review request appeared behind the dialog): its words show in the dialog and the reason is not posted twice on retry", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, NO_CONTACTS);
    const detail = new ChangeRequestDetailPage(page);
    await openOlderChangeAt(api, detail, "customer_review"); // an older change: nobody was asked
    await expectActionBar(detail, { primary: null, reschedule: false, menu: ["Roll back", "Cancel change"] }); // nobody is asked

    await detail.openChangeStateMenu();
    await detail.rollbackMenuItem().click();
    const dialog = detail.reasonDialog();
    await expect(dialog.getByRole("heading", { name: "Roll back this change?" })).toBeVisible();
    await dialog.getByLabel("Reason").fill("The customer's review failed on the phone.");

    // Meanwhile someone registers a contact for the project: the customer is now being asked.
    api.setProjectContacts(GAMMA.id, [FAKE_CUST_ONE]);
    api.syncCustomers();
    await dialog.getByRole("button", { name: "Roll back", exact: true }).click();

    // The backend's refusal, verbatim, with the news that the reason itself was recorded.
    const alert = dialog.getByRole("alert");
    await expect(alert).toContainText(customerStageManualRefusal("rollback"));
    await expect(alert).toContainText("Your reason was recorded as an internal note, but the state did not change");
    await expect(dialog.getByLabel("Reason")).toBeDisabled(); // locked: retrying only re-sends the state change
    expect(api.state()).toBe("customer_review");

    // The request goes away again (the contact is deregistered): retrying works, and the reason is not posted twice.
    api.setProjectContacts(GAMMA.id, []);
    api.syncCustomers();
    await dialog.getByRole("button", { name: "Roll back", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(detail.currentStep()).toContainText("Rollback");
    expect(api.state()).toBe("rollback");
    expect(api.requests().filter((r) => r === `POST /change-requests/${FAKE_CR_ID}/comments`)).toHaveLength(1);
    expect(api.journal()).toEqual([{ kind: "comment", text: "The customer's review failed on the phone." }]);
  });
});

// ---------------------------------------------------------------------------
// The state machine, as the backend now enforces it (mocked backend: the fake mirrors `patchChangeRequestTx`):
//  - a FINAL state (Canceled, Closed, Rollback) has NO exit: a cancelled change used to be revived with
//    `{state: "implement"}` (200), including one cancelled out of Customer Approval with nobody having answered;
//  - there is no jump over a state or an approval gate: `{state: "implement"}` from New, Assess or Authorize with the
//    customer's approval ticked used to land in Implement, skipping Peer, CAB and the customer;
//  - Re-schedule out of Customer Approval re-asks the customer, even for a row whose stored requirement is false (a
//    migrated row defaults to false): the CAB's new approval goes back to the customer, never to Scheduled.
// Every refusal is a 400 with its own words and writes nothing: no state, no stage, no work note.
// ---------------------------------------------------------------------------

/** Every lifecycle state a manual PATCH could name (the API's spelling). */
const EVERY_STATE = ["new", "assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"] as const;

/** A planned window that differs from the fake's own (2030-03-01 09:00-11:00 UTC), as a Re-schedule sends it. */
const RESCHEDULE_BODY = { state: "authorize", plannedStartOn: "2030-03-08 09:00:00", plannedEndOn: "2030-03-08 11:00:00" };

/**
 * Everything the fake holds that a refused PATCH must leave alone: state, planned window, stages with their approver
 * rows, the journal, and the tick boxes.
 */
function snapshot(api: FakeChangeRequestApi): string {
  return JSON.stringify({ state: api.state(), planned: api.planned(), stages: api.stages(), journal: api.journal(), flags: api.flags() });
}

test.describe("change request state machine — final states and jumps (mocked backend)", () => {
  test("Canceled out of Customer Approval is final: not one state can be named afterwards (a Re-schedule included), each refusal says so in words and writes nothing; no action is offered", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await driveToCustomerApproval(page, api, detail);
    await cancelChangeWithReason(page, detail, "The customer asked to postpone indefinitely.");
    await expect(detail.currentStep()).toContainText("Canceled");
    expect(api.state()).toBe("canceled");
    const before = snapshot(api);

    for (const target of EVERY_STATE) {
      const refused = await patchStateFromPage(page, target);
      // Naming the state it is in is a resend (no move) for every state but Rollback, whose own refusal says it is final.
      if (target === "canceled") {
        expect([target, refused.status], "a resend of Canceled is no move").toEqual([target, 200]);
        continue;
      }
      expect([target, refused.status, refused.message], `a canceled change asked for ${target}`).toEqual([target, 400, finalStateMessage(target, "canceled")]);
    }
    // A Re-schedule is a state change too: no way back into Authorize and the customer's request does not reopen.
    const rescheduled = await patchFromPage(page, RESCHEDULE_BODY);
    expect([rescheduled.status, rescheduled.message]).toEqual([400, finalStateMessage("authorize", "canceled")]);
    // Not even riding along with a state change: a refused PATCH writes nothing of its own request either.
    const withComment = await patchFromPage(page, { state: "implement", comment: "revive it" });
    expect(withComment.status).toBe(400);
    expect(snapshot(api)).toBe(before);

    await page.reload();
    await expect(detail.currentStep()).toContainText("Canceled");
    await expect(detail.changeStateButton()).toHaveCount(0);
    await expect(detail.rescheduleButton()).toHaveCount(0);
    await expectNoBypass(detail);
  });

  test("Rollback is final: nothing moves a rolled-back change anywhere, Cancel included, and nothing is written", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, {});
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    // Rolled back (a reason first, as the dialog takes one).
    await rollBackWithReason(page, detail, "The deployment was reversed.");
    expect(api.state()).toBe("rollback");
    const before = snapshot(api);
    for (const target of EVERY_STATE) {
      const refused = await patchStateFromPage(page, target);
      // Rollback names itself: a resend would be no move, but "rollback" is only ever set from a review state.
      const want = target === "rollback" ? 'state "rollback" can only be set from review or customer_review' : finalStateMessage(target, "rollback");
      expect([target, refused.status, refused.message], `a rolled-back change asked for ${target}`).toEqual([target, 400, want]);
    }
    expect(snapshot(api)).toBe(before);
    await page.reload();
    await expect(detail.changeStateButton()).toHaveCount(0);
  });

  test("Closed is final: nothing moves a closed change anywhere, Cancel included, and nothing is written", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, {}, {});
    const detail = new ChangeRequestDetailPage(page);
    await driveToReview(page, api, detail);
    await detail.closeButton().click();
    await expect(detail.currentStep()).toContainText("Closed");
    expect(api.state()).toBe("closed");
    const before = snapshot(api);
    for (const target of EVERY_STATE) {
      const refused = await patchStateFromPage(page, target);
      if (target === "closed") {
        expect([target, refused.status], "a resend of Closed is no move").toEqual([target, 200]);
        continue;
      }
      expect([target, refused.status, refused.message], `a closed change asked for ${target}`).toEqual([target, 400, finalStateMessage(target, "closed")]);
    }
    expect(snapshot(api)).toBe(before);
    await page.reload();
    await expect(detail.changeStateButton()).toHaveCount(0);
  });

  test("no jump over a state or an approval gate: with the customer's approval ticked, Implement, Review, Customer Review and Close are refused from New, Assess and Authorize, and every refusal writes nothing", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerApprovalRequired: true, customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await openDetail(detail);
    const jumps = ["implement", "review", "customer_review", "closed", "scheduled", "customer_approval", "rollback"];

    /** Asks for every jump from the current state: each is a 400 that leaves the change exactly as it was. */
    const expectNoJump = async (from: string): Promise<void> => {
      expect(api.state()).toBe(from);
      const before = snapshot(api);
      for (const target of jumps) {
        const refused = await patchStateFromPage(page, target);
        expect([from, target, refused.status], `${from} -> ${target}`).toEqual([from, target, 400]);
        // In the backend's own words: a jump says what the change waits for and which moves ARE open; out of a customer
        // state every destination is the customer's own answer; the review-only refusals keep theirs.
        if (target === "implement" || target === "review") {
          const want = from === "customer_approval" ? customerAnswerRefusal(target, false, "customer_approval") : stateJumpMessage(target, from, true);
          expect(refused.message, `${from} -> ${target}`).toBe(want);
        }
        expect(snapshot(api), `${from} -> ${target} wrote something`).toBe(before);
      }
    };

    // New: the next state is Assess (Request Approval) or Canceled.
    await expectNoJump("new");
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    // A resent Request Approval from where it landed is no move (and provisions nothing twice)...
    const stagesAfterRequest = JSON.stringify(api.stages());
    expect(await patchStateFromPage(page, "assess")).toMatchObject({ status: 200 });
    expect(api.state()).toBe("assess");
    expect(JSON.stringify(api.stages())).toBe(stagesAfterRequest);
    // Assess: Peer approval is pending.
    await expectNoJump("assess");
    await switchTo(page, api, FAKE_PEER);
    await detail.approve("Pat Peer");
    await expect(detail.currentStep()).toContainText("Authorize");
    // Authorize: the CAB approval is pending. Request Approval is not a move out of anywhere but New (and where it lands).
    expect(await patchStateFromPage(page, "assess")).toEqual({ status: 400, message: "approval can only be requested for a change request in the New state" });
    await expectNoJump("authorize");
    expect(api.stages().map((st) => [st.stage, st.status])).toEqual([["Peer Approval", "APPROVED"], ["CAB Approval", "REQUESTED"]]);
    // The legal path still works: the CAB approves and the customer is asked, not skipped.
    await switchTo(page, api, FAKE_CAB);
    await detail.approve("Cam Cab");
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    expect(api.state()).toBe("customer_approval");
    // Customer Approval: only the customer's own answer (or a Re-schedule or a Cancel) moves it.
    await expectNoJump("customer_approval");
    await expect(detail.approverStatus(FAKE_CUST_ONE.name, "Customer Approval")).toHaveText("Requested");
  });

  test("no jump over a state later on either: Scheduled, Implement and Review only move to their next state (or Cancel), and Review needs its customer review when it is ticked", async ({ page }) => {
    test.setTimeout(240_000);
    const api = await installFakeChangeRequestApi(page, "normal", FAKE_CREATOR, { customerReviewRequired: true }, ON_ACME);
    const detail = new ChangeRequestDetailPage(page);
    await approveInternally(page, api, detail);
    await switchTo(page, api, FAKE_CREATOR);
    await expect(detail.currentStep()).toContainText("Scheduled");

    // Scheduled: Implement or Cancel. Review, Customer Review and Close would skip Implement.
    for (const target of ["review", "customer_review", "closed", "rollback"]) {
      const refused = await patchStateFromPage(page, target);
      expect([target, refused.status], `scheduled -> ${target}`).toEqual([target, 400]);
    }
    expect((await patchStateFromPage(page, "review")).message).toBe(stateJumpMessage("review", "scheduled", true));
    expect(api.state()).toBe("scheduled");
    await page.getByRole("button", { name: "Start implementation" }).click();
    await expect(detail.currentStep()).toContainText("Implement");
    // Implement: Review or Cancel. Customer Review and Close would skip Review.
    for (const target of ["customer_review", "closed", "rollback"]) {
      const refused = await patchStateFromPage(page, target);
      expect([target, refused.status], `implement -> ${target}`).toEqual([target, 400]);
    }
    expect((await patchStateFromPage(page, "customer_review")).message).toBe(stateJumpMessage("customer_review", "implement", true));
    expect(api.state()).toBe("implement");
    await page.getByRole("button", { name: "Mark implemented" }).click();
    await expect(detail.currentStep()).toContainText("Review");
    // Review with the customer's review required: Close would skip it.
    const closeFromReview = await patchStateFromPage(page, "closed");
    expect(closeFromReview.status).toBe(400);
    expect(closeFromReview.message).toMatch(/customer review is required for this change request/);
    expect(api.state()).toBe("review");
    // The legal path: Send for customer review.
    await detail.sendForCustomerReviewButton().click();
    await expect(detail.currentStep()).toContainText("Customer Review");
  });
});

// ---------------------------------------------------------------------------
// The customer requirements lock, on the REAL stack (the CSM webapp, the CSM backend, entity-service,
// the database): who the customer is (the Customer Project, from which the read-only Customer Group is
// derived) and whether the customer is asked (the two creation-form boxes) are fixed when the change is
// created. In New the project and both boxes are freely editable. The moment Request Approval is pressed
// the project is FROZEN for everyone and the boxes are ADD-ONLY: ticking one on is accepted until the gate
// it controls is passed, unticking one is refused in every state but New. A correction is Cancel and Clone.
// A Re-schedule back to Authorize therefore cannot reopen anything: a box that was ticked stays ticked, so
// the CAB's approval of the new plan asks the same contacts again. Every refusal is a readable 400.
//
// Each change request is RAISED through the CSM backend (utils/realStackApi.ts: jane raises, alice and bob
// approve) and the dialog is driven as jane, the creator. The pictures (E2E_SHOT_DIR) are of the dialog.
// The same table as the Go truth table (`TestCustomerRequirementsLock_TruthTable`) and the Vitest one, state
// by state, is asserted at the API in "every state" below, with the backend's own words.
// ---------------------------------------------------------------------------

const LOCK_PREFIX = "E2E lock: ";
const lockSubject = (title: string): string => `${LOCK_PREFIX}${title}`;
async function deleteLockChanges(): Promise<void> {
  await psql(`delete from work_item where type = 'CHANGE_REQUEST' and subject like '${LOCK_PREFIX}%'`);
}
async function shotTo(page: Page, name: string): Promise<void> {
  const dir = process.env.E2E_SHOT_DIR?.trim();
  if (!dir) return;
  fs.mkdirSync(dir, { recursive: true });
  await page.waitForTimeout(500); // a dialog fades in: wait it out
  await page.screenshot({ path: path.join(dir, `${name}.png`) });
}
/** A tall, dark window for the pictures (the Edit dialog is long) -- harmless to the assertions. */
async function pictureWindow(page: Page): Promise<void> {
  await page.setViewportSize({ width: 1280, height: 1500 });
  await page.emulateMedia({ colorScheme: "dark" });
}
/** What a customer requirement's checkbox in the open Edit dialog says: ticked, enabled, and the line under it. */
async function requirementBox(detail: ChangeRequestDetailPage, box: "approval" | "review") {
  const checkbox = box === "approval" ? detail.editCustomerApprovalCheckbox() : detail.editCustomerReviewCheckbox();
  return { checkbox, text: detail.editDialog().locator(`#cr-edit-customer-${box}-desc`) };
}

test.describe("the customer requirements lock (real stack)", () => {
  test.describe.configure({ timeout: 180_000 });
  test.beforeEach(async () => {
    test.skip(
      !realStackNamed(),
      "These tests raise change requests through the CSM portal's backend, which WRITES: name the stack's own, E2E_CSM_BFF_URL " +
        "(the isolated stack's is http://localhost:18082), together with its mock identity provider (E2E_OIDC_URL, http://localhost:19100), " +
        "entity-service (E2E_ENTITY_SERVICE_URL, http://localhost:18081) and Postgres (E2E_POSTGRES_CONTAINER).",
    );
    await deleteLockChanges();
  });
  test.afterAll(async () => {
    if (realStackNamed()) await deleteLockChanges();
  });

  test("in New the Customer Project and both boxes are freely editable: the dialog offers them and every change, ticks and un-ticks and a swap of project, is saved", async ({ page }) => {
    await pictureWindow(page);
    const cr = await raise({ subject: lockSubject("New is free"), projectId: null, approval: false, review: false });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeEnabled();
    await expect((await requirementBox(detail, "approval")).checkbox).toBeEnabled();
    await expect((await requirementBox(detail, "review")).checkbox).toBeEnabled();
    for (const reason of [PROJECT_FROZEN_REASON, REQUIREMENT_ADD_ONLY_REASON, REQUIREMENT_NEEDS_PROJECT_REASON, REQUIREMENT_ONCE_SAVED]) {
      await expect(detail.editDialog().getByText(reason), `New shows "${reason}"`).toHaveCount(0);
    }
    await shotTo(page, "20-csm-edit-dialog-new-everything-editable");

    // A project and both ticks.
    await detail.editProjectField().click();
    await page.getByRole("option", { name: EXAMPLE_CORP.name }).click();
    await detail.editCustomerApprovalCheckbox().check();
    await detail.editCustomerReviewCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    let stored = (await staff("jane").get(cr.id)).body;
    expect([stored.state, stored.project?.id, stored.customerApprovalRequired, stored.customerReviewRequired]).toEqual(["new", EXAMPLE_CORP.id, true, true]);

    // Still New: un-tick them both and swap the project for another one.
    await page.reload();
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeEnabled();
    await expect(detail.editCustomerApprovalCheckbox()).toBeChecked();
    await expect(detail.editCustomerApprovalCheckbox()).toBeEnabled();
    await detail.editCustomerApprovalCheckbox().uncheck();
    await detail.editCustomerReviewCheckbox().uncheck();
    await detail.editProjectField().click();
    await page.getByRole("option", { name: OTHER_CORP.name }).click();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    stored = (await staff("jane").get(cr.id)).body;
    expect([stored.state, stored.project?.id, stored.customerApprovalRequired, stored.customerReviewRequired]).toEqual(["new", OTHER_CORP.id, false, false]);

    // And through the API: a project change and an un-tick are accepted in New.
    expect((await staff("jane").patch(cr.id, { projectId: EXAMPLE_CORP.id, customerApprovalRequired: true })).status).toBe(200);
    expect((await staff("jane").patch(cr.id, { customerApprovalRequired: false })).status).toBe(200);
  });

  test("Request Approval is disabled with the reason when a customer box is ticked and there is no Customer Project; the API refuses it in words; choosing a project enables it", async ({ page }) => {
    await pictureWindow(page);
    const cr = await raise({ subject: lockSubject("needs a project"), projectId: null, approval: true, review: false });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);

    const blocked = page.getByLabel(new RegExp(`Request Approval: .*${REQUEST_APPROVAL_NEEDS_PROJECT_REASON}`, "i"));
    await expect(blocked).toBeVisible();
    await expect(blocked.getByRole("button", { name: "Request Approval" })).toBeDisabled();
    await blocked.hover(); // the reason is the button's tooltip
    await expect(page.getByRole("tooltip")).toContainText(REQUEST_APPROVAL_NEEDS_PROJECT_REASON);
    await shotTo(page, "21-csm-request-approval-disabled-needs-a-project");

    // The backend is the authority: asked anyway, it says why in words and moves nothing.
    const refused = await staff("jane").patch(cr.id, { state: "assess" });
    expect(refused.status).toBe(400);
    expect((refused.body as { message?: string }).message).toBe(REQUEST_APPROVAL_NEEDS_PROJECT);
    expect(await stateOf(cr.id)).toBe("new");

    // A project, set in the dialog, lifts the block.
    await detail.openEditDialog();
    await detail.editProjectField().click();
    await page.getByRole("option", { name: EXAMPLE_CORP.name }).click();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(detail.requestApprovalButton()).toBeEnabled();
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    expect(await stateOf(cr.id)).toBe("assess");

    // The same refusal for a change that clears nothing and names no project in the same PATCH...
    const second = await raise({ subject: lockSubject("needs a project, review only"), projectId: null, approval: false, review: true });
    const refusedReview = await staff("jane").patch(second.id, { state: "assess" });
    expect([refusedReview.status, (refusedReview.body as { message?: string }).message]).toEqual([400, REQUEST_APPROVAL_NEEDS_PROJECT]);
    // ...while clearing the box in the same PATCH, or choosing the project in it, is accepted.
    expect((await staff("jane").patch(second.id, { customerReviewRequired: false, state: "assess" })).status).toBe(200);
    const third = await raise({ subject: lockSubject("needs a project, chosen with the request"), projectId: null, approval: true, review: false, type: "standard" });
    expect((await staff("jane").patch(third.id, { projectId: EXAMPLE_CORP.id, state: "assess" })).status).toBe(200);
    expect(await stateOf(third.id)).toBe("customer_approval");
  });

  /**
   * The first active project whose change requests would reach a customer gate with nobody to ask, as the BACKEND itself says it
   * (a throwaway change on it reports no `customerContacts`): the generated projects hold no registered portal-user contact, the
   * seeded ones (Example Corp, Other Corp) do. Not a definition of its own: the backend's list is the one asked.
   */
  async function projectWithNobodyToAsk(): Promise<{ id: string; name: string } | undefined> {
    const rows = (await psqlOutput("select id::text || '|' || name from project where is_active order by name")).split("\n").filter(Boolean);
    for (const row of rows) {
      const [id, name] = row.split("|") as [string, string];
      const probe = await raise({ subject: lockSubject("probe: who can be asked"), projectId: id, approval: false, review: false });
      if ((await staff("jane").get(probe.id)).body.customerContacts?.length === 0) return { id, name };
    }
    return undefined;
  }

  test("Request Approval is refused when a customer box is ticked and nobody on the project can be asked: disabled with the reason, refused in the backend's words by the API, lifted by clearing the box; ticking a box on after New is refused the same way, in the dialog", async ({ page }) => {
    await pictureWindow(page);
    const nobody = await projectWithNobodyToAsk();
    test.skip(!nobody, "every project of this database has a registered portal-user contact: nothing to refuse");
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    const cr = await raise({ subject: lockSubject("nobody to ask"), projectId: nobody!.id, approval: true, review: false });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);

    // The page knows the project has no registered contact: Request Approval is disabled with the reason (its tooltip).
    const blocked = page.getByLabel(`Request Approval: ${REQUEST_APPROVAL_NEEDS_CONTACT_REASON}`);
    await expect(blocked).toBeVisible();
    await expect(blocked.getByRole("button", { name: "Request Approval" })).toBeDisabled();
    await blocked.hover();
    await expect(page.getByRole("tooltip")).toContainText(REQUEST_APPROVAL_NEEDS_CONTACT_REASON);
    await shotTo(page, "23-csm-request-approval-disabled-nobody-to-ask");

    // The backend is the authority: asked anyway, it says why in the words the fake mirrors, and moves nothing.
    const refused = await staff("jane").patch(cr.id, { state: "assess" });
    expect([refused.status, message(refused)]).toEqual([400, nobodyToAskMessage(true, false)]);
    expect(await stateOf(cr.id)).toBe("new");
    // The same for the review box, alone or with the other.
    const second = await raise({ subject: lockSubject("nobody to ask, review only"), projectId: nobody!.id, approval: false, review: true });
    const refusedReview = await staff("jane").patch(second.id, { state: "assess" });
    expect([refusedReview.status, message(refusedReview)]).toEqual([400, nobodyToAskMessage(false, true)]);
    // ...while clearing the box in the same PATCH leaves nobody to ask for, and is accepted.
    expect((await staff("jane").patch(second.id, { customerReviewRequired: false, state: "assess" })).status).toBe(200);
    expect(await stateOf(second.id)).toBe("assess");

    // In New the box can be cleared in the dialog: the block lifts and Request Approval goes through.
    await detail.openEditDialog();
    await detail.editCustomerApprovalCheckbox().uncheck();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    await expect(blocked).toHaveCount(0);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    expect(await stateOf(cr.id)).toBe("assess");

    // After New an unticked box can still be added until its gate, but not when nobody on the project can be asked: the dialog does
    // not pre-empt the backend, and shows its words when the save is refused.
    await detail.openEditDialog();
    await expect(detail.editCustomerReviewCheckbox()).toBeEnabled();
    await detail.editCustomerReviewCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog().getByRole("alert")).toContainText(nobodyToAskMessage(false, true));
    await shotTo(page, "24-csm-edit-dialog-tick-on-refused-nobody-to-ask");
    await expect(detail.editDialog()).toBeVisible();
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();
    for (const [field, words] of [
      ["customerApprovalRequired", nobodyToAskMessage(true, false)],
      ["customerReviewRequired", nobodyToAskMessage(false, true)],
    ] as const) {
      const tick = await staff("jane").patch(cr.id, { [field]: true });
      expect([tick.status, message(tick)], field).toEqual([400, words]);
    }
    const both = await staff("jane").patch(cr.id, { customerApprovalRequired: true, customerReviewRequired: true });
    expect([both.status, message(both)]).toEqual([400, nobodyToAskMessage(true, true)]);
    const stored = (await staff("jane").get(cr.id)).body;
    expect([stored.state, stored.customerApprovalRequired, stored.customerReviewRequired]).toEqual(["assess", false, false]);
  });

  test("after Request Approval the Customer Project is read-only in the dialog and refused by the API; a ticked box is read-only and cannot be unticked; an unticked one can be added, once, and then cannot be removed; resending what is stored is accepted", async ({ page }) => {
    await pictureWindow(page);
    const cr = await raise({ subject: lockSubject("frozen"), projectId: EXAMPLE_CORP.id, approval: true, review: false });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");
    expect(await stateOf(cr.id)).toBe("assess");

    // The dialog: the project read-only with why, the ticked box read-only with why, the unticked one open, with its warning.
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editDialog().getByText(PROJECT_FROZEN_REASON)).toBeVisible();
    const approval = await requirementBox(detail, "approval");
    await expect(approval.checkbox).toBeChecked();
    await expect(approval.checkbox).toBeDisabled();
    await expect(approval.text).toHaveText(REQUIREMENT_ADD_ONLY_REASON);
    const review = await requirementBox(detail, "review");
    await expect(review.checkbox).not.toBeChecked();
    await expect(review.checkbox).toBeEnabled();
    await expect(review.text).toContainText(REQUIREMENT_ONCE_SAVED);
    await shotTo(page, "22-csm-edit-dialog-assess-project-frozen-box-add-only");
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();

    // The API: every refusal is a readable 400 and a refused PATCH writes nothing, not even the rest of its own request.
    const jane = staff("jane");
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    const swap = await jane.patch(cr.id, { projectId: OTHER_CORP.id });
    expect([swap.status, message(swap)]).toEqual([400, projectFrozenMessage("assess")]);
    const untick = await jane.patch(cr.id, { customerApprovalRequired: false });
    expect([untick.status, message(untick)]).toEqual([400, requirementCannotBeRemovedMessage("customerApprovalRequired", "assess")]);
    const back = await jane.patch(cr.id, { state: "new" });
    expect([back.status, message(back)]).toEqual([400, CANNOT_RETURN_TO_NEW]);
    const mixed = await jane.patch(cr.id, { title: lockSubject("renamed by a refused PATCH"), projectId: OTHER_CORP.id });
    expect(mixed.status).toBe(400);
    const after = (await jane.get(cr.id)).body;
    expect([after.project?.id, after.customerApprovalRequired, after.title ?? after.subject]).toEqual([EXAMPLE_CORP.id, true, lockSubject("frozen")]);

    // A client that resends the whole form is not punished: the stored values are accepted.
    const resend = await jane.patch(cr.id, { projectId: EXAMPLE_CORP.id, customerApprovalRequired: true, customerReviewRequired: false });
    expect(resend.status, JSON.stringify(resend.body)).toBe(200);

    // The unticked box can still be added (in the dialog), exactly once: then it is read-only too.
    await page.reload();
    await detail.openEditDialog();
    await detail.editCustomerReviewCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    expect((await jane.get(cr.id)).body.customerReviewRequired).toBe(true);
    await detail.openEditDialog();
    await expect(detail.editCustomerReviewCheckbox()).toBeChecked();
    await expect(detail.editCustomerReviewCheckbox()).toBeDisabled();
    await expect((await requirementBox(detail, "review")).text).toHaveText(REQUIREMENT_ADD_ONLY_REASON);
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();
    const untickReview = await jane.patch(cr.id, { customerReviewRequired: false });
    expect([untickReview.status, message(untickReview)]).toEqual([400, requirementCannotBeRemovedMessage("customerReviewRequired", "assess")]);

    // Corrections are Cancel and Clone: the Clone action is offered, and the change can still be cancelled.
    await expect(detail.cloneButton()).toBeVisible();
    expect((await jane.patch(cr.id, { state: "canceled" })).status).toBe(200);
  });

  test("every state, at the API, with the backend's own words: the project is frozen after New, a box is add-only until its gate, and past the gate nothing can be added", async () => {
    // A box is ticked or not, the project is there or not; each row is one change request put into the state through the real flow.
    const jane = staff("jane");
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    const states: Array<{ state: WalkTarget; approvalAddable: boolean; reviewAddable: boolean }> = [
      { state: "assess", approvalAddable: true, reviewAddable: true },
      { state: "authorize", approvalAddable: true, reviewAddable: true },
      { state: "scheduled", approvalAddable: false, reviewAddable: true },
      { state: "implement", approvalAddable: false, reviewAddable: true },
      { state: "review", approvalAddable: false, reviewAddable: true },
      { state: "closed", approvalAddable: false, reviewAddable: false },
      { state: "canceled", approvalAddable: false, reviewAddable: false },
    ];
    for (const row of states) {
      // Two changes per state: one to add the approval box to, one the review box (each can be added once).
      const forApproval = await raise({ subject: lockSubject(`${row.state}, add approval`), projectId: EXAMPLE_CORP.id, approval: false, review: false });
      const forReview = await raise({ subject: lockSubject(`${row.state}, add review`), projectId: EXAMPLE_CORP.id, approval: false, review: false });
      await walkTo(forApproval, row.state);
      await walkTo(forReview, row.state);
      expect(await stateOf(forApproval.id), row.state).toBe(row.state);

      // The project: frozen in every one of them (a swap is a readable 400, the stored one resent is accepted).
      const swap = await jane.patch(forApproval.id, { projectId: OTHER_CORP.id });
      expect([swap.status, message(swap)], `${row.state}: swap the project`).toEqual([400, projectFrozenMessage(row.state)]);
      expect((await jane.patch(forApproval.id, { projectId: EXAMPLE_CORP.id })).status, `${row.state}: resend the project`).toBe(200);
      // Back to New: never.
      const toNew = await jane.patch(forApproval.id, { state: "new" });
      expect(toNew.status, `${row.state}: back to New`).toBe(400);
      // A finished change is refused as every other request to move it is; any other says it cannot return to New.
      expect(message(toNew), `${row.state}: back to New`).toBe(row.state === "closed" || row.state === "canceled" ? finalStateMessage("new", row.state) : CANNOT_RETURN_TO_NEW);

      // Adding a box: allowed until its gate, a 400 in the gate's words after it.
      const addApproval = await jane.patch(forApproval.id, { customerApprovalRequired: true });
      if (row.approvalAddable) expect(addApproval.status, `${row.state}: add the approval box ${JSON.stringify(addApproval.body)}`).toBe(200);
      else expect([addApproval.status, message(addApproval)], `${row.state}: add the approval box`).toEqual([400, requirementGatePassedMessage("customerApprovalRequired", row.state)]);
      const addReview = await jane.patch(forReview.id, { customerReviewRequired: true });
      if (row.reviewAddable) expect(addReview.status, `${row.state}: add the review box ${JSON.stringify(addReview.body)}`).toBe(200);
      else expect([addReview.status, message(addReview)], `${row.state}: add the review box`).toEqual([400, requirementGatePassedMessage("customerReviewRequired", row.state)]);

      // Once added, never removed (where it could be added).
      if (row.approvalAddable) {
        const remove = await jane.patch(forApproval.id, { customerApprovalRequired: false });
        expect([remove.status, message(remove)], `${row.state}: remove the approval box`).toEqual([400, requirementCannotBeRemovedMessage("customerApprovalRequired", row.state)]);
      }
      if (row.reviewAddable) {
        const remove = await jane.patch(forReview.id, { customerReviewRequired: false });
        expect([remove.status, message(remove)], `${row.state}: remove the review box`).toEqual([400, requirementCannotBeRemovedMessage("customerReviewRequired", row.state)]);
      }
    }
  });

  test("a change with no Customer Project can never take a customer box after New: the API says so and the dialog shows both boxes disabled with the reason", async ({ page }) => {
    await pictureWindow(page);
    const cr = await raise({ subject: lockSubject("no project"), projectId: null, approval: false, review: false });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await detail.requestApproval();
    await expect(detail.currentStep()).toContainText("Assess");

    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    for (const field of ["customerApprovalRequired", "customerReviewRequired"] as const) {
      const refused = await staff("jane").patch(cr.id, { [field]: true });
      expect([refused.status, message(refused)], field).toEqual([400, requirementNeedsProjectMessage(field)]);
    }
    // ...nor can a project be given to it now.
    const give = await staff("jane").patch(cr.id, { projectId: EXAMPLE_CORP.id });
    expect([give.status, message(give)]).toEqual([400, projectFrozenMessage("assess")]);

    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeDisabled();
    for (const box of ["approval", "review"] as const) {
      const row = await requirementBox(detail, box);
      await expect(row.checkbox, `${box} box`).toBeDisabled();
      await expect(row.text, `${box} reason`).toHaveText(REQUIREMENT_NEEDS_PROJECT_REASON);
    }
    await shotTo(page, "23-csm-edit-dialog-assess-no-project-boxes-disabled");
  });

  test("a Re-schedule asks the customer again and the change stays in Customer Approval: no Authorize, no CAB; the box stays ticked and the project cannot be swapped, and the customer's own answer then schedules it", async ({ page }) => {
    await pictureWindow(page);
    const cr = await raise({ subject: lockSubject("re-schedule"), projectId: EXAMPLE_CORP.id, approval: true, review: false });
    const jane = staff("jane");
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    await ok("Request Approval", await jane.patch(cr.id, { state: "assess" }));
    await ok("Peer approval", await staff("alice").decide(cr.id, "approved"));
    await ok("CAB approval", await staff("alice").decide(cr.id, "approved"));
    expect(await stateOf(cr.id)).toBe("customer_approval");
    const customerRows = async () =>
      (await psqlOutput(
        "select a.state from approval_stage s join approval_stage_approver a on a.stage_id = s.id " +
          `where s.work_item_id = '${cr.id}' and s.checkpoint_label = 'Customer Approval' order by s.created_on, s.id, a.id`,
      ))
        .split("\n")
        .filter(Boolean);
    const stageCount = async () => Number(await psqlOutput(`select count(*) from approval_stage where work_item_id = '${cr.id}'`));
    expect(await customerRows()).toEqual(["REQUESTED", "REQUESTED"]);
    const stagesBefore = await stageCount();

    // Re-schedule (a new window), the way the Re-schedule dialog sends it.
    const start = new Date(Date.now() + 9 * 86_400_000);
    start.setUTCHours(10, 0, 0, 0);
    const iso = (d: Date) => d.toISOString().slice(0, 19).replace("T", " ");
    const end = new Date(start.getTime() + 2 * 3_600_000);
    await ok("Re-schedule", await staff("alice").patch(cr.id, { state: "authorize", plannedStartOn: iso(start), plannedEndOn: iso(end) }));
    // The state does not move and no CAB stage is opened: the change itself has not changed.
    expect(await stateOf(cr.id)).toBe("customer_approval");
    expect(await customerRows(), "the customers' request is cancelled, not deleted, and asked again in a fresh stage").toEqual(["CANCELLED", "CANCELLED", "REQUESTED", "REQUESTED"]);
    expect(await stageCount(), "one new stage: the customers'").toBe(stagesBefore + 1);

    // A ticked box can never be unticked, and the project is frozen: nothing customer-related is open to a Re-schedule either.
    const untick = await jane.patch(cr.id, { customerApprovalRequired: false });
    expect([untick.status, message(untick)]).toEqual([400, requirementCannotBeRemovedMessage("customerApprovalRequired", "customer_approval")]);
    const swap = await jane.patch(cr.id, { projectId: OTHER_CORP.id });
    expect([swap.status, message(swap)]).toEqual([400, projectFrozenMessage("customer_approval")]);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    await expect(detail.proposalBanner()).toHaveCount(0);
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editCustomerApprovalCheckbox()).toBeChecked();
    await expect(detail.editCustomerApprovalCheckbox()).toBeDisabled();
    await shotTo(page, "24-csm-edit-dialog-customer-approval-after-reschedule-nothing-customer-related-editable");
    await detail.editDialog().getByRole("button", { name: "Cancel" }).click();

    // The customer's answer (applied as the customer portal does) schedules it: nobody at WSO2 had anything to approve.
    const answered = await decideAsCustomer(cr.id, "dave.mendis@example.com", "approved");
    expect(answered.status, answered.body).toBe(200);
    expect(await stateOf(cr.id)).toBe("scheduled");
  });

  test("a required customer review cannot be skipped by unticking it to close from Review: the box cannot be removed, and Review offers Send for customer review, never Close", async ({ page }) => {
    const cr = await raise({ subject: lockSubject("review required"), projectId: EXAMPLE_CORP.id, approval: false, review: true });
    const jane = staff("jane");
    await walkTo(cr, "review");
    expect(await stateOf(cr.id)).toBe("review");
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    const untick = await jane.patch(cr.id, { customerReviewRequired: false });
    expect([untick.status, message(untick)]).toEqual([400, requirementCannotBeRemovedMessage("customerReviewRequired", "review")]);
    const close = await staff("alice").patch(cr.id, { state: "closed" });
    expect(close.status, "closing a change whose customer review is required").toBe(400);
    expect(await stateOf(cr.id)).toBe("review");
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await expect(detail.sendForCustomerReviewButton()).toBeVisible();
    await expect(detail.closeButton()).toHaveCount(0);
  });

  test("a canceled, a closed and a rolled-back change request have no exit: every state by every request is refused in words that name both states and nothing moves; naming the state it is in is no move", async () => {
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    const finals: Array<{ state: "canceled" | "closed" | "rollback"; reach: (c: Raised) => Promise<void> }> = [
      { state: "canceled", reach: (c) => walkTo(c, "canceled") },
      { state: "closed", reach: (c) => walkTo(c, "closed") },
      {
        state: "rollback",
        reach: async (c) => {
          await walkTo(c, "review");
          await ok("Roll back", await staff("alice").patch(c.id, { state: "rollback" }));
        },
      },
    ];
    for (const final of finals) {
      const cr = await raise({ subject: lockSubject(`final ${final.state}`), projectId: EXAMPLE_CORP.id, approval: false, review: false });
      await final.reach(cr);
      expect(await stateOf(cr.id), final.state).toBe(final.state);
      for (const target of EVERY_STATE) {
        const answer = await staff("jane").patch(cr.id, { state: target });
        const label = `${final.state} -> ${target}`;
        if (target === final.state && target !== "rollback") {
          expect(answer.status, `${label}: naming the state it is in is no move`).toBe(200);
        } else if (target === "rollback" && final.state === "rollback") {
          // Rollback names itself, and "rollback" is only ever set from a review state.
          expect([answer.status, message(answer)], label).toEqual([400, 'state "rollback" can only be set from review or customer_review']);
        } else {
          expect([answer.status, message(answer)], label).toEqual([400, finalStateMessage(target, final.state)]);
        }
        expect(await stateOf(cr.id), `${label} moved the change`).toBe(final.state);
      }
      // A Re-schedule is a state change too, and a refused PATCH writes nothing of its own request.
      const re = await staff("alice").patch(cr.id, { state: "authorize", plannedStartOn: "2031-01-05 10:00:00", plannedEndOn: "2031-01-05 12:00:00" });
      expect([re.status, message(re)], `${final.state}: Re-schedule`).toEqual([400, finalStateMessage("authorize", final.state)]);
      expect(await stateOf(cr.id)).toBe(final.state);
    }
  });

  test("no jump over a state or an approval gate, in the backend's words: with both customer boxes ticked, Implement, Review, Customer Review and Close are refused from New, Assess and Authorize, and out of Customer Approval every destination is the customer's own answer", async () => {
    const message = (r: ApiResult) => (r.body as { message?: string }).message;
    const jane = staff("jane");
    const cr = await raise({ subject: lockSubject("no jumps"), projectId: EXAMPLE_CORP.id, approval: true, review: true });
    const expectNoJump = async (from: string): Promise<void> => {
      expect(await stateOf(cr.id)).toBe(from);
      for (const target of ["implement", "review", "customer_review", "closed"]) {
        const answer = await jane.patch(cr.id, { state: target });
        const want = from === "customer_approval" ? customerAnswerRefusal(target, false, "customer_approval") : stateJumpMessage(target, from, true);
        expect([from, target, answer.status, message(answer)], `${from} -> ${target}`).toEqual([from, target, 400, want]);
        expect(await stateOf(cr.id), `${from} -> ${target} moved the change`).toBe(from);
      }
    };
    await expectNoJump("new");
    await ok("Request Approval", await jane.patch(cr.id, { state: "assess" }));
    await expectNoJump("assess");
    // Request Approval is not a jump out of anywhere else either.
    const again = await jane.patch(cr.id, { state: "assess" });
    expect(again.status, "a resent Request Approval is no move").toBe(200);
    await ok("Peer approval", await staff("alice").decide(cr.id, "approved"));
    await expectNoJump("authorize");
    await ok("CAB approval", await staff("alice").decide(cr.id, "approved"));
    // The legal path still works, and the customer is asked rather than skipped.
    await expectNoJump("customer_approval");
  });
});

// ---------------------------------------------------------------------------
// MIGRATED (legacy) change requests in the CSM portal, on the real stack. The customer portal's own spec
// (customer-change-request-legacy.spec.ts) proves what CUSTOMERS see of them; this is the staff's side: the rows the
// sync writes -- no customer-stage rows, customer flags false, an approval stage with no label -- must not lock WSO2 out
// of their own change requests. The rows are the customer portal e2e's fixture (fixtures/legacy-change-requests.sql, ids
// and numbers in the migrated shape, created in 1999), written and removed here through the stack's Postgres.
// ---------------------------------------------------------------------------

const LEGACY_SQL = path.resolve(process.cwd(), "../../customer-portal/webapp/tests/e2e/fixtures/legacy-change-requests.sql");
const legacyId = (number: string): string => {
  const hex = createHash("md5").update(`legacy-${number}`).digest("hex");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
};
async function deleteLegacy(): Promise<void> {
  await psql("delete from work_item where created_by = 'sn-sync' and number ~ '^CHG0039[1234]'");
}

test.describe("migrated (legacy) change requests in the CSM portal (real stack)", () => {
  test.describe.configure({ timeout: 180_000 });
  test.beforeEach(async () => {
    test.skip(!realStackNamed(), "name the stack under test: E2E_CSM_BFF_URL, E2E_OIDC_URL, E2E_ENTITY_SERVICE_URL, E2E_POSTGRES_CONTAINER (see auth/README.md)");
    test.skip(!fs.existsSync(LEGACY_SQL), `the customer portal e2e fixture ${LEGACY_SQL} is not there (run from apps/csm-portal/webapp)`);
    await psql(fs.readFileSync(LEGACY_SQL, "utf8"));
  });
  test.afterAll(async () => {
    if (realStackNamed()) await deleteLegacy();
  });

  test("an Emergency change in Authorize whose only approval stage was synced with no label: its pending approver sees Approve in the Approvals tab, decides, and the change is Scheduled (it used to be refused as a stale Peer stage)", async ({ browser }) => {
    const id = legacyId("CHG0039301");
    await asPersona(browser, "crInternalApprover", async (page) => {
      await pictureWindow(page);
      const detail = new ChangeRequestDetailPage(page);
      await detail.goto(id);
      await expect(detail.currentStep()).toContainText("Authorize");
      await expect(detail.approverStatus(ALICE)).toHaveText("Requested");
      await expect(detail.approverStatus(BOB)).toHaveText("Requested");
      // Only the signed-in user's own pending row offers Approve / Reject.
      await expect(detail.approveButton(ALICE)).toBeVisible();
      await expect(detail.approveButton(BOB)).toHaveCount(0);
      await shotTo(page, "30-csm-legacy-synced-stage-pending-approver-can-decide");
      await detail.approve(ALICE);
      await expect(detail.currentStep()).toContainText("Scheduled");
      await expect(detail.approverStatus(ALICE)).toHaveText("Approved");
    });
  });

  test("a legacy change request sitting in Customer Approval with nobody asked: the Approvals tab says so, and staff cannot answer for the customer (Re-schedule and Cancel change only; the API refuses {state: scheduled})", async ({ page }) => {
    await pictureWindow(page);
    const detail = new ChangeRequestDetailPage(page);
    const id = legacyId("CHG0039104");
    await detail.goto(id);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
    // The project has registered contacts, but no request was ever made of them (a change migrated from the previous system
    // sitting at the gate): the note says nobody is asked, not that no contacts are registered, and that Cancel change is the
    // only way out of Customer Approval.
    await expect(page.getByText(NOBODY_ASKED_TEXT)).toBeVisible();
    await expect(page.getByText(CANCEL_ONLY_WAY_OUT)).toBeVisible();
    await expect(page.getByText(NO_CUSTOMER_GROUP_TEXT)).toHaveCount(0);
    await expectActionBar(detail, { primary: null, reschedule: true, menu: ["Cancel change"] });
    await expectNoBypass(detail);
    await shotTo(page, "31-csm-legacy-customer-approval-nobody-asked");
    // Nobody was asked, and still the backend will not record the customer's approval, from any caller.
    const refused = await patchState(page, id, "scheduled");
    expect([refused.status, refused.message]).toEqual([400, customerAnswerRefusal("scheduled")]);
    expect(await stateOf(id)).toBe("customer_approval");
  });

  test("a legacy change request sitting in Customer Review with nobody asked: the note names Roll back or Cancel change as the ways out, Roll back is enabled, there is no Close, and the API refuses {state: closed}", async ({ page }) => {
    await pictureWindow(page);
    const detail = new ChangeRequestDetailPage(page);
    const id = legacyId("CHG0039108");
    await detail.goto(id);
    await expect(detail.currentStep()).toContainText("Customer Review");
    await expect(page.getByText(NOBODY_ASKED_TEXT)).toBeVisible();
    await expect(page.getByText(ROLL_BACK_OR_CANCEL_WAYS_OUT)).toBeVisible();
    await expect(detail.closeButton()).toHaveCount(0);
    await expectActionBar(detail, { primary: null, reschedule: false, menu: ["Roll back", "Cancel change"] });
    await detail.openChangeStateMenu();
    await expect(detail.rollbackMenuItem()).toBeEnabled(); // nobody is asked, so nothing holds it back
    await detail.closeChangeStateMenu();
    await expectNoBypass(detail);
    await shotTo(page, "33-csm-legacy-customer-review-nobody-asked");
    const refused = await patchState(page, id, "closed");
    expect([refused.status, refused.message]).toEqual([400, customerAnswerRefusal("closed")]);
    expect(await stateOf(id)).toBe("customer_review");
  });

  test("Re-schedule on a legacy change in Customer Approval whose requirement box is false (migration 0189 defaulted it) asks the customer again and the change stays in Customer Approval: no CAB, the box is not written, and only the customer's own answer schedules it", async () => {
    const id = legacyId("CHG0039104");
    const alice = staff("alice");
    expect((await staff("jane").get(id)).body.customerApprovalRequired, "a migrated row's own requirement column is false").toBeFalsy();
    const stageCount = async () => Number(await psqlOutput(`select count(*) from approval_stage where work_item_id = '${id}'`));
    expect(await stageCount(), "a migrated row nobody was asked about has no stage").toBe(0);

    const start = new Date(Date.now() + 9 * 86_400_000);
    start.setUTCHours(10, 0, 0, 0);
    const iso = (d: Date) => d.toISOString().slice(0, 19).replace("T", " ");
    await ok("Re-schedule", await alice.patch(id, { state: "authorize", plannedStartOn: iso(start), plannedEndOn: iso(new Date(start.getTime() + 2 * 3_600_000)) }));
    expect(await stateOf(id), "no CAB loop: the change itself has not changed").toBe("customer_approval");
    // Our own requirement column is a sync-owned shape a Re-schedule must not touch; the customer is asked all the same (a fresh stage).
    expect((await staff("jane").get(id)).body.customerApprovalRequired).toBeFalsy();
    expect(await stageCount(), "the contacts are asked in one fresh customer stage, and no CAB stage").toBe(1);
    expect(
      await psqlOutput(`select s.checkpoint_label || ':' || a.state from approval_stage s join approval_stage_approver a on a.stage_id = s.id where s.work_item_id = '${id}' order by a.id`),
    ).toBe("Customer Approval:REQUESTED\nCustomer Approval:REQUESTED");

    const answered = await decideAsCustomer(id, "dave.mendis@example.com", "approved");
    expect(answered.status, answered.body).toBe(200);
    expect(await stateOf(id)).toBe("scheduled");
  });

  test("a legacy change request in Scheduled with its flags false and a Customer Project: the Edit dialog freezes the project, closes the approval box (its gate is passed) and still lets the review box be added", async ({ page }) => {
    await pictureWindow(page);
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(legacyId("CHG0039105"));
    await expect(detail.currentStep()).toContainText("Scheduled");
    await detail.openEditDialog();
    await expect(detail.editProjectField()).toBeDisabled();
    await expect(detail.editDialog().getByText(PROJECT_FROZEN_REASON)).toBeVisible();
    await expect(detail.editCustomerApprovalCheckbox()).toBeDisabled();
    await expect(detail.editDialog().getByText(/^Locked: the change request has already reached the customer approval step or later\./)).toBeVisible();
    await expect(detail.editCustomerReviewCheckbox()).toBeEnabled();
    await expect(detail.editDialog().getByText(REQUIREMENT_ONCE_SAVED)).toBeVisible();
    await shotTo(page, "32-csm-legacy-scheduled-edit-dialog");
    await detail.editCustomerReviewCheckbox().check();
    await detail.saveEdit();
    await expect(detail.editDialog()).toHaveCount(0);
    expect((await staff("jane").get(legacyId("CHG0039105"))).body.customerReviewRequired).toBe(true);
    // ...and then it stays: add-only holds for a migrated row exactly as for a native one.
    const untick = await staff("jane").patch(legacyId("CHG0039105"), { customerReviewRequired: false });
    expect(untick.status).toBe(400);
  });
});

// ---------------------------------------------------------------------------
// A customer's PROPOSED TIME, on the REAL stack (the CSM webapp, the CSM backend, entity-service as csm_app with row-level
// security, the database). The previous system's own mechanism: the customer's proposal is written to customer_updated_on (a planned
// START) and nothing else, the change STAYS in Customer Approval with the planned window untouched, and WSO2 answers it from the
// change request: Accept proposed time (Scheduled in one step, no CAB, no new customer request) or Propose a different time
// (the customer is asked again; keeping the window declines). The customer proposes the way the customer portal does (a PATCH
// to entity-service with their own token, utils/customerPortalDecision.ts); every database assertion is on the rows the
// backend wrote, so "nothing else was written" is checked where it matters.
// ---------------------------------------------------------------------------

const PROPOSAL_PREFIX = "E2E proposal: ";
const proposalSubject = (title: string): string => `${PROPOSAL_PREFIX}${title}`;
async function deleteProposalChanges(): Promise<void> {
  await psql(`delete from work_item where type = 'CHANGE_REQUEST' and subject like '${PROPOSAL_PREFIX}%'`);
}

/** The columns a proposal reads and writes, as the database holds them (UTC, "YYYY-MM-DD HH:MM:SS"). */
async function proposalRow(id: string): Promise<{ state: string; start: string; end: string; proposed: string; answer: string; stamped: string }> {
  const utc = (col: string): string => `coalesce(to_char(${col} at time zone 'utc', 'YYYY-MM-DD HH24:MI:SS'), '')`;
  const out = await psqlOutput(
    `select state::text, ${utc("start_on")}, ${utc("end_on")}, ${utc("customer_updated_on")}, ` +
      `coalesce(customer_updated_date_confirmation::text, ''), coalesce(is_customer_approval_required::text, '') from change_request where id = '${id}'`,
  );
  const [state, start, end, proposed, answer, stamped] = out.split("|");
  return { state: state!, start: start!, end: end!, proposed: proposed!, answer: answer!, stamped: stamped! };
}

/** Row counts of the tables a proposal must not grow, for a change request. */
async function writtenRows(id: string): Promise<{ stages: number; approvers: number; comments: number }> {
  const count = async (sql: string): Promise<number> => Number(await psqlOutput(sql));
  return {
    stages: await count(`select count(*) from approval_stage where work_item_id = '${id}'`),
    approvers: await count(`select count(*) from approval_stage_approver where work_item_id = '${id}'`),
    comments: await count(`select count(*) from comment where work_item_id = '${id}'`),
  };
}

const utcIso = (d: Date): string => d.toISOString().slice(0, 19).replace("T", " ");
/** A start `days` days ahead at `hour`:00 UTC. */
function startIn(days: number, hour = 10): Date {
  const d = new Date(Date.now() + days * 86_400_000);
  d.setUTCHours(hour, 0, 0, 0);
  return d;
}

test.describe("a customer's proposed time (real stack)", () => {
  test.describe.configure({ timeout: 240_000 });
  test.beforeEach(async () => {
    test.skip(
      !realStackNamed(),
      "name the stack under test: E2E_CSM_BFF_URL, E2E_OIDC_URL, E2E_ENTITY_SERVICE_URL, E2E_POSTGRES_CONTAINER (see auth/README.md)",
    );
    await deleteProposalChanges();
  });
  test.afterAll(async () => {
    if (realStackNamed()) await deleteProposalChanges();
  });

  /** A Normal change on Example Corp (dave and erin are asked), planned two weeks out for two hours, brought to Customer Approval. */
  async function atCustomerApproval(title: string): Promise<{ cr: Raised; planned: { start: Date; end: Date } }> {
    const cr = await raise({ subject: proposalSubject(title), projectId: EXAMPLE_CORP.id, approval: true, review: false });
    const start = startIn(14);
    const planned = { start, end: new Date(start.getTime() + 2 * 3_600_000) };
    await plan(cr, { start: utcIso(planned.start), end: utcIso(planned.end) });
    await ok("Request Approval", await staff("jane").patch(cr.id, { state: "assess" }));
    await ok("Peer approval", await staff("alice").decide(cr.id, "approved"));
    await ok("CAB approval", await staff("alice").decide(cr.id, "approved"));
    expect(await stateOf(cr.id)).toBe("customer_approval");
    return { cr, planned };
  }

  test("Dave proposes a start: the change waits in Customer Approval with its window untouched and nothing else written; the banner names him, and Accept proposed time schedules it by the proposal, with no CAB, no new request and nothing stamped as the customer's approval", async ({ page }) => {
    await pictureWindow(page);
    const { cr, planned } = await atCustomerApproval("accept");
    const before = await writtenRows(cr.id);
    const proposedStart = new Date(planned.start.getTime() + 7 * 86_400_000);

    const proposal = await proposeAsCustomer(cr.id, "dave.mendis@example.com", { plannedStartOn: utcIso(proposedStart) });
    expect(proposal.status, proposal.body).toBe(200);
    // The previous system's own columns hold it; the plan, the state and every stage and approver row are what they were.
    const waiting = await proposalRow(cr.id);
    expect(waiting).toMatchObject({ state: "CUSTOMER_APPROVAL", start: utcIso(planned.start), end: utcIso(planned.end), proposed: utcIso(proposedStart), answer: "" });
    const afterProposal = await writtenRows(cr.id);
    expect({ stages: afterProposal.stages, approvers: afterProposal.approvers }, "a proposal writes no stage and no approver row").toEqual({ stages: before.stages, approvers: before.approvers });

    // What WSO2 reads: the backend's own verdict, Dave named as the proposer, Accept on offer.
    const read = (await staff("jane").get(cr.id)).body as unknown as { customerProposal?: Record<string, unknown>; hasCustomerApproved?: boolean };
    expect(read.customerProposal).toMatchObject({ answer: "pending", proposerRecorded: true, proposedByEmail: "dave.mendis@example.com", canAccept: true });
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await expect(detail.currentStep()).toContainText("Customer Approval");
    await expect(detail.proposalBanner()).toBeVisible();
    await expect(detail.customerProposalBanner()).toBeVisible(); // the proposer is on record
    await expect(detail.proposalBanner()).toContainText("Proposed by Dave Mendis (dave.mendis@example.com)");
    await expect(detail.proposalBanner()).toContainText("Same length as the planned window (2 hours)");
    await expect(detail.proposalWaitingReason()).toBeVisible();
    await expect(detail.acceptProposedTimeButton()).toBeEnabled();
    await shotTo(page, "40-csm-proposal-banner-real-stack");

    // Accept, from the page.
    await detail.acceptProposedTimeButton().click();
    await expect(detail.acceptDialog()).toBeVisible();
    await shotTo(page, "41-csm-accept-proposed-time-dialog-real-stack");
    await detail.acceptDialogConfirm().click();
    await expect(detail.acceptDialog()).toHaveCount(0);
    await expect(detail.currentStep()).toContainText("Scheduled");

    const accepted = await proposalRow(cr.id);
    expect(accepted, "the proposal is the plan, its length kept, the answer is AGREE, the change is Scheduled").toMatchObject({
      state: "SCHEDULED",
      start: utcIso(proposedStart),
      end: utcIso(new Date(proposedStart.getTime() + 2 * 3_600_000)),
      answer: "AGREE",
    });
    expect(accepted.stamped, "no staff action records the customer's approval").not.toBe("true");
    const after = await writtenRows(cr.id);
    expect({ stages: after.stages, approvers: after.approvers }, "no CAB stage, no second request").toEqual({ stages: before.stages, approvers: before.approvers });
    expect(
      (await psqlOutput(`select count(*) from approval_stage_approver where work_item_id = '${cr.id}' and state = 'REQUESTED'`)),
      "the customers' own request is closed like any state change closes it",
    ).toBe("0");
    const scheduled = (await staff("jane").get(cr.id)).body as unknown as { hasCustomerApproved?: boolean; customerProposal?: { answer?: string } };
    expect(scheduled.hasCustomerApproved).toBeFalsy();
    expect(scheduled.customerProposal?.answer).toBe("agreed");
    await expect(detail.proposalBanner()).toHaveCount(0);
    await detail.page.getByRole("tab", { name: "Approval" }).click();
    await expect(detail.overviewCell("Customer approved")).toContainText("Proposed time accepted");
    await shotTo(page, "42-csm-after-accept-real-stack");
    // Accept is another door whose only precondition is the customer's own proposal: a manual scheduled stays refused.
    const refused = await staff("alice").patch(cr.id, { state: "scheduled" });
    expect(refused.status).toBe(400);
  });

  test("Propose a different time asks the customer again (no CAB) and answers the proposal DISAGREE; a decline keeps the window and the customers' live request; the customer's next proposal brings the banner back", async ({ page }) => {
    await pictureWindow(page);
    const { cr, planned } = await atCustomerApproval("counter and decline");
    const before = await writtenRows(cr.id);
    const first = new Date(planned.start.getTime() + 7 * 86_400_000);
    expect((await proposeAsCustomer(cr.id, "dave.mendis@example.com", { plannedStartOn: utcIso(first) })).status).toBe(200);
    const pendingVersion = ((await staff("jane").get(cr.id)).body as unknown as { customerProposal?: { startOn?: string } }).customerProposal?.startOn;
    expect(pendingVersion).toBeTruthy();

    // A Re-schedule that names no proposal while one waits is refused: an old client never answers a time it did not see.
    const blind = await staff("alice").patch(cr.id, { state: "authorize", plannedEndOn: utcIso(new Date(planned.end.getTime() + 3_600_000)) });
    expect(blind.status).toBe(409);
    expect((blind.body as { message?: string }).message).toBe(customerProposedWhileOpenMessage(pendingVersion!));

    // The counter: WSO2's own window, with the proposal and the plan it was shown.
    const counterStart = new Date(planned.start.getTime() + 3 * 86_400_000);
    const counter = await staff("alice").patch(cr.id, {
      state: "authorize",
      plannedStartOn: utcIso(counterStart),
      plannedEndOn: utcIso(new Date(counterStart.getTime() + 2 * 3_600_000)),
      expectedCustomerUpdatedOn: pendingVersion,
      expectedPlannedStartOn: planned.start.toISOString(),
      expectedPlannedEndOn: planned.end.toISOString(),
    });
    expect(counter.status, JSON.stringify(counter.body)).toBe(200);
    expect(await proposalRow(cr.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", start: utcIso(counterStart), proposed: utcIso(first), answer: "DISAGREE" });
    const afterCounter = await writtenRows(cr.id);
    expect(afterCounter.stages, "one new stage, the customers' fresh request: no CAB").toBe(before.stages + 1);
    expect(
      await psqlOutput(`select count(*) from approval_stage_approver a join approval_stage s on s.id = a.stage_id where s.work_item_id = '${cr.id}' and s.checkpoint_label = 'Customer Approval' and a.state = 'REQUESTED'`),
    ).toBe("2");
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(cr.id);
    await expect(detail.proposalBanner()).toHaveCount(0);
    await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");

    // The customer proposes again (their next proposal clears the answer): the banner is back, and WSO2 declines it, keeping the window.
    const second = new Date(counterStart.getTime() + 5 * 86_400_000);
    expect((await proposeAsCustomer(cr.id, "erin.jayawardena@example.com", { plannedStartOn: utcIso(second) })).status).toBe(200);
    await page.reload();
    await expect(detail.proposalBanner()).toBeVisible();
    await expect(detail.proposalBanner()).toContainText("Proposed by Erin Jayawardena (erin.jayawardena@example.com)");
    const live = await writtenRows(cr.id);
    await detail.proposeDifferentTimeButton().click();
    await expect(detail.counterDialog()).toBeVisible();
    await expect(detail.counterSubmit("Decline proposed time")).toBeEnabled();
    await detail.counterSubmit("Decline proposed time").click();
    await expect(detail.counterDialog()).toHaveCount(0);
    expect(await proposalRow(cr.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", start: utcIso(counterStart), proposed: utcIso(second), answer: "DISAGREE" });
    expect(await writtenRows(cr.id), "a decline writes the answer and nothing else").toEqual(live);
    await expect(detail.proposalBanner()).toHaveCount(0);
  });

  test("on the rows the sync writes: a migrated change in Customer Approval with a customer date and no answer is a stored time nobody is recorded as having proposed: Accept is refused (page and API), and Propose a different time is a plain Re-schedule that writes no answer", async ({ page }) => {
    test.skip(!fs.existsSync(LEGACY_SQL), `the customer portal e2e fixture ${LEGACY_SQL} is not there (run from apps/csm-portal/webapp)`);
    await psql(fs.readFileSync(LEGACY_SQL, "utf8"));
    try {
      const detail = new ChangeRequestDetailPage(page);
      // CHG0039112: Customer Approval, a window planned (2031-03-01 10:00 - 12:00), nobody asked (no stage). The sync wrote a customer date, no answer.
      const id = legacyId("CHG0039112");
      await psql(`update change_request set customer_updated_on = '2031-03-08 10:00:00+00', customer_updated_date_confirmation = NULL where id = '${id}'`);
      const read = (await staff("jane").get(id)).body as unknown as { customerProposal?: Record<string, unknown> };
      // The last writer is the sync, not a registered contact: nobody is named, and Accept is not on offer, with the reason.
      expect(read.customerProposal).toMatchObject({ answer: "pending", startOn: "2031-03-08T10:00:00Z", proposerRecorded: false, canAccept: false });
      expect(read.customerProposal?.acceptBlockedReason).toBe(ACCEPT_PROPOSER_NOT_RECORDED);
      expect(read.customerProposal?.proposedByEmail).toBeUndefined();

      await detail.goto(id);
      await expect(detail.neutralProposalBanner()).toBeVisible(); // nobody on record: the banner does not say the customer proposed it
      await expect(detail.proposalWaitingReason()).toHaveCount(0);
      await expect(detail.blockingReason()).toHaveText("Awaiting Customer Approval");
      await expect(detail.neutralProposalBanner()).toContainText(") but nobody is recorded as having proposed it.");
      await expect(detail.acceptProposedTimeButton()).toBeDisabled();
      await expect(detail.proposeDifferentTimeButton()).toBeEnabled();
      await shotTo(page, "43-csm-stored-time-nobody-recorded-migrated-row");

      // The API refuses an Accept sent anyway: a 409 with its code, and nothing is written.
      const accept = await staff("alice").patch(id, {
        confirmCustomerUpdatedDate: "agree",
        expectedCustomerUpdatedOn: "2031-03-08T10:00:00Z",
        expectedPlannedStartOn: "2031-03-01T10:00:00Z",
        expectedPlannedEndOn: "2031-03-01T12:00:00Z",
      });
      expect(accept.status).toBe(409);
      expect(accept.body as { message?: string; errorCode?: string }).toMatchObject({ message: ACCEPT_PROPOSER_NOT_RECORDED, errorCode: "change_request_proposer_not_recorded" });
      // ... and a request with no window is no decline of a time nobody proposed.
      const bare = await staff("alice").patch(id, { state: "authorize" });
      expect([bare.status, (bare.body as { message?: string }).message]).toEqual([400, NO_RECORDED_PROPOSAL_TO_DECLINE]);
      expect(await proposalRow(id)).toMatchObject({ state: "CUSTOMER_APPROVAL", start: "2031-03-01 10:00:00", answer: "" });

      // Propose a different time: a plain Re-schedule (the window must change). The customers are asked, nothing is answered.
      await detail.proposeDifferentTimeButton().click();
      await expect(detail.counterDialog()).toContainText("There is no proposal to decline.");
      await expect(detail.counterSubmit("Propose this time")).toBeDisabled();
      await detail.fillRescheduleWindow("Planned start", NEXT_WEEK_START, detail.counterDialog());
      await detail.fillRescheduleWindow("Planned end", NEXT_WEEK_END, detail.counterDialog());
      await detail.counterSubmit("Propose this time").click();
      await expect(detail.counterDialog()).toHaveCount(0);
      expect(await proposalRow(id), "no DISAGREE is written against a date nobody proposed").toMatchObject({ state: "CUSTOMER_APPROVAL", answer: "" });
      expect(
        await psqlOutput(`select count(*) from approval_stage_approver a join approval_stage s on s.id = a.stage_id where s.work_item_id = '${id}' and s.checkpoint_label = 'Customer Approval' and a.state = 'REQUESTED'`),
        "the project's contacts are asked to approve the new time",
      ).toBe("2");
    } finally {
      await deleteLegacy();
    }
  });

  test("a date on a migrated change that is NOT a proposal waiting for WSO2 is never read as one: one waiting on an unlabeled approval in Authorize, one whose date is the planned start, one with an answer", async () => {
    test.skip(!fs.existsSync(LEGACY_SQL), `the customer portal e2e fixture ${LEGACY_SQL} is not there (run from apps/csm-portal/webapp)`);
    await psql(fs.readFileSync(LEGACY_SQL, "utf8"));
    try {
      const message = (r: ApiResult) => (r.body as { message?: string }).message;
      const answerOf = async (id: string): Promise<string | undefined> =>
        ((await staff("jane").get(id)).body as unknown as { customerProposal?: { answer?: string } }).customerProposal?.answer;

      // CHG0039301: an Emergency change in Authorize, its one synced stage unlabeled and REQUESTED (alice, bob). A stale customer date on it.
      const authorize = legacyId("CHG0039301");
      await psql(`update change_request set customer_updated_on = '2031-03-08 10:00:00+00' where id = '${authorize}'`);
      expect(await answerOf(authorize)).toBe("unanswered");
      const accept = await staff("alice").patch(authorize, {
        confirmCustomerUpdatedDate: "agree",
        expectedCustomerUpdatedOn: "2031-03-08T10:00:00Z",
        expectedPlannedStartOn: "2031-03-01T10:00:00Z",
        expectedPlannedEndOn: "2031-03-01T12:00:00Z",
      });
      expect([accept.status, message(accept)]).toEqual([409, acceptNotInCustomerApproval("authorize")]);
      expect(await stateOf(authorize)).toBe("authorize");

      // CHG0039112 with the date equal to the planned start (applied already): nothing to answer.
      const applied = legacyId("CHG0039112");
      await psql(`update change_request set customer_updated_on = start_on, customer_updated_date_confirmation = NULL where id = '${applied}'`);
      expect(await answerOf(applied)).toBe("unanswered");

      // ... and with a standing answer (a Disagree written in the previous system): history, not a proposal.
      await psql(`update change_request set customer_updated_on = '2031-03-08 10:00:00+00', customer_updated_date_confirmation = 'DISAGREE' where id = '${applied}'`);
      expect(await answerOf(applied)).toBe("disagreed");
      const again = await staff("alice").patch(applied, {
        confirmCustomerUpdatedDate: "agree",
        expectedCustomerUpdatedOn: "2031-03-08T10:00:00Z",
        expectedPlannedStartOn: "2031-03-01T10:00:00Z",
        expectedPlannedEndOn: "2031-03-01T12:00:00Z",
      });
      expect([again.status, message(again)]).toEqual([409, NO_PROPOSAL_WAITING]);
      expect(await stateOf(applied)).toBe("customer_approval");
    } finally {
      await deleteLegacy();
    }
  });
  test("a customer date the sync left far ahead is a stored time nobody proposed: Accept is refused for that reason first, whatever window it would give, in the read model and at the API, and nothing is written", async () => {
    test.skip(!fs.existsSync(LEGACY_SQL), `the customer portal e2e fixture ${LEGACY_SQL} is not there (run from apps/csm-portal/webapp)`);
    await psql(fs.readFileSync(LEGACY_SQL, "utf8"));
    try {
      const id = legacyId("CHG0039112"); // Customer Approval, planned 2031-03-01 10:00 - 12:00 (2 hours), nobody asked
      const shown = { expectedPlannedStartOn: "2031-03-01T10:00:00Z", expectedPlannedEndOn: "2031-03-01T12:00:00Z" };
      const proposalOf = async (): Promise<Record<string, unknown> | undefined> =>
        ((await staff("jane").get(id)).body as unknown as { customerProposal?: Record<string, unknown> }).customerProposal;

      // One whose 2-hour window would end after the year 2100, and the last one that fits: neither is accepted, since the sync's
      // date has no registered contact recorded as its proposer. That refusal is the first reason, ahead of the window's own.
      for (const start of ["2100-12-31 22:30:00+00", "2100-12-31 21:30:00+00"]) {
        await psql(`update change_request set customer_updated_on = '${start}', customer_updated_date_confirmation = NULL where id = '${id}'`);
        expect(await proposalOf()).toMatchObject({ answer: "pending", proposerRecorded: false, canAccept: false, acceptBlockedReason: ACCEPT_PROPOSER_NOT_RECORDED });
        const startIso = start.replace(" ", "T").replace("+00", "Z");
        const refused = await staff("alice").patch(id, { confirmCustomerUpdatedDate: "agree", expectedCustomerUpdatedOn: startIso, ...shown });
        expect([refused.status, (refused.body as { message?: string }).message]).toEqual([409, ACCEPT_PROPOSER_NOT_RECORDED]);
        expect(await proposalRow(id)).toMatchObject({ state: "CUSTOMER_APPROVAL", start: "2031-03-01 10:00:00", end: "2031-03-01 12:00:00", answer: "" });
      }
    } finally {
      await deleteLegacy();
    }
  });
});

test.describe("seeded fixtures (local stack) — create with an assignment group", () => {
  test("a team picked from the Assignment group picker is saved on create (no FK 400)", async ({ page }) => {
    test.setTimeout(60_000);

    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();
    await cr.selectType("Normal");
    await cr.subjectField().fill(`[E2E] local create with assignment group ${new Date().toISOString()}`);

    const group = page.getByRole("combobox", { name: /^Assignment group/ });
    await group.fill("Apollo");
    await page.getByRole("option", { name: /Apollo/ }).first().click();

    const [response] = await Promise.all([
      page.waitForResponse((r) => r.request().method() === "POST" && /\/change-requests$/.test(r.url())),
      cr.createButton().click(),
    ]);
    expect(response.status(), await response.text()).toBe(201);
    await expect(page).toHaveURL(/\/operations\/change-requests\/(?!new(?:[/?#]|$))[^/]+$/, { timeout: 15_000 });
  });
});
