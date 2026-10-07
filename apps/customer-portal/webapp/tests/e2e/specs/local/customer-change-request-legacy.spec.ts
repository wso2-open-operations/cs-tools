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
// MIGRATED (LEGACY) CHANGE REQUESTS, on the real local stack.
//
// The dev, staging and production databases hold change requests migrated or synced from
// ServiceNow next to the ones raised through this portal. A migrated one was never put to the
// customer through our approval flow: it has no customer-stage rows, its customer flags are
// false, its approval stages (when it has any) carry no label. The product's rule for them
// (entity-service/CLAUDE.md, "Customer visibility and the cutover"):
//
//   * a change request created BEFORE the cutover instant (CR_STRICT_VISIBILITY_FROM; the
//     local stack sets 2000-01-01T00:00:00Z) is LEGACY: its project's registered contacts see
//     it exactly as customers always have, in every state but New / Assess / Authorize;
//   * one created at or after it follows the strict, designated-only rule;
//   * a legacy change request a customer ACTS on (answers, or proposes a new time) records
//     the designation in the same transaction, so it stays visible afterwards, Authorize
//     included;
//   * a legacy one in Customer Approval / Customer Review that has no live stage ("Demo Test 1":
//     asked under an older build) used to answer 409 "nobody asked": the first customer act
//     provisions the stage, once, and a plain read never writes;
//   * a stage the sync mirrored has no label: its pending approver stays pending across state
//     changes, is never shown to a customer by name, and a decision on it still works.
//
// The rows (fixtures/legacy-change-requests.sql) are written the way the sync writes them, with
// ServiceNow-style ids and numbers, created in 1999, i.e. before the local cutover. They are put
// back before every test and removed afterwards (a project's list is counted in other specs).
// Pictures of the key screens go to E2E_SHOT_DIR when it is set.
//
// ⚠️ STATE-CHANGING: needs E2E_POSTGRES_CONTAINER and E2E_CSM_BFF_URL and SKIPS without them.
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, openLocalContext, withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import {
  FIXTURES,
  LEGACY,
  LEGACY_VISIBLE,
  countsWith,
  customerApi,
  customerCounts,
  deleteLegacyChangeRequests,
  deleteRaisedChanges,
  futureWindow,
  legacyId,
  lumenProjectId,
  psql,
  raiseChange,
  requestApproval,
  resetFixtures,
  seedLegacyChangeRequests,
  shot,
  staffApi,
  staffDecides,
  staffMoves,
  stageRows,
  storedState,
  withFixtureStack,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test, true);

const BROWSER_ZONE = "America/New_York";
test.use({ timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 } });

const { projectId } = FIXTURES;
const id = legacyId;

/** The state label each legacy row on project 401 is in, as the customer API words it. */
const LABEL_OF: Record<string, string> = {
  [LEGACY.new]: "New",
  [LEGACY.assess]: "Assess",
  [LEGACY.authorize]: "Authorize",
  [LEGACY.customerApproval]: "Customer Approval",
  [LEGACY.scheduled]: "Scheduled",
  [LEGACY.implement]: "Implement",
  [LEGACY.review]: "Review",
  [LEGACY.customerReview]: "Customer Review",
  [LEGACY.rollback]: "Rollback",
  [LEGACY.closed]: "Closed",
  [LEGACY.canceled]: "Canceled",
  [LEGACY.customerApprovalToPropose]: "Customer Approval",
  [LEGACY.customerApprovalForErin]: "Customer Approval",
  [LEGACY.emergencyInAuthorize]: "Authorize",
  [LEGACY.staleStageScheduled]: "Scheduled",
  [LEGACY.oneSecondBefore]: "Scheduled",
  [LEGACY.atTheInstant]: "Scheduled",
};

/** The legacy numbers on project 401 that a customer was NEVER shown: New / Assess / Authorize, and the row created at the cutover instant itself. */
const LEGACY_HIDDEN = [LEGACY.new, LEGACY.assess, LEGACY.authorize, LEGACY.emergencyInAuthorize, LEGACY.atTheInstant] as const;

const onlyLegacy = (numbers: string[]) => numbers.filter((n) => n.startsWith("CHG0039")).sort();

