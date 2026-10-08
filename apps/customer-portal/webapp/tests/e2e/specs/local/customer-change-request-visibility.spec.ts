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
// WHO SEES A CHANGE REQUEST, over its whole life, on the real local stack.
//
// A customer sees a change request exactly when it was DESIGNATED to them: it reached
// Customer Approval and/or Customer Review and they were one of the registered contacts
// asked (they hold, or ever held, an approver row on a Customer Approval / Customer Review
// stage of it), and it is still on a project they are a registered contact of. Once
// designated it stays visible in every later state. A change request that never asked the
// customer, one that has not got there yet and one asked of other contacts only are not
// there at all: absent from the list, the stat cards and the dashboard count, and a 404 on
// the detail, the approvals and every write, whatever address the customer types.
//
// Every change request here is RAISED THROUGH THE CSM PORTAL'S BACKEND by WSO2 staff
// (jane raises, alice and bob approve in the CAB's seats) and walked through the real
// approval flow, so the rows the rule reads are the ones the product writes, not seeded ones.
// The customers act in the customer portal's own UI (mira proposes and approves, noel reviews); WSO2 answers a
// proposal through the CSM portal's backend (bob counters it).
// What is asserted, at every step of the life, is what mira, noel and dave (another project)
// really get: the list, the stat cards' counts (the numbers themselves, against what the
// project showed before the change request was raised), the dashboard's Outstanding count,
// the detail, the approvals and a refused write. Pictures of the key screens go to
// E2E_SHOT_DIR when it is set.
//
//   1. Both boxes ticked, on Lumen Works Platform (mira, noel): invisible in New, Assess and
//      Authorize; visible to BOTH contacts the moment it reaches Customer Approval; mira
//      proposes a new start and the change STAYS in Customer Approval, visible to mira AND
//      noel (list, detail, the counts: nothing about the change moved); WSO2 answers with a
//      window of its own, the contacts are asked again, mira approves, and it is Scheduled,
//      Implement, Review, Customer Review (noel answers), Closed: visible all the way. dave
//      (project 401) never sees it, by API or by the address he would type.
//   2. A change request that never needs the customer is never visible, in any state through
//      Closed (Normal and Standard; an Emergency change never asks the customer at all, and a
//      customer box cannot even be ticked on it); one that needs only the customer's REVIEW is invisible
//      until Customer Review and visible from there on.
//   3. A contact registered AFTER the stage was provisioned was never asked: they see nothing,
//      in any state of that change request, though they are a registered contact of the project.
//   4. The approvals a customer reads hold the customer's rows with their names and the internal
//      stages as a label and a status only: no staff name, e-mail or group.
//
// ⚠️ STATE-CHANGING: raises change requests and re-seeds, so it needs E2E_POSTGRES_CONTAINER and
// E2E_CSM_BFF_URL and SKIPS without them. It removes what it raised before and after.
//

import { test, expect, type Page } from "../../fixtures/test";
import { LOCAL_PERSONAS, openLocalContext, withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import {
  EXAMPLE_CORP_ABT_GROUP_ID,
  FIXTURES,
  LATE_CONTACT,
  RAISED_PREFIX,
  approverRows,
  customerApi,
  countsWith,
  customerApiFor,
  customerCounts,
  deleteRaisedChanges,
  entityAsCustomer,
  entityServiceUrl,
  futureWindow,
  lumenProjectId,
  planWindow,
  proposalRow,
  psql,
  raiseChange,
  registerLateContact,
  removeLateContact,
  requestApproval,
  resetFixtures,
  shot,
  staffApi,
  staffCountersProposal,
  staffDecides,
  staffMoves,
  storedState,
  withFixtureStack,
  type Counts,
  type RaisedChange,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "mira");
withFixtureStack(test, true);

const BROWSER_ZONE = "America/New_York";
test.use({ timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 } });

/** An id no change request has. */
const NO_SUCH_CHANGE_REQUEST = "00000000-0000-0000-0000-00000000dead";

/**
 * Asserts what mira and noel get of `change` RIGHT NOW over the API: listed / not, the stat cards and the
 * dashboard count (against `bases`, what they said before it was raised), the detail and the approvals (200 with the
 * state `label`, or 404), and, when it is hidden, that a write is a 404 as well (a visible one is never poked: an
 * answer would move it). dave, of another project, gets nothing of it, ever.
 */
async function expectSeenBy(
  change: RaisedChange,
  lumen: string,
  visibleTo: Array<"mira" | "noel">,
  label: string | null,
  bases: Record<"mira" | "noel", Counts>,
  when: string,
): Promise<void> {
  for (const who of ["mira", "noel"] as const) {
    const api = customerApi(who);
    const visible = visibleTo.includes(who);
    const here = `${change.number} (${when}) for ${who}`;    // (it exists, and WSO2's staff see it in every state: what is hidden is hidden from the customer, not gone)
    expect((await staffApi("alice").get(change.id)).status, `${here}: staff read`).toBe(200);


    const item = (await api.listed(lumen)).find((c) => c.number === change.number);
    expect(!!item, `${here}: listed`).toBe(visible);
    const read = await api.get(change.id);
    expect(read.status, `${here}: detail ${JSON.stringify(read.body)}`).toBe(visible ? 200 : 404);
    expect((await api.approvals(change.id)).status, `${here}: approvals`).toBe(visible ? 200 : 404);
    if (visible) {
      expect(item?.state?.label, `${here}: state in the list`).toBe(label);
      expect(read.body.state?.label, `${here}: state on the detail`).toBe(label);
    } else {
      // a hidden change request is not writable either, and says the same 404 an id that does not exist says
      expect((await api.patch(change.id, { isCustomerApproved: true })).status, `${here}: answer`).toBe(404);
      // (a field no customer may set is the customer backend's own 403, for an id that exists and one that does not alike:
      // it is no oracle; entity-service itself answers 404, see the direct calls in the first test)
      const nothing = await api.patch(NO_SUCH_CHANGE_REQUEST, { title: "x" });
      expect((await api.patch(change.id, { title: "x" })).status, `${here}: a field no customer may set answers as for an id that does not exist`).toBe(nothing.status);
      expect((await api.decision(change.id, { decision: "approved" })).status, `${here}: decision`).toBe(404);
      // ...nor commentable through the case route, which the customer backend forwards for any id
      expect((await api.commentViaCaseRoute(change.id)).status, `${here}: comment through the case route`).toBe(404);
      // The case-like routes that take any work item id answer for it exactly as for an id that exists nowhere (no oracle),
      // and the global search knows nothing of it.
      expect(await api.caseRouteAnswers(change.id), `${here}: the case routes`).toEqual(await api.caseRouteAnswers(NO_SUCH_CHANGE_REQUEST));
      expect(await api.globalSearchMentions(change.number), `${here}: global search by number`).toBe(false);
      expect(await api.globalSearchMentions(change.title), `${here}: global search by title`).toBe(false);
    }

    // The numbers on the stat cards and the dashboard are the list's, not a wider set.
    const now = await customerCounts(LOCAL_PERSONAS[who].email, lumen);
    expect(now, `${here}: counts`).toEqual(countsWith(bases[who], visible ? label : null));
  }

  // dave (Example Corp) can never see a Lumen change request: not by id, not in his own project's list, not under Lumen's id.
  const dave = customerApi("dave");
  const daveHere = `${change.number} (${when}) for dave`;
  expect((await dave.get(change.id)).status, `${daveHere}: detail`).toBe(404);
  expect((await dave.approvals(change.id)).status, `${daveHere}: approvals`).toBe(404);
  expect((await dave.patch(change.id, { isCustomerApproved: true })).status, `${daveHere}: answer`).toBe(404);
  expect((await dave.commentViaCaseRoute(change.id)).status, `${daveHere}: comment through the case route`).toBe(404);
  expect(await dave.listedNumbers(FIXTURES.projectId), `${daveHere}: his own list`).not.toContain(change.number);
  expect(await dave.listedNumbers(lumen), `${daveHere}: Lumen's list`).toEqual([]);
}

/**
 * The Operations hub's "Outstanding Change Requests / Latest 5 change requests" list, as the customer's page shows it:
 * waits for the search that fills it, then says whether `change` is among them.
 */
async function expectOnHub(page: Page, lumen: string, change: RaisedChange, listed: boolean, when: string): Promise<void> {
  const [search] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/change-requests/search") && r.request().method() === "POST", { timeout: 60_000 }),
    page.goto(`/projects/${lumen}/operations`),
  ]);
  await search.finished();
  await expect(page.getByText("Latest 5 change requests")).toBeVisible({ timeout: 60_000 });
  await page.waitForTimeout(600); // let the answer render
  await expect(page.getByText(change.number, { exact: true }), `${change.number} on the Operations hub (${when})`).toHaveCount(listed ? 1 : 0);
}