test.describe("Local stack — legacy (migrated) change requests", () => {
  test.describe.configure({ timeout: 300_000 });

  test.beforeEach(async () => {
    await deleteRaisedChanges();
    await resetFixtures();
    await seedLegacyChangeRequests();
  });
  test.afterAll(async () => {
    await deleteLegacyChangeRequests();
    await deleteRaisedChanges();
  });

  test(`dave and erin see the legacy change requests exactly as customers see them today: every state but New, Assess and Authorize, the row at the cutover instant not, and the stat cards count what the list shows`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL, timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 } });
    try {
      for (const who of ["dave", "erin"] as const) {
        const api = customerApi(who);
        const listed = await api.listed(projectId);
        const legacyListed = onlyLegacy(listed.map((c) => c.number));
        expect(legacyListed, `${who}: the legacy rows listed`).toEqual([...LEGACY_VISIBLE].sort());
        for (const c of listed.filter((x) => x.number.startsWith("CHG0039"))) {
          expect(c.state?.label, `${who}: state of ${c.number}`).toBe(LABEL_OF[c.number]);
        }
        // The two fixtures (asked of dave and erin) are there beside them.
        expect(listed.map((c) => c.number)).toEqual(expect.arrayContaining(["CHG-FIXED-007", "CHG-FIXED-008"]));

        // The stat cards: the list, state by state, and nothing else (New / Assess / Authorize rows add nothing).
        const visibleLabels = [...LEGACY_VISIBLE].map((n) => LABEL_OF[n]);
        const stats = (await customerCounts(LOCAL_PERSONAS[who].email, projectId))!;
        const expected = countsWith({ total: 0, active: 0, outstanding: 0, actionRequired: 0, resolved: 0, dashboard: 0, byState: {} }, ["Customer Approval", "Customer Review", ...visibleLabels]);
        expect({ ...stats, byState: undefined }, `${who}: stat cards and the dashboard`).toEqual({ ...expected, byState: undefined });
        for (const label of Object.keys(expected.byState)) {
          expect(stats.byState[label] ?? 0, `${who}: ${label} card`).toBe(expected.byState[label]);
        }
        expect(stats.total, "the cards count the list").toBe(listed.length);

        // By id: 200 for the visible ones (nothing to answer unless asked), 404 for the hidden, approvals alike.
        for (const number of [...LEGACY_VISIBLE]) {
          const read = await api.get(id(number));
          expect(read.status, `${who}: ${number}`).toBe(200);
          expect((await api.approvals(id(number))).status, `${who}: ${number} approvals`).toBe(200);
        }
        for (const number of LEGACY_HIDDEN) {
          expect((await api.get(id(number))).status, `${who}: ${number} (${LABEL_OF[number]}) read`).toBe(404);
          expect((await api.approvals(id(number))).status, `${who}: ${number} approvals`).toBe(404);
          expect((await api.patch(id(number), { isCustomerApproved: true })).status, `${who}: ${number} answer`).toBe(404);
        }
      }

      // The state filters: Authorize lists none of the hidden ones; the New / Assess ids list nothing.
      const dave = customerApi("dave");
      expect(onlyLegacy(await dave.listedNumbers(projectId, { stateKeys: [-3] }))).toEqual([]);
      expect(onlyLegacy(await dave.listedNumbers(projectId, { stateKeys: [-5, -4] }))).toEqual([]);

      // The pages: the list shows the legacy rows by state, the hidden ones are not found by address.
      const list = new ChangeRequestsPage(page);
      await list.open(projectId);
      await list.waitForList();
      // (The list pages its rows, so each number is looked up with the list's own search.)
      for (const number of [LEGACY.scheduled, LEGACY.closed, LEGACY.customerReview, LEGACY.oneSecondBefore]) {
        await list.searchFor(number);
        await expect(list.rowByNumber(number), `${number} in dave's list`).toHaveCount(1);
        await expect(list.rowByNumber(number)).toContainText(LABEL_OF[number]);
      }
      for (const number of LEGACY_HIDDEN) {
        await list.searchFor(number);
        await expect(list.rowByNumber(number), `${number} (${LABEL_OF[number]}) in dave's list`).toHaveCount(0);
      }
      await list.searchFor("CHG0039");
      await expect(page.getByText(/Showing \d+ of 12 change requests/), "the list says how many legacy rows it holds").toBeVisible();
      await shot(page, "10-legacy-dave-list");
      const details = new ChangeRequestDetailsPage(page);
      await details.open(projectId, id(LEGACY.closed), LEGACY.closed);
      await expect(details.currentStage()).toHaveText(UI.stages.closed);
      await expect(details.answerButtons()).toHaveCount(0);
      expect((await details.openAndSettle(projectId, id(LEGACY.authorize), LEGACY.authorize)).loaded, "legacy Authorize rendered").toBe(false);
      await shot(page, "11-legacy-authorize-by-address-not-found");

      const erinDetails = new ChangeRequestDetailsPage(await erinContext.newPage());
      await erinDetails.open(projectId, id(LEGACY.scheduled), LEGACY.scheduled);
      await expect(erinDetails.currentStage()).toHaveText(UI.stages.scheduled);
    } finally {
      await erinContext.close();
    }
  });

  test("a change request raised after the cutover that nobody was asked about is NOT shown in a state a legacy one is: the same Scheduled, a different row", async () => {
    // Identical in state, project and flags to CHG0039105 (Scheduled, flags false, nothing asked); only its age differs.
    const strict = await raiseChange({ title: "strict, Scheduled, nobody asked", projectId, approval: false, review: false });
    expect((await requestApproval(strict.id)).status).toBe(200);
    await staffDecides("alice", strict.id);
    await staffDecides("alice", strict.id);
    expect(await storedState(strict.id)).toBe("SCHEDULED");

    for (const who of ["dave", "erin"] as const) {
      const api = customerApi(who);
      const listed = (await api.listed(projectId)).map((c) => c.number);
      expect(listed, `${who}: the legacy Scheduled`).toContain(LEGACY.scheduled);
      expect(listed, `${who}: the strict Scheduled`).not.toContain(strict.number);
      expect((await api.get(id(LEGACY.scheduled))).status).toBe(200);
      expect((await api.get(strict.id)).status, `${who}: the strict one by id`).toBe(404);
      expect((await api.approvals(strict.id)).status).toBe(404);
    }
    // The one at the instant itself is strict; one second earlier is legacy.
    expect((await customerApi("dave").get(id(LEGACY.oneSecondBefore))).status).toBe(200);
    expect((await customerApi("dave").get(id(LEGACY.atTheInstant))).status).toBe(404);
  });

  test(`a legacy change request in Customer Approval with NO live stage (the "Demo Test 1" shape, on Lumen Works Platform) can be answered by a contact: reading changes nothing, the answer provisions the stage once and it stays visible to both contacts`, async ({
    browser,
    baseURL,
  }) => {
    const lumen = await lumenProjectId();
    const number = LEGACY.lumenDemoTest;
    const crId = id(number);
    const miraContext = await openLocalContext(test, browser, "mira", { baseURL, timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 } });
    try {
      const mira = customerApi("mira");
      const noel = customerApi("noel");

      // Reading it, in the API and in the page, offers the answer and writes nothing.
      for (let i = 0; i < 3; i++) {
        const read = await mira.get(crId);
        expect(read.status).toBe(200);
        expect(read.body.state?.label).toBe("Customer Approval");
        expect(read.body.customerCanAnswer, "the contact is offered the answer").toBe(true);
      }
      const page = await miraContext.newPage();
      const details = new ChangeRequestDetailsPage(page);
      await details.open(lumen, crId, number);
      for (const name of [UI.buttons.approve, UI.buttons.reject, UI.buttons.proposeNewTime]) {
        await expect(details.button(name), `${name} for mira`).toBeVisible();
      }
      await shot(page, "12-legacy-demo-test-1-answer-offered");
      expect(await stageRows(crId), "reading provisioned nothing").toEqual([]);

      // The answer: Approve in the page.
      await details.button(UI.buttons.approve).click();
      await expect(details.banner(UI.banners.approved)).toBeVisible();
      await expect(details.currentStage()).toHaveText(UI.stages.scheduled);
      expect(await storedState(crId)).toBe("SCHEDULED");
      expect(await stageRows(crId), "the stage the answer needed, written once, every contact asked").toEqual([
        `Customer Approval|${LOCAL_PERSONAS.mira.email}|APPROVED`,
        `Customer Approval|${LOCAL_PERSONAS.noel.email}|CANCELLED`,
      ]);
      await shot(page, "13-legacy-demo-test-1-scheduled");

      // Both still see it, and a second answer is refused without provisioning anything again.
      for (const who of [mira, noel]) {
        expect((await who.get(crId)).status).toBe(200);
        expect(await who.listedNumbers(lumen)).toContain(number);
      }
      const late = await noel.patch(crId, { isCustomerApproved: true });
      expect(late.status, JSON.stringify(late.body)).toBe(409);
      expect(await stageRows(crId)).toHaveLength(2);
    } finally {
      await miraContext.close();
    }
  });

  test("a legacy Customer Review with no live stage: dave's Unsuccessful provisions the stage, sends it to Rollback, and erin's request is cancelled, never left asking", async () => {
    const crId = id(LEGACY.customerReview);
    expect(await stageRows(crId)).toEqual([]);
    expect((await customerApi("dave").get(crId)).body.customerCanAnswer).toBe(true);
    expect((await customerApi("erin").get(crId)).body.customerCanAnswer).toBe(true);
    const answered = await customerApi("dave").patch(crId, { isCustomerReviewed: false });
    expect(answered.status, JSON.stringify(answered.body)).toBe(200);
    expect(await storedState(crId)).toBe("ROLLBACK");
    expect(await stageRows(crId)).toEqual([
      "Customer Review|dave.mendis@example.com|REJECTED",
      "Customer Review|erin.jayawardena@example.com|CANCELLED",
    ]);
    expect((await customerApi("erin").get(crId)).status).toBe(200);
    expect((await customerApi("erin").get(crId)).body.customerCanAnswer).toBe(false);
  });

  test(`a legacy change request dave PROPOSES a new time on goes to Authorize, a state a legacy one is hidden in, and STAYS visible to dave and erin: the loop through CAB and erin's approval keeps it theirs, while an untouched legacy one in Authorize is never shown`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const number = LEGACY.customerApprovalToPropose;
    const crId = id(number);
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL, timezoneId: BROWSER_ZONE, colorScheme: "dark", viewport: { width: 1280, height: 900 } });
    try {
      const dave = customerApi("dave");
      const erin = customerApi("erin");
      const list = new ChangeRequestsPage(page);
      const details = new ChangeRequestDetailsPage(page);
      const erinPage = await erinContext.newPage();
      const erinDetails = new ChangeRequestDetailsPage(erinPage);

      expect(await stageRows(crId)).toEqual([]);
      await details.open(projectId, crId, number);
      await expect(details.button(UI.buttons.proposeNewTime)).toBeVisible();
      await details.button(UI.buttons.proposeNewTime).click();
      await expect(details.proposeDialog()).toBeVisible();
      const zone = await details.proposeTimeZone();
      const window = futureWindow(zone, { daysAhead: 6, startHour: 10, hours: 3 });
      await details.fillProposedWindow(window.start, window.end);
      await details.submitProposalButton().click();
      await expect(details.banner(UI.banners.proposedNormal)).toBeVisible();
      await expect(details.currentStage()).toHaveText(UI.stages.authorize);
      expect(await storedState(crId)).toBe("AUTHORIZE");

      // The proposal needed a stage, so the contacts were asked and then cancelled: that is the designation.
      const afterProposal = await stageRows(crId);
      expect(
        afterProposal.filter((r) => r.startsWith("Customer Approval")),
        "the stage the proposal provisioned, superseded by the proposal itself",
      ).toEqual(["Customer Approval|dave.mendis@example.com|CANCELLED", "Customer Approval|erin.jayawardena@example.com|CANCELLED"]);
      expect(afterProposal.filter((r) => r.startsWith("CAB Approval")).map((r) => r.split("|")[2]), "the CAB is asked about the new time").toEqual(["REQUESTED", "REQUESTED", "REQUESTED"]);
      for (const [who, label] of [[dave, "dave"], [erin, "erin"]] as const) {
        const listed = await who.listed(projectId);
        expect(listed.find((c) => c.number === number)?.state?.label, `${label}: legacy row in Authorize after the proposal`).toBe("Authorize");
        expect((await who.get(crId)).status).toBe(200);
        // ...whereas the legacy row that sits in Authorize and was never acted on is not theirs
        expect(listed.map((c) => c.number)).not.toContain(LEGACY.authorize);
        expect((await who.get(id(LEGACY.authorize))).status).toBe(404);
      }
      await list.open(projectId);
      await list.waitForList();
      await expect(list.rowByNumber(number)).toHaveCount(1);
      await expect(list.rowByNumber(number)).toContainText("Authorize");
      await expect(list.rowByNumber(LEGACY.authorize)).toHaveCount(0);
      await shot(page, "14-legacy-list-authorize-after-proposal");
      await erinDetails.open(projectId, crId, number);
      await expect(erinDetails.currentStage()).toHaveText(UI.stages.authorize);
      await expect(erinDetails.answerButtons()).toHaveCount(0);
      const stats = (await customerCounts(LOCAL_PERSONAS.dave.email, projectId))!;
      expect(stats.byState.Authorize, "the Authorize card counts the one they proposed on").toBe(1);

      // WSO2's CAB approves the new time. A migrated change request has customer_approval_required = false (that column
      // is ours, and the sync never set it), but a proposal IS a Re-schedule and a Re-schedule writes the requirement
      // (true) with the new window: the new plan goes back to the customers, as it does for a native change request.
      // It used to end in Scheduled with nobody asked about the window, a gap of migration 0189's default, not a rule.
      await staffDecides("alice", crId);
      expect(await storedState(crId)).toBe("CUSTOMER_APPROVAL");
      expect((await stageRows(crId)).filter((r) => r.startsWith("Customer Approval")), "both contacts are asked a second time").toEqual([
        "Customer Approval|dave.mendis@example.com|CANCELLED",
        "Customer Approval|erin.jayawardena@example.com|CANCELLED",
        "Customer Approval|dave.mendis@example.com|REQUESTED",
        "Customer Approval|erin.jayawardena@example.com|REQUESTED",
      ]);
      for (const who of [dave, erin]) {
        expect((await who.listed(projectId)).find((c) => c.number === number)?.state?.label).toBe("Customer Approval");
        const seen = await who.get(crId);
        expect(seen.status).toBe(200);
        expect(seen.body.customerCanAnswer, "asked afresh").toBe(true);
      }
      await erinDetails.open(projectId, crId, number);
      await expect(erinDetails.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(erinDetails.approvalButtons().first()).toBeVisible();
      await shot(erinPage, "15-legacy-erin-detail-customer-approval-after-proposal");
      const done = await psql(`select to_char(start_on at time zone 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"') from change_request where id = '${crId}'`);
      expect(done.trim(), "the window dave proposed is the one put to them").toBe(window.startUtc);
      // erin approves the new window: Scheduled for it, and a legacy Scheduled is a state customers see.
      const approved = await erin.patch(crId, { isCustomerApproved: true });
      expect(approved.status, JSON.stringify(approved.body)).toBe(200);
      expect(await storedState(crId)).toBe("SCHEDULED");
      for (const who of [dave, erin]) {
        expect((await who.listed(projectId)).find((c) => c.number === number)?.state?.label).toBe("Scheduled");
        expect((await who.get(crId)).status).toBe(200);
      }
    } finally {
      await erinContext.close();
    }
  });

  test("WSO2 can ask a legacy change request's customers again after a proposal: the box is ticked in Authorize (add-only is allowed there, the project is stored), the CAB approves, and both contacts are asked afresh", async () => {
    const number = LEGACY.customerApprovalForErin;
    const crId = id(number);
    const dave = customerApi("dave");
    const erin = customerApi("erin");
    const window = futureWindow("America/New_York", { daysAhead: 8, startHour: 9, hours: 2 });
    const proposed = await erin.patch(crId, {
      plannedStartOn: window.startUtc.replace("T", " ").replace("Z", ""),
      plannedEndOn: window.endUtc.replace("T", " ").replace("Z", ""),
    });
    expect(proposed.status, JSON.stringify(proposed.body)).toBe(200);
    expect(await storedState(crId)).toBe("AUTHORIZE");
    for (const who of [dave, erin]) expect((await who.get(crId)).status, "visible in Authorize").toBe(200);

    // Authorize: unticked -> ticked is allowed (the Customer Project is stored); the lock only ever forbids taking it away.
    const ticked = await staffApi("alice").patch(crId, { customerApprovalRequired: true });
    expect(ticked.status, JSON.stringify(ticked.body)).toBe(200);
    const unticked = await staffApi("alice").patch(crId, { customerApprovalRequired: false });
    expect(unticked.status, "once ticked it cannot be removed").toBe(400);
    expect(JSON.stringify(unticked.body)).toContain("can no longer be turned off");

    await staffDecides("bob", crId);
    expect(await storedState(crId)).toBe("CUSTOMER_APPROVAL");
    expect((await stageRows(crId)).filter((r) => r.startsWith("Customer Approval"))).toEqual([
      "Customer Approval|dave.mendis@example.com|CANCELLED",
      "Customer Approval|erin.jayawardena@example.com|CANCELLED",
      "Customer Approval|dave.mendis@example.com|REQUESTED",
      "Customer Approval|erin.jayawardena@example.com|REQUESTED",
    ]);
    expect((await dave.get(crId)).body.customerCanAnswer).toBe(true);
    expect((await dave.patch(crId, { isCustomerApproved: true })).status).toBe(200);
    expect(await storedState(crId)).toBe("SCHEDULED");
    for (const who of [dave, erin]) expect((await who.get(crId)).status).toBe(200);
  });

  test("a stage the sync mirrored with no label: an Emergency change in Authorize is decided by its pending approver (it used to be refused as stale); a stale position-0 stage on a Scheduled one keeps its pending approver through every state move, and customers are shown its label and status only", async () => {
    // --- CHG0039301: Emergency in Authorize, ONE synced stage with no label (alice and bob REQUESTED).
    const emergency = id(LEGACY.emergencyInAuthorize);
    const alice = staffApi("alice");
    const view = await alice.approvals(emergency);
    expect(view.status, JSON.stringify(view.body)).toBe(200);
    const stage = view.body.approvals?.[0];
    expect(stage, "the synced stage is read").toBeTruthy();
    const aliceRow = stage!.approvers?.find((a) => a.name === "Alice Perera");
    expect(aliceRow?.status).toBe("REQUESTED");
    expect(aliceRow?.canDecide, "the pending approver of the synced ECAB stage can decide").toBe(true);
    // A customer is shown none of this while it is in Authorize: legacy Authorize is hidden.
    expect((await customerApi("dave").get(emergency)).status).toBe(404);
    const decided = await alice.decide(emergency, "approved");
    expect(decided.status, JSON.stringify(decided.body)).toBe(200);
    expect(await storedState(emergency), "the decision on the synced stage moved the Emergency change on").toBe("SCHEDULED");
    expect((await stageRows(emergency)).filter((r) => r.includes("alice.perera"))).toEqual(["(no label)|alice.perera@example.com|APPROVED"]);
    // Scheduled is a state legacy customers see: the change request now is theirs to read.
    expect((await customerApi("dave").get(emergency)).status).toBe(200);

    // --- CHG0039302: Normal, Scheduled, a stale position-0 stage (bob REQUESTED, plus a row whose user is missing).
    const stale = id(LEGACY.staleStageScheduled);
    expect(await stageRows(stale)).toEqual(["(no label)|bob.fernando@example.com|REQUESTED", "(no label)|(no user)|REQUESTED"]);
    const bob = staffApi("bob");
    for (const [moveTo, stored] of [["implement", "IMPLEMENT"], ["review", "REVIEW"]] as const) {
      await staffMoves("alice", stale, moveTo);
      expect(await storedState(stale)).toBe(stored);
      expect(await stageRows(stale), `a move to ${stored} left the synced rows as they were`).toEqual([
        "(no label)|bob.fernando@example.com|REQUESTED",
        "(no label)|(no user)|REQUESTED",
      ]);
      const read = await bob.approvals(stale);
      expect(read.status, `the approvals of a change with an approver whose user is missing (${stored})`).toBe(200);
      const bobRow = read.body.approvals?.flatMap((s) => s.approvers ?? []).find((a) => a.name === "Bob Fernando");
      expect(bobRow?.status).toBe("REQUESTED");
      expect(bobRow?.canDecide, `bob can still decide in ${stored}`).toBe(true);
    }
    // The customer's view of the synced stage: label and status, no name.
    const customerView = JSON.stringify((await customerApi("dave").approvals(stale)).body);
    expect(customerView).not.toContain("Bob");
    expect(customerView).not.toContain("Fernando");
    expect(customerView).not.toContain("bob.");
    // A finished change takes every row, as it always did.
    await staffMoves("alice", stale, "closed");
    expect(await storedState(stale)).toBe("CLOSED");
    expect(await stageRows(stale)).toEqual(["(no label)|bob.fernando@example.com|CANCELLED", "(no label)|(no user)|CANCELLED"]);
  });
});