test.describe("Local stack — who sees a change request, over its whole life", () => {
  test.describe.configure({ timeout: 360_000 });

  test.beforeEach(async () => {
    await deleteRaisedChanges();
    await removeLateContact();
    await resetFixtures();
  });
  test.afterAll(async () => {
    await deleteRaisedChanges();
    await removeLateContact();
  });

  test(`both boxes ticked on Lumen Works Platform: invisible in New, Assess and Authorize; visible to ${LOCAL_PERSONAS.mira.email} AND ${LOCAL_PERSONAS.noel.email} at Customer Approval; still visible, in Customer Approval, after mira proposes a new start and WSO2 answers it, and in every state through Closed; never to dave`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const lumen = await lumenProjectId();
    const bases = { mira: (await customerCounts(LOCAL_PERSONAS.mira.email, lumen))!, noel: (await customerCounts(LOCAL_PERSONAS.noel.email, lumen))! };
    expect(bases.mira, "the two contacts of one project see the same counts").toEqual(bases.noel);

    const noelContext = await openLocalContext(test, browser, "noel", {
      baseURL, timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 },
    });
    const daveContext = await openLocalContext(test, browser, "dave", {
      baseURL, timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 },
    });
    try {
      const change = await raiseChange({ title: "Lumen, both boxes", projectId: lumen, approval: true, review: true });
      // A proposal moves the planned START, so the change needs a planned window (a raised one has none).
      const planned = futureWindow("UTC", { daysAhead: 20, startHour: 10, hours: 2 });
      await planWindow(change.id, planned);
      const miraPage = new ChangeRequestDetailsPage(page);
      const noelBrowserPage = await noelContext.newPage();
      const noelPage = new ChangeRequestDetailsPage(noelBrowserPage);
      const daveBrowserPage = await daveContext.newPage();
      const davePage = new ChangeRequestDetailsPage(daveBrowserPage);
      const miraList = new ChangeRequestsPage(page);
      const both = ["mira", "noel"] as const;

      // ---- New: raised by WSO2 with both boxes ticked, nobody asked, nothing to see.
      expect(await storedState(change.id)).toBe("NEW");
      await expectSeenBy(change, lumen, [], null, bases, "New");
      await miraList.open(lumen);
      await miraList.waitForList();
      await expect(miraList.rowByNumber(change.number), "New in mira's list").toHaveCount(0);
      await miraList.searchFor(change.number); // not even by its number
      await expect(miraList.rowByNumber(change.number), "New found by its number in mira's list").toHaveCount(0);
      await miraList.clearSearch(); // the list remembers its last search: the later phases look at the whole list
      const hidden = await miraPage.openAndSettle(lumen, change.id, change.number);
      expect(hidden.loaded, "New rendered for mira").toBe(false);
      await expect(miraPage.answerButtons()).toHaveCount(0);
      await shot(page, "01-mira-new-by-address-not-found");
      await expectOnHub(page, lumen, change, false, "New");
      // Straight at entity-service, as the customer backend would forward it (its own machine token, mira's ID token): the
      // service itself says 404 to a read, to the approvals, to an answer and to a field no customer may set.
      if (entityServiceUrl()) {
        for (const [method, route, body] of [
          ["GET", `/change-requests/${change.id}`, undefined],
          ["GET", `/change-requests/${change.id}/approvals`, undefined],
          ["PATCH", `/change-requests/${change.id}`, { isCustomerApproved: true }],
          ["PATCH", `/change-requests/${change.id}`, { title: "x" }],
          ["POST", `/change-requests/${change.id}/approvals/decision`, { decision: "approved" }],
        ] as const) {
          const direct = await entityAsCustomer(LOCAL_PERSONAS.mira.email, method, route, body);
          expect(direct.status, `entity-service ${method} ${route} ${JSON.stringify(body ?? "")}: ${JSON.stringify(direct.body)}`).toBe(404);
        }
      }

      // ---- Assess (Request Approval): the peer approval is asked of WSO2's staff only.
      const requested = await requestApproval(change.id);
      expect(requested.status, JSON.stringify(requested.body)).toBe(200);
      expect(await storedState(change.id)).toBe("ASSESS");
      await expectSeenBy(change, lumen, [], null, bases, "Assess");
      expect((await noelPage.openAndSettle(lumen, change.id, change.number)).loaded, "Assess rendered for noel").toBe(false);

      // ---- Authorize: the peer approved, the CAB is asked.
      await staffDecides("alice", change.id);
      expect(await storedState(change.id)).toBe("AUTHORIZE");
      await expectSeenBy(change, lumen, [], null, bases, "Authorize, CAB asked");
      expect((await miraPage.openAndSettle(lumen, change.id, change.number)).loaded, "Authorize rendered for mira").toBe(false);
      expect(await approverRows(change.id).then((rows) => rows.filter((r) => r.stage.startsWith("Customer")))).toEqual([]);

      // ---- Customer Approval: the CAB approved, so the customer's stage is provisioned for BOTH contacts.
      await staffDecides("alice", change.id);
      expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");
      expect((await approverRows(change.id)).filter((r) => r.stage === "Customer Approval").map((r) => `${r.email}|${r.state}`)).toEqual([
        `${LOCAL_PERSONAS.mira.email}|REQUESTED`,
        `${LOCAL_PERSONAS.noel.email}|REQUESTED`,
      ]);
      await expectSeenBy(change, lumen, [...both], "Customer Approval", bases, "Customer Approval");
      await miraList.open(lumen);
      await miraList.waitForList();
      await expect(miraList.rowByNumber(change.number)).toHaveCount(1);
      await expect(miraList.rowByNumber(change.number)).toContainText("Customer Approval");
      await miraPage.open(lumen, change.id, change.number);
      await expect(miraPage.currentStage()).toHaveText(UI.stages.customerApproval);
      for (const name of [UI.buttons.approve, UI.buttons.reject, UI.buttons.proposeNewTime]) {
        await expect(miraPage.button(name), `${name} for mira`).toBeVisible();
      }
      await shot(page, "02-mira-customer-approval-detail");
      await expectOnHub(page, lumen, change, true, "Customer Approval");
      await shot(page, "02b-mira-operations-hub-customer-approval");
      await noelPage.open(lumen, change.id, change.number);
      await expect(noelPage.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(noelPage.button(UI.buttons.approve)).toBeVisible();
      // ...and dave, at the address he would type for it, gets the project refused (it is not his).
      const lumenAddress = `/projects/${lumen}/operations/change-requests/${change.id}`;
      await davePage.openAndSettle(FIXTURES.projectId, change.id, change.number);
      await expect(davePage.answerButtons(), "dave offered an answer on another project's change").toHaveCount(0);
      await daveBrowserPage.goto(lumenAddress);
      await expect(davePage.answerButtons()).toHaveCount(0);
      await expect(daveBrowserPage.getByText(change.number, { exact: true })).toHaveCount(0);
      await shot(daveBrowserPage, "03-dave-lumen-address-refused");

      // ---- mira proposes a new START: the change STAYS in Customer Approval, visible to both, and nothing about it moved.
      await miraPage.open(lumen, change.id, change.number);
      await miraPage.button(UI.buttons.proposeNewTime).click();
      await expect(miraPage.proposeDialog()).toBeVisible();
      const zone = await miraPage.proposeTimeZone();
      const proposal = futureWindow(zone, { daysAhead: 24, startHour: 16, hours: 2 });
      await miraPage.fillProposedStart(proposal.start);
      await miraPage.submitProposalButton().click();
      await expect(miraPage.banner(UI.banners.proposed)).toBeVisible({ timeout: 20_000 });
      await expect(miraPage.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(miraPage.proposalWaitingNote()).toContainText(UI.notes.waitingOwn);
      expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");
      expect(await proposalRow(change.id)).toEqual({ proposedUtc: proposal.startUtc, answer: "" });
      expect(
        (await approverRows(change.id)).filter((r) => r.stage === "Customer Approval").map((r) => `${r.email}|${r.state}`),
        "the customers' requests stand: a proposal answers nothing and cancels nothing",
      ).toEqual([`${LOCAL_PERSONAS.mira.email}|REQUESTED`, `${LOCAL_PERSONAS.noel.email}|REQUESTED`]);
      await expectSeenBy(change, lumen, [...both], "Customer Approval", bases, "Customer Approval after mira's proposal");
      await miraList.open(lumen);
      await miraList.waitForList();
      await expect(miraList.rowByNumber(change.number), "mira lost it from her list after proposing").toHaveCount(1);
      await expect(miraList.rowByNumber(change.number)).toContainText("Customer Approval");
      await shot(page, "04-mira-list-customer-approval-after-proposal");
      // The State filter offers Authorize (and never New or Assess), and filtering by Customer Approval lists exactly the designated change request.
      await miraList.openFilters();
      const stateOptions = await miraList.stateFilterOptions();
      expect(stateOptions, "the State filter").toContain("Authorize");
      expect(stateOptions).not.toContain("New");
      expect(stateOptions).not.toContain("Assess");
      await miraList.filterByState("Customer Approval");
      await expect(miraList.rowByNumber(change.number), "mira's Customer Approval filter").toHaveCount(1);
      await shot(page, "04a-mira-list-state-filter-customer-approval");
      await miraList.clearFilters(); // the list remembers its filters: the later phases look at the whole list
      // The Calendar view is the same list in a month grid: the designated change request's window (the planned one) is on it.
      await miraList.openView("Calendar View");
      await expect(page.getByText(change.title, { exact: false }).first(), "the planned window on the calendar").toBeVisible({ timeout: 30_000 });
      await shot(page, "04c-mira-calendar-view-customer-approval-after-proposal");
      await miraList.openView("List View");
      await expectOnHub(page, lumen, change, true, "Customer Approval after the proposal (outstanding for a customer)");
      await shot(page, "04b-mira-operations-hub-customer-approval-after-proposal");
      // noel did not propose: he is told neutrally, and may still answer.
      await noelPage.open(lumen, change.id, change.number);
      await expect(noelPage.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(noelPage.proposalWaitingNote()).toContainText(UI.notes.waitingOther);
      await expect(noelPage.button(UI.buttons.approve), "noel offered Approve while the proposal waits").toBeVisible();
      await shot(noelBrowserPage, "05-noel-detail-customer-approval-after-proposal");
      expect((await customerApi("noel").get(change.id)).body.customerCanAnswer).toBe(true);
      // The same time proposed again is refused in words (it already waits), and moves nothing.
      const refused = await customerApi("mira").patch(change.id, { plannedStartOn: proposal.startUtc });
      expect(refused.status, `a second proposal of the same time: ${JSON.stringify(refused.body)}`).toBe(400);
      expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");

      // ---- WSO2 (bob) answers with a window of its own: still Customer Approval, a FRESH stage asked of both. Still visible to both.
      const wso2 = futureWindow("UTC", { daysAhead: 26, startHour: 9, hours: 3 });
      const countered = await staffCountersProposal("bob", change.id, wso2);
      expect(countered.status, JSON.stringify(countered.body)).toBe(200);
      expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");
      expect((await approverRows(change.id)).filter((r) => r.stage === "Customer Approval").map((r) => `${r.email}|${r.state}`)).toEqual([
        `${LOCAL_PERSONAS.mira.email}|CANCELLED`,
        `${LOCAL_PERSONAS.noel.email}|CANCELLED`,
        `${LOCAL_PERSONAS.mira.email}|REQUESTED`,
        `${LOCAL_PERSONAS.noel.email}|REQUESTED`,
      ]);
      await expectSeenBy(change, lumen, [...both], "Customer Approval", bases, "Customer Approval again");
      await noelPage.open(lumen, change.id, change.number);
      await expect(noelPage.proposalNotAcceptedNote(), "noel is told the proposal was not accepted").toBeVisible();
      await expect(noelPage.button(UI.buttons.approve), "noel asked again").toBeVisible();

      // ---- mira approves in the UI: Scheduled, and both still see it.
      await miraPage.open(lumen, change.id, change.number);
      await miraPage.button(UI.buttons.approve).click();
      await expect(miraPage.banner(UI.banners.approved)).toBeVisible();
      await expect(miraPage.currentStage()).toHaveText(UI.stages.scheduled);
      expect(await storedState(change.id)).toBe("SCHEDULED");
      await expectSeenBy(change, lumen, [...both], "Scheduled", bases, "Scheduled");
      await shot(page, "06-mira-detail-scheduled");

      // ---- WSO2 implements and reviews it: still visible, nothing to answer.
      for (const [moveTo, stored, label, stage] of [
        ["implement", "IMPLEMENT", "Implement", UI.stages.implement],
        ["review", "REVIEW", "Review", UI.stages.review],
      ] as const) {
        await staffMoves("alice", change.id, moveTo);
        expect(await storedState(change.id)).toBe(stored);
        await expectSeenBy(change, lumen, [...both], label, bases, label);
        await noelPage.open(lumen, change.id, change.number);
        await expect(noelPage.currentStage()).toHaveText(stage);
        await expect(noelPage.answerButtons()).toHaveCount(0);
      }

      // ---- Customer Review: both are asked; noel answers in the UI -> Closed.
      await staffMoves("alice", change.id, "customer_review");
      expect(await storedState(change.id)).toBe("CUSTOMER_REVIEW");
      expect((await approverRows(change.id)).filter((r) => r.stage === "Customer Review").map((r) => `${r.email}|${r.state}`)).toEqual([
        `${LOCAL_PERSONAS.mira.email}|REQUESTED`,
        `${LOCAL_PERSONAS.noel.email}|REQUESTED`,
      ]);
      await expectSeenBy(change, lumen, [...both], "Customer Review", bases, "Customer Review");
      await noelPage.open(lumen, change.id, change.number);
      await expect(noelPage.button(UI.buttons.successful)).toBeVisible();
      await noelPage.button(UI.buttons.successful).click();
      await expect(noelPage.banner(UI.banners.markedSuccessful)).toBeVisible();
      await expect(noelPage.currentStage()).toHaveText(UI.stages.closed);
      expect(await storedState(change.id)).toBe("CLOSED");

      // ---- Closed: the end of its life, and still theirs.
      await expectSeenBy(change, lumen, [...both], "Closed", bases, "Closed");
      await miraList.open(lumen);
      await miraList.waitForList();
      await expect(miraList.rowByNumber(change.number)).toHaveCount(1);
      await expect(miraList.rowByNumber(change.number)).toContainText("Closed");
      await shot(page, "07-mira-list-closed");
      await expectOnHub(page, lumen, change, false, "Closed: no longer outstanding");
      await miraPage.open(lumen, change.id, change.number);
      await expect(miraPage.currentStage()).toHaveText(UI.stages.closed);
      await expect(miraPage.answerButtons()).toHaveCount(0);
      expect(
        (await approverRows(change.id)).map((r) => `${r.stage}|${r.email}|${r.state}`).filter((r) => r.includes("mira") || r.includes("noel")),
        "every request ever made of the two contacts is still on record",
      ).toHaveLength(6);
    } finally {
      await noelContext.close();
      await daveContext.close();
    }
  });

  test("an unticked Normal and an unticked Standard change request are never visible to a customer, in any state through Closed", async () => {
    const lumen = await lumenProjectId();
    const bases = { mira: (await customerCounts(LOCAL_PERSONAS.mira.email, lumen))!, noel: (await customerCounts(LOCAL_PERSONAS.noel.email, lumen))! };

    // Normal, both boxes off: Peer, CAB, then straight to Scheduled; never asks anybody.
    const normal = await raiseChange({ title: "Lumen, nothing ticked (Normal)", projectId: lumen, approval: false, review: false });
    await expectSeenBy(normal, lumen, [], null, bases, "Normal, New");
    expect((await requestApproval(normal.id)).status).toBe(200);
    await expectSeenBy(normal, lumen, [], null, bases, "Normal, Assess");
    await staffDecides("alice", normal.id);
    await expectSeenBy(normal, lumen, [], null, bases, "Normal, Authorize");
    await staffDecides("alice", normal.id);
    expect(await storedState(normal.id)).toBe("SCHEDULED");
    await expectSeenBy(normal, lumen, [], null, bases, "Normal, Scheduled (a state customers have always seen, nobody was asked)");
    for (const [moveTo, stored] of [["implement", "IMPLEMENT"], ["review", "REVIEW"], ["closed", "CLOSED"]] as const) {
      await staffMoves("alice", normal.id, moveTo);
      expect(await storedState(normal.id)).toBe(stored);
      await expectSeenBy(normal, lumen, [], null, bases, `Normal, ${stored}`);
    }
    expect((await approverRows(normal.id)).filter((r) => r.stage.startsWith("Customer")), "nobody was ever asked").toEqual([]);

    // Standard, both boxes off: Request Approval goes straight to Scheduled.
    const standard = await raiseChange({ title: "Lumen, nothing ticked (Standard)", projectId: lumen, approval: false, review: false, type: "standard" });
    expect((await requestApproval(standard.id)).status).toBe(200);
    expect(await storedState(standard.id)).toBe("SCHEDULED");
    await expectSeenBy(standard, lumen, [], null, bases, "Standard, Scheduled");
    await staffMoves("alice", standard.id, "implement");
    await staffMoves("alice", standard.id, "review");
    await staffMoves("alice", standard.id, "closed");
    await expectSeenBy(standard, lumen, [], null, bases, "Standard, Closed");

    // And one raised for NOBODY (no Customer Project) never reaches anyone either.
    const noProject = await raiseChange({ title: "no Customer Project, nothing ticked", projectId: null, approval: false, review: false });
    expect((await requestApproval(noProject.id)).status).toBe(200);
    await staffDecides("alice", noProject.id);
    await staffDecides("alice", noProject.id);
    expect(await storedState(noProject.id)).toBe("SCHEDULED");
    expect((await customerApi("mira").get(noProject.id)).status).toBe(404);
    expect((await customerApi("dave").get(noProject.id)).status).toBe(404);
    expect(await customerApi("dave").listedNumbers(FIXTURES.projectId)).not.toContain(noProject.number);
  });

  test("a Standard change with Customer Approval ticked goes straight to Customer Approval at Request Approval, where both contacts see it (and a proposal keeps it there, as it does every change); an Emergency change never asks the customer: a ticked box is refused at create, and one raised without is invisible through its CAB approval", async () => {
    const lumen = await lumenProjectId();
    const bases = { mira: (await customerCounts(LOCAL_PERSONAS.mira.email, lumen))!, noel: (await customerCounts(LOCAL_PERSONAS.noel.email, lumen))! };

    const standard = await raiseChange({ title: "Lumen, Standard with approval", projectId: lumen, approval: true, review: false, type: "standard" });
    await expectSeenBy(standard, lumen, [], null, bases, "Standard, New");
    expect((await requestApproval(standard.id)).status).toBe(200);
    expect(await storedState(standard.id), "a Standard change has no internal approval").toBe("CUSTOMER_APPROVAL");
    await expectSeenBy(standard, lumen, ["mira", "noel"], "Customer Approval", bases, "Standard, Customer Approval");
    // mira proposes a new start: like every change, a Standard one stays in Customer Approval and nobody is asked again
    // (a proposal answers nothing, and WSO2 answers it)
    const window = futureWindow("UTC", { daysAhead: 7, startHour: 11, hours: 2 });
    await planWindow(standard.id, window);
    const rowsBefore = await approverRows(standard.id);
    const proposed = await customerApi("mira").patch(standard.id, { plannedStartOn: futureWindow("UTC", { daysAhead: 9, startHour: 11, hours: 2 }).startUtc });
    expect(proposed.status, JSON.stringify(proposed.body)).toBe(200);
    expect(await storedState(standard.id)).toBe("CUSTOMER_APPROVAL");
    expect(await approverRows(standard.id), "no request was touched").toEqual(rowsBefore);
    await expectSeenBy(standard, lumen, ["mira", "noel"], "Customer Approval", bases, "Standard, after a proposal");
    expect((await customerApi("noel").patch(standard.id, { isCustomerApproved: true })).status).toBe(200);
    expect(await storedState(standard.id)).toBe("SCHEDULED");
    await expectSeenBy(standard, lumen, ["mira", "noel"], "Scheduled", bases, "Standard, Scheduled");

    // An Emergency change acts without the customer's consent: neither box can be ticked on it (the backend refuses the create),
    // so nobody is ever designated and it is invisible to the project's contacts in every state.
    const refused = await staffApi("jane").create({
      subject: `${RAISED_PREFIX}Lumen, Emergency with approval`,
      type: "emergency",
      groupId: EXAMPLE_CORP_ABT_GROUP_ID,
      projectId: lumen,
      customerApprovalRequired: true,
      customerReviewRequired: false,
    });
    expect(refused.status, JSON.stringify(refused.body)).toBe(400);
    expect(JSON.stringify(refused.body)).toContain("Emergency changes proceed without customer consent");
    // (bases already hold the Standard one: its two states are what the counts now carry)
    const basesWithStandard = {
      mira: countsWith(bases.mira, "Scheduled"),
      noel: countsWith(bases.noel, "Scheduled"),
    };
    const emergency = await raiseChange({ title: "Lumen, Emergency", projectId: lumen, approval: false, review: false, type: "emergency" });
    expect((await requestApproval(emergency.id)).status).toBe(200);
    expect(await storedState(emergency.id), "an Emergency change goes straight to the CAB (there is no ECAB)").toBe("AUTHORIZE");
    await expectSeenBy(emergency, lumen, [], null, basesWithStandard, "Emergency, Authorize (the CAB asked)");
    await staffDecides("bob", emergency.id);
    expect(await storedState(emergency.id), "the CAB's approval schedules it: the customer is never asked").toBe("SCHEDULED");
    await expectSeenBy(emergency, lumen, [], null, basesWithStandard, "Emergency, Scheduled: nobody was ever asked");
  });

  test("only the customer's REVIEW ticked: invisible through Scheduled, Implement and Review (the box alone shows nothing), visible to both contacts from Customer Review on, and kept after Closed", async () => {
    const lumen = await lumenProjectId();
    const bases = { mira: (await customerCounts(LOCAL_PERSONAS.mira.email, lumen))!, noel: (await customerCounts(LOCAL_PERSONAS.noel.email, lumen))! };
    const change = await raiseChange({ title: "Lumen, review only", projectId: lumen, approval: false, review: true });
    expect((await requestApproval(change.id)).status).toBe(200);
    await staffDecides("alice", change.id);
    await staffDecides("alice", change.id);
    expect(await storedState(change.id)).toBe("SCHEDULED");
    for (const [moveTo, stored] of [["implement", "IMPLEMENT"], ["review", "REVIEW"]] as const) {
      await staffMoves("alice", change.id, moveTo);
      expect(await storedState(change.id)).toBe(stored);
      await expectSeenBy(change, lumen, [], null, bases, `review-only, ${stored}, the review box is ticked but nobody was asked`);
    }
    await staffMoves("alice", change.id, "customer_review");
    expect(await storedState(change.id)).toBe("CUSTOMER_REVIEW");
    await expectSeenBy(change, lumen, ["mira", "noel"], "Customer Review", bases, "review-only, Customer Review");

    // mira answers (through the API: the UI answers are covered above): Unsuccessful -> Rollback, still theirs.
    const answered = await customerApi("mira").patch(change.id, { isCustomerReviewed: false });
    expect(answered.status, JSON.stringify(answered.body)).toBe(200);
    expect(await storedState(change.id)).toBe("ROLLBACK");
    await expectSeenBy(change, lumen, ["mira", "noel"], "Rollback", bases, "review-only, Rollback");
  });

  test(`a contact registered AFTER the change request was put to the others (${LATE_CONTACT.email}) was never asked: they see nothing of it in any state, and the others still do`, async () => {
    const lumen = await lumenProjectId();
    const change = await raiseChange({ title: "Lumen, a late contact", projectId: lumen, approval: true, review: false });
    expect((await requestApproval(change.id)).status).toBe(200);
    await staffDecides("alice", change.id);
    await staffDecides("alice", change.id);
    expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");

    // Only now does Ozzy become a registered, portal-enabled contact of the project.
    await registerLateContact();
    const late = customerApiFor(LATE_CONTACT.email);
    const me = await late.projectIdByName(LOCAL_PERSONAS.mira.project);
    expect(me, "the late contact is a registered contact of Lumen Works Platform, and can list its projects").toBe(lumen);
    const lateBase = (await customerCounts(LATE_CONTACT.email, lumen))!;

    const watch = async (when: string) => {
      expect(await late.listedNumbers(lumen), `${change.number} (${when}) in the late contact's list`).not.toContain(change.number);
      expect((await late.get(change.id)).status, `${when}: detail`).toBe(404);
      expect((await late.approvals(change.id)).status, `${when}: approvals`).toBe(404);
      expect((await late.patch(change.id, { isCustomerApproved: true })).status, `${when}: answer`).toBe(404);
      expect((await late.decision(change.id, { decision: "approved" })).status, `${when}: decision`).toBe(404);
      expect(await customerCounts(LATE_CONTACT.email, lumen), `${when}: the late contact's counts`).toEqual(lateBase);
      // mira, who was asked, still has it
      expect((await customerApi("mira").get(change.id)).status, `${when}: mira`).toBe(200);
    };
    await watch("Customer Approval");
    // the stage was provisioned before Ozzy registered: no request exists for him, and none is made later
    expect((await approverRows(change.id)).filter((r) => r.email === LATE_CONTACT.email)).toEqual([]);

    // mira approves: Scheduled -> still not his; through to Closed.
    expect((await customerApi("mira").patch(change.id, { isCustomerApproved: true })).status).toBe(200);
    await watch("Scheduled");
    await staffMoves("alice", change.id, "implement");
    await staffMoves("alice", change.id, "review");
    await watch("Review");
    await staffMoves("alice", change.id, "closed");
    await watch("Closed");
    expect((await approverRows(change.id)).filter((r) => r.email === LATE_CONTACT.email)).toEqual([]);
    await removeLateContact();
  });

  test("designation is not enough on its own: a contact who is no longer REGISTERED on the project sees nothing (and sees it again when registered), and a change request the sync moved to another project is theirs no more and nobody's there", async () => {
    const lumen = await lumenProjectId();
    const change = await raiseChange({ title: "Lumen, membership edges", projectId: lumen, approval: true, review: false });
    expect((await requestApproval(change.id)).status).toBe(200);
    await staffDecides("alice", change.id);
    await staffDecides("alice", change.id);
    expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");
    const mira = customerApi("mira");
    const noel = customerApi("noel");
    const lumenBase = (await customerCounts(LOCAL_PERSONAS.noel.email, lumen))!;
    const noelSees = async (when: string, sees: boolean) => {
      expect((await noel.get(change.id)).status, `noel, ${when}: detail`).toBe(sees ? 200 : 404);
      expect((await noel.approvals(change.id)).status, `noel, ${when}: approvals`).toBe(sees ? 200 : 404);
      expect((await noel.listedNumbers(lumen)).includes(change.number), `noel, ${when}: listed`).toBe(sees);
    };
    await noelSees("registered and asked", true);

    // noel's registration on the project is ended (the contact is deactivated): the request made of him is still on record, and still shows nothing.
    await psql(`update project_contact set state = 'DEACTIVATED' where lower(email) = '${LOCAL_PERSONAS.noel.email}' and project_id = '${lumen}'`);
    try {
      await noelSees("deactivated on the project", false);
      expect((await noel.patch(change.id, { isCustomerApproved: true })).status, "a deactivated contact's answer").toBe(404);
      expect((await noel.stats(lumen)) === undefined || (await customerCounts(LOCAL_PERSONAS.noel.email, lumen))!.total === 0, "a deactivated contact's cards count nothing of it").toBe(true);
      expect((await mira.get(change.id)).status, "mira, still registered").toBe(200);
    } finally {
      await psql(`update project_contact set state = 'REGISTERED' where lower(email) = '${LOCAL_PERSONAS.noel.email}' and project_id = '${lumen}'`);
    }
    await noelSees("registered again (the designation was never taken away)", true);
    expect(await customerCounts(LOCAL_PERSONAS.noel.email, lumen)).toEqual(countsWith(lumenBase, null));

    // The sync moves the change request to Example Corp's project (the API cannot: the Customer Project is frozen once approval
    // was requested). dave and erin are registered contacts of THAT project, but nobody there was ever asked.
    const dave = customerApi("dave");
    const erin = customerApi("erin");
    await psql(`update work_item set project_id = '${FIXTURES.projectId}' where id = '${change.id}'`);
    try {
      expect((await mira.get(change.id)).status, "mira, after the move").toBe(404);
      expect((await mira.approvals(change.id)).status).toBe(404);
      expect(await mira.listedNumbers(lumen)).not.toContain(change.number);
      for (const [name, who] of [["dave", dave], ["erin", erin]] as const) {
        expect((await who.get(change.id)).status, `${name}, a registered contact of the new project nobody asked`).toBe(404);
        expect((await who.approvals(change.id)).status, `${name}: approvals`).toBe(404);
        expect(await who.listedNumbers(FIXTURES.projectId), `${name}: the new project's list`).not.toContain(change.number);
        expect((await who.patch(change.id, { isCustomerApproved: true })).status, `${name}: answer`).toBe(404);
      }
      if (entityServiceUrl()) {
        // ...and the service itself, which is where the rule lives, says the same
        const direct = await entityAsCustomer(LOCAL_PERSONAS.dave.email, "GET", `/change-requests/${change.id}`);
        expect(direct.status, `entity-service, dave: ${JSON.stringify(direct.body)}`).toBe(404);
      }
    } finally {
      await psql(`update work_item set project_id = '${lumen}' where id = '${change.id}'`);
    }
    expect((await mira.get(change.id)).status, "mira, moved back").toBe(200);
    expect((await dave.get(change.id)).status, "dave, with it back on Lumen").toBe(404);
  });

  test("the approvals a customer reads hold the customer's own rows by name, and every internal stage as a label and a status only", async () => {
    const lumen = await lumenProjectId();
    const change = await raiseChange({ title: "Lumen, approvals privacy", projectId: lumen, approval: true, review: false });
    expect((await requestApproval(change.id)).status).toBe(200);
    await staffDecides("alice", change.id);
    await staffDecides("bob", change.id);
    expect(await storedState(change.id)).toBe("CUSTOMER_APPROVAL");

    // What staff see, for contrast: the Peer and CAB stages with their people and group.
    const staffView = JSON.stringify((await staffApi("alice").approvals(change.id)).body);
    for (const name of ["Alice Perera", "Bob Fernando", "Carol Silva", "Example Corp ABT"]) {
      expect(staffView, `staff see ${name}`).toContain(name);
    }

    for (const who of ["mira", "noel"] as const) {
      const read = await customerApi(who).approvals(change.id);
      expect(read.status).toBe(200);
      const raw = JSON.stringify(read.body);
      for (const secret of ["Alice", "Bob", "Carol", "Perera", "Fernando", "Silva", "alice.", "bob.", "carol.", "Example Corp ABT", "example.com", "assignmentGroup"]) {
        expect(raw, `${who} is shown "${secret}" of an internal stage`).not.toContain(secret);
      }
      const stages = read.body.approvals ?? [];
      expect(stages.map((s) => s.stage)).toEqual(["Peer Approval", "CAB Approval", "Customer Approval"]);
      const [peer, cab, customers] = stages as Array<{ stage: string; status?: string; approvers?: { name?: string; status?: string }[]; approverName?: string }>;
      for (const internal of [peer, cab]) {
        expect(internal.status, `${internal.stage} status`).toBeTruthy();
        expect(internal.approvers ?? [], `${internal.stage} approvers`).toEqual([]);
        expect(internal.approverName ?? "", `${internal.stage} approver name`).toBe("");
      }
      expect((customers.approvers ?? []).map((a) => `${a.name}|${a.status}`).sort()).toEqual(["Mira Santos|REQUESTED", "Noel Prasad|REQUESTED"]);
    }
  });
});
