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
// A customer PROPOSES A NEW IMPLEMENTATION TIME in the customer portal, on the real
// local stack, and WSO2 answers it.
//
// The process (entity-service/CLAUDE.md, "A customer's proposed time"): the previous system's
// own mechanism. A contact who is asked at Customer Approval proposes a new START (the planned
// length stays, so the dialog shows the end and never asks for it). The proposal is written
// to the change request's customer_updated_on and NOTHING else moves: the change STAYS in
// Customer Approval, its planned window is what WSO2 planned, and every customer request
// stays live. WSO2 then answers it, from the CSM portal (here: through the CSM portal's
// backend, as its buttons do):
//   * Accept proposed time  -> the proposed start is applied to the planned length and the change
//                              goes straight to Scheduled: no CAB, no second ask of the customer;
//   * Propose a different time -> WSO2's own window, and the customers are asked again (no CAB);
//   * the same with the window kept = a decline (the customers' live requests are untouched).
//
// What the tests prove, with who acts as who:
//   1. The dialog asks for a START; the end is shown (read-only) and follows it, and an empty,
//      a past or an unchanged start shows an inline error and sends NOTHING.
//   2. ACCEPT: dave proposes -> the change stays in Customer Approval with every answer on offer,
//      the page says WSO2 has not answered (and that Approve approves the CURRENT schedule),
//      the stack holds only the proposed instant -> alice accepts through the CSM portal's
//      backend -> Scheduled with the proposed start and the planned length, no stage and no
//      approver row added, the customers' requests closed, the flag of the customer's approval
//      untouched -> both customers see Scheduled, the accepted note and the step done.
//   3. COUNTER: WSO2 proposes its own window -> the customers are asked again with fresh
//      requests, the page says the proposal was not accepted and shows the current window, and
//      erin approves it.
//   4. DECLINE: the window kept -> the live requests untouched, "not accepted" on the page, and
//      the customer can propose again (the answer is cleared) and be accepted.
//   5. A Standard change behaves the same (no copy or flow depends on the type), a plain Re-schedule
//      asks the customers again with no CAB, and Approve while a proposal waits approves the CURRENT window.
//   6. Holds, no window, stale pages and the refusals, in the service's words; the loop repeats.
//
// ⚠️ STATE-CHANGING (re-seeds the stack's Postgres first, so it needs
// E2E_POSTGRES_CONTAINER, and for WSO2's side E2E_CSM_BFF_URL; it SKIPS without them).
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, openLocalContext, withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import {
  FIXTURES,
  STAFF_APPROVERS,
  addHours,
  approverRows,
  changeRequestRow,
  customerApi,
  futureWindow,
  patchAsStaff,
  planWindow,
  proposalRow,
  psql,
  resetFixtures,
  shot,
  staffAcceptsProposal,
  staffApi,
  staffCountersProposal,
  staffDeclinesProposal,
  staffReschedules,
  stackEndpoints,
  wallTime,
  withFixtureStack,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test, true);

const { approval, standardNew, projectId } = FIXTURES;

/**
 * The browser's time zone, pinned to one that is neither UTC nor the machine's, so
 * the conversion of a proposed wall time to the UTC the backend stores is really
 * exercised. (The dialog reads the signed-in user's zone when the profile has one and
 * the browser's otherwise: the tests read the zone the dialog says and compute from it.)
 */
const BROWSER_ZONE = "America/New_York";
test.use({ timezoneId: BROWSER_ZONE });

/** The planned length of the fixture's window (see `plannedFixtureWindow`): a proposal keeps it. */
const LENGTH_HOURS = 2;

/** The customers' rows of a change, as `name|state`, oldest stage first. */
async function customerRows(id: string): Promise<string[]> {
  return (await approverRows(id))
    .filter((r) => r.stage === "Customer Approval")
    .map((r) => `${r.email.split("@")[0]}|${r.state}`);
}

/** Dave proposes `startUtc` through the customer backend, as the dialog would: the start and the end that keeps the length. */
async function davePropose(id: string, startUtc: string, who: "dave" | "erin" = "dave"): Promise<void> {
  const sent = await customerApi(who).patch(id, {
    plannedStartOn: startUtc,
    plannedEndOn: addHours(startUtc, LENGTH_HOURS),
  });
  expect(sent.status, JSON.stringify(sent.body)).toBe(200);
}

test.describe("Local stack — a customer proposes a new implementation time and WSO2 answers it", () => {
  test.describe.configure({ timeout: 240_000 });

  test.beforeEach(async () => {
    await resetFixtures();
  });

  test(`Propose New Time asks for a START: the end is shown, read-only, and follows it; an empty, a past and an unchanged start show inline errors and send nothing`, async ({
    page,
  }) => {
    const patches: string[] = [];
    page.on("request", (request) => {
      if (request.method() === "PATCH") patches.push(request.url());
    });

    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await dave.button(UI.buttons.proposeNewTime).click();

    const dialog = dave.proposeDialog();
    await expect(dialog).toBeVisible();
    // The start is asked for; the end is shown, and cannot be typed.
    await expect(dave.proposedStart()).toBeEditable();
    await expect(dave.proposedEnd()).toBeVisible();
    await expect(dave.proposedEnd()).not.toBeEditable();
    await expect(dave.proposedEnd()).toHaveAttribute("readonly", "");
    // The same for every change type: no CAB round trip, no second approval is promised.
    await expect(dialog).toContainText(UI.propose.notice);
    await expect(dialog).toContainText(`planned length of ${LENGTH_HOURS} hours ${UI.propose.lengthStaysTheSame}`);
    await expect(dialog).not.toContainText(/internal|CAB|approval again/i);

    // The fields start from the planned window, in the zone the dialog says it reads.
    const zone = await dave.proposeTimeZone();
    test.info().annotations.push({ type: "time zone read by the dialog", description: zone });
    const planned = await changeRequestRow(approval.id);
    await expect(dave.proposedStart()).toHaveValue(wallTime(new Date(planned.startUtc), zone));
    await expect(dave.proposedEnd()).toHaveValue(wallTime(new Date(planned.endUtc), zone));

    // 1. Nothing typed: the start is required.
    await dave.proposedStart().fill("");
    await dave.submitProposalButton().click();
    await expect(dialog.getByText(UI.propose.errors.startRequired)).toBeVisible();
    await expect(dave.proposedEnd()).toHaveValue("");

    // 2. A start in the past.
    const past = futureWindow(zone, { daysAhead: -2, startHour: 10, hours: LENGTH_HOURS });
    await dave.fillProposedStart(past.start);
    await expect(dialog.getByText(UI.propose.errors.startPast)).toBeVisible();
    await expect(dialog.getByText(UI.propose.errors.startRequired)).toBeHidden();
    await dave.submitProposalButton().click();
    await expect(dialog.getByText(UI.propose.errors.startPast)).toBeVisible();

    // 3. The start that is already the planned one.
    await dave.fillProposedStart(wallTime(new Date(planned.startUtc), zone));
    await dave.submitProposalButton().click();
    await expect(dialog.getByRole("alert").filter({ hasText: UI.propose.errors.unchanged })).toBeVisible();

    // 4. A good start: the end follows it, keeping the planned length.
    const later = futureWindow(zone, { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
    await dave.fillProposedStart(later.start);
    await expect(dave.proposedEnd(), "the end follows the start, same length").toHaveValue(later.end);
    await expect(dialog.getByRole("alert")).toHaveCount(0);

    // The dialog stayed open and nothing was sent: the change still waits, unchanged.
    await expect(dialog).toBeVisible();
    expect(patches, "an invalid start must not leave the browser").toEqual([]);
    const api = await customerApi("dave").get(approval.id);
    expect(api.body.state?.label).toBe("Customer Approval");
    expect(api.body.customerCanAnswer).toBe(true);
    expect(api.body.customerProposal ?? null).toBeNull();
    expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: planned.startUtc, endUtc: planned.endUtc });
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: "", answer: "" });

    // Cancel closes it, still without a request.
    await dialog.getByRole("button", { name: UI.propose.cancel, exact: true }).click();
    await expect(dialog).toBeHidden();
    expect(patches).toEqual([]);
  });

  test(`ACCEPT: ${LOCAL_PERSONAS.dave.email} proposes a start for ${approval.number}, the change STAYS in Customer Approval with every answer on offer and nothing else moves; WSO2 accepts it and the change is Scheduled at that start`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL, timezoneId: BROWSER_ZONE });
    try {
      const dave = new ChangeRequestDetailsPage(page);
      const erinPage = await erinContext.newPage();
      const erin = new ChangeRequestDetailsPage(erinPage);
      const planned = await changeRequestRow(approval.id);
      const rowsBefore = await approverRows(approval.id);
      const flagBefore = await psql(`select is_customer_approval_required from change_request where id = '${approval.id}'`);
      await dave.open(projectId, approval.id, approval.number);
      await erin.open(projectId, approval.id, approval.number);

      // dave proposes a start, in the zone the dialog says it reads.
      await dave.button(UI.buttons.proposeNewTime).click();
      await expect(dave.proposeDialog()).toBeVisible();
      const zone = await dave.proposeTimeZone();
      const proposal = futureWindow(zone, { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
      await dave.fillProposedStart(proposal.start);
      await expect(dave.proposedEnd()).toHaveValue(proposal.end);
      await shot(page, "20-propose-dialog-start-and-derived-end");
      await dave.submitProposalButton().click();

      // The copy tells him what happens next: WSO2 accepts it or suggests another time.
      await expect(dave.banner(UI.banners.proposed)).toBeVisible({ timeout: 20_000 });
      await expect(dave.proposeDialog()).toBeHidden();

      // The change STAYED in Customer Approval, and every answer is still on offer.
      await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(dave.button(name), `${name} stays for dave`).toBeVisible();
      }
      // The page keeps saying WSO2 has not answered, after the toast has gone, and warns that Approve approves the
      // CURRENT schedule. It never says the change is reviewed internally: it is not in Authorize.
      await expect(dave.banner(UI.banners.proposed)).toBeHidden({ timeout: 20_000 });
      await expect(dave.proposalWaitingNote()).toContainText(UI.notes.waitingOwn);
      await expect(dave.proposalWaitingNote()).toContainText(UI.notes.approvingNow);
      await expect(dave.windowProposedStart()).toContainText("waiting for WSO2");
      await expect(dave.stageCaption(UI.stageCaptions.waitingOwn)).toBeVisible();
      await expect(dave.internalReviewNote()).toHaveCount(0);
      await expect(dave.windowCardTitle(UI.windowCard.planned)).toBeVisible();
      // Closing the dialog handed focus back to the button that opened it, which is still there.
      await expect(dave.button(UI.buttons.proposeNewTime)).toBeFocused();
      await shot(page, "21-dave-waiting-for-wso2");

      // What the stack holds: ONLY the proposed instant. The state, the window, every stage and every approver row are as they were.
      expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: planned.startUtc, endUtc: planned.endUtc });
      expect(await proposalRow(approval.id), "the proposal, as the UTC instant, with no answer").toEqual({ proposedUtc: proposal.startUtc, answer: "" });
      expect(await approverRows(approval.id), "no stage, no approver row was touched").toEqual(rowsBefore);

      // What each side is told. A customer: pending, and whether the proposer is the person reading. WSO2: who and when.
      const daveApi = (await customerApi("dave").get(approval.id)).body;
      expect(daveApi.customerProposal).toMatchObject({ answer: "pending", proposedByViewer: true });
      expect(daveApi.customerCanAnswer, "the proposer is still asked").toBe(true);
      const erinApi = (await customerApi("erin").get(approval.id)).body;
      expect(erinApi.customerProposal).toMatchObject({ answer: "pending", proposedByViewer: false });
      expect(erinApi.customerCanAnswer).toBe(true);
      const staffView = (await staffApi("alice").get(approval.id)).body;
      expect(staffView.state).toBe("customer_approval");
      expect(staffView.customerProposal).toMatchObject({
        answer: "pending",
        startOn: proposal.startUtc,
        proposerRecorded: true,
        proposedByEmail: LOCAL_PERSONAS.dave.email,
        canAccept: true,
      });

      // erin did not propose: she is told neutrally, and may still answer.
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.proposalWaitingNote()).toContainText(UI.notes.waitingOther);
      await expect(erin.proposalWaitingNote()).not.toContainText(UI.notes.waitingOwn);
      await expect(erin.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(erin.button(UI.buttons.approve)).toBeVisible();
      await expect(erin.stageCaption(UI.stageCaptions.waitingOther)).toBeVisible();
      await shot(erinPage, "22-erin-waiting-for-wso2-neutral");

      // WSO2 (alice) accepts it, as the CSM portal's "Accept proposed time" does.
      const accepted = await staffAcceptsProposal("alice", approval.id);
      expect(accepted.status, JSON.stringify(accepted.body)).toBe(200);

      // Scheduled at the proposed start with the planned length; AGREE; no CAB, no stage, no new request.
      const done = await changeRequestRow(approval.id);
      expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", proposal.startUtc, addHours(proposal.startUtc, LENGTH_HOURS)]);
      expect(await proposalRow(approval.id)).toEqual({ proposedUtc: proposal.startUtc, answer: "AGREE" });
      expect(
        (await approverRows(approval.id)).map((r) => `${r.stage}|${r.email}`),
        "no stage and no approver row was added",
      ).toEqual(rowsBefore.map((r) => `${r.stage}|${r.email}`));
      expect(await customerRows(approval.id), "the customers' requests are closed, not answered").toEqual(["dave.mendis|CANCELLED", "erin.jayawardena|CANCELLED"]);
      expect(
        await psql(`select is_customer_approval_required from change_request where id = '${approval.id}'`),
        "no staff action records a customer's approval: the flag is not stamped",
      ).toBe(flagBefore);

      // Both customers see Scheduled with the accepted window, the step done, nothing to answer.
      for (const [who, view] of [["dave", dave], ["erin", erin]] as const) {
        await view.open(projectId, approval.id, approval.number);
        await expect(view.currentStage(), who).toHaveText(UI.stages.scheduled);
        await expect(view.answerButtons(), who).toHaveCount(0);
        await expect(view.proposalWaitingNote(), who).toHaveCount(0);
        await expect(view.windowCardTitle(UI.windowCard.scheduled), who).toBeVisible();
        await expect(view.windowAcceptedNote(), who).toContainText(UI.notes.windowAccepted);
        await expect(view.stageCaption(UI.stageCaptions.accepted), who).toBeVisible();
        if (who === "dave") await shot(page, "23-dave-scheduled-proposal-accepted");
        const api = (await customerApi(who).get(approval.id)).body;
        expect(api.state?.label).toBe("Scheduled");
        expect(api.customerProposal).toMatchObject({ answer: "agreed" });
        expect(api.customerCanAnswer).toBe(false);
        expect(api.hasCustomerApproved, "the flag stays false: the display reads the proposal").toBe(false);
      }
    } finally {
      await erinContext.close();
    }
  });

  test(`COUNTER: WSO2 proposes its own window -> the customers are asked again with fresh requests, no CAB, the page says the proposal was not accepted, and erin approves WSO2's window`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL, timezoneId: BROWSER_ZONE });
    try {
      const dave = new ChangeRequestDetailsPage(page);
      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      const rowsBefore = await approverRows(approval.id);
      const proposal = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
      await davePropose(approval.id, proposal.startUtc);

      // WSO2's own window: a different start AND a different length (it is WSO2's to plan).
      const wso2 = futureWindow("UTC", { daysAhead: 45, startHour: 9, hours: 3 });
      const countered = await staffCountersProposal("alice", approval.id, wso2);
      expect(countered.status, JSON.stringify(countered.body)).toBe(200);

      // Still Customer Approval, WSO2's window, the answer DISAGREE; the customers asked again, fresh requests, no CAB.
      expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: wso2.startUtc, endUtc: wso2.endUtc });
      expect(await proposalRow(approval.id)).toEqual({ proposedUtc: proposal.startUtc, answer: "DISAGREE" });
      expect(await customerRows(approval.id), "the old requests are cancelled, a fresh one is asked of each").toEqual([
        "dave.mendis|CANCELLED",
        "erin.jayawardena|CANCELLED",
        "dave.mendis|REQUESTED",
        "erin.jayawardena|REQUESTED",
      ]);
      expect((await approverRows(approval.id)).filter((r) => r.stage !== "Customer Approval"), "no CAB stage appeared").toEqual(
        rowsBefore.filter((r) => r.stage !== "Customer Approval"),
      );
      const api = (await customerApi("dave").get(approval.id)).body;
      expect(api.state?.label).toBe("Customer Approval");
      expect(api.customerProposal).toMatchObject({ answer: "disagreed" });
      expect(api.customerCanAnswer).toBe(true);

      // dave's page: the proposal was not accepted, the CURRENT window is below, and he can answer again.
      await dave.open(projectId, approval.id, approval.number);
      await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(dave.proposalNotAcceptedNote()).toContainText(UI.notes.notAccepted);
      await expect(dave.proposalNotAcceptedNote()).toContainText(UI.notes.notAcceptedBelow);
      await expect(dave.proposalNotAcceptedNote()).not.toContainText(/new planned window/i);
      await expect(dave.proposalWaitingNote()).toHaveCount(0);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(dave.button(name), `${name} is back for dave`).toBeVisible();
      }
      await shot(page, "24-dave-not-accepted-asked-again");
      // The dialog starts from WSO2's window.
      await dave.button(UI.buttons.proposeNewTime).click();
      const zone = await dave.proposeTimeZone();
      await expect(dave.proposedStart()).toHaveValue(wallTime(new Date(wso2.startUtc), zone));
      await dave.proposeDialog().getByRole("button", { name: UI.propose.cancel, exact: true }).click();

      // erin approves WSO2's window: Scheduled at exactly it.
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.proposalNotAcceptedNote()).toBeVisible();
      await erin.button(UI.buttons.approve).click();
      await expect(erin.banner(UI.banners.approved)).toBeVisible();
      await expect(erin.currentStage()).toHaveText(UI.stages.scheduled);
      const done = await changeRequestRow(approval.id);
      expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", wso2.startUtc, wso2.endUtc]);
      // History stays history: the note goes with the state, and nothing claims WSO2 accepted anything.
      await expect(erin.proposalNotAcceptedNote()).toHaveCount(0);
      await expect(erin.windowAcceptedNote()).toHaveCount(0);
    } finally {
      await erinContext.close();
    }
  });

  test(`DECLINE: WSO2 keeps its window -> the customers' live requests are untouched, the page says the proposal was not accepted, and the customer proposes again and is accepted`, async ({
    page,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);
    const planned = await changeRequestRow(approval.id);
    const rowsBefore = await approverRows(approval.id);
    const first = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
    await davePropose(approval.id, first.startUtc);

    const declined = await staffDeclinesProposal("alice", approval.id);
    expect(declined.status, JSON.stringify(declined.body)).toBe(200);

    // Only the answer was written: the window, every stage and every request are as they were.
    expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: planned.startUtc, endUtc: planned.endUtc });
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: first.startUtc, answer: "DISAGREE" });
    expect(await approverRows(approval.id), "a decline touches no stage and no request").toEqual(rowsBefore);
    expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer, "the customer keeps their live request").toBe(true);

    // The page: not accepted, the CURRENT window below (never "new": it did not change), the answers on offer.
    await dave.open(projectId, approval.id, approval.number);
    await expect(dave.proposalNotAcceptedNote()).toContainText(UI.notes.notAccepted);
    await expect(dave.proposalNotAcceptedNote()).toContainText(UI.notes.notAcceptedBelow);
    await expect(dave.proposalNotAcceptedNote()).not.toContainText(/new planned window/i);
    for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
      await expect(dave.button(name), name).toBeVisible();
    }

    // The same time again is refused in the dialog's words by the service (WSO2 asked for another); another start is a new proposal.
    const sameAgain = await customerApi("dave").patch(approval.id, { plannedStartOn: first.startUtc, plannedEndOn: addHours(first.startUtc, LENGTH_HOURS) });
    expect(sameAgain.status, JSON.stringify(sameAgain.body)).toBe(400);
    expect(JSON.stringify(sameAgain.body)).toMatch(/WSO2 asked for a different time than that one/);

    await dave.button(UI.buttons.proposeNewTime).click();
    const zone = await dave.proposeTimeZone();
    const second = futureWindow(zone, { daysAhead: 42, startHour: 11, hours: LENGTH_HOURS });
    await dave.fillProposedStart(second.start);
    await dave.submitProposalButton().click();
    await expect(dave.banner(UI.banners.proposed)).toBeVisible({ timeout: 20_000 });

    // A new proposal clears the answer: it waits again, and the not-accepted note goes.
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: second.startUtc, answer: "" });
    await expect(dave.proposalWaitingNote()).toContainText(UI.notes.waitingOwn);
    await expect(dave.proposalNotAcceptedNote()).toHaveCount(0);
    expect(await approverRows(approval.id), "still no stage and no request touched").toEqual(rowsBefore);

    // WSO2 accepts the second proposal: Scheduled at it.
    const accepted = await staffAcceptsProposal("alice", approval.id);
    expect(accepted.status, JSON.stringify(accepted.body)).toBe(200);
    const done = await changeRequestRow(approval.id);
    expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", second.startUtc, addHours(second.startUtc, LENGTH_HOURS)]);
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: second.startUtc, answer: "AGREE" });
  });

  test(`a STANDARD change (${standardNew.number}) is the same: the same dialog copy, the proposal waits in Customer Approval, and accepting it schedules it`, async ({
    page,
  }) => {
    // Request Approval on a Standard change with Customer Approval ticked lands it straight in Customer Approval.
    const requested = await patchAsStaff(STAFF_APPROVERS.alice, standardNew.id, { state: "assess" });
    expect(requested.status, JSON.stringify(requested.body)).toBe(200);
    const plannedStart = futureWindow("UTC", { daysAhead: 30, startHour: 10, hours: LENGTH_HOURS });
    await planWindow(standardNew.id, { startUtc: plannedStart.startUtc, endUtc: plannedStart.endUtc });
    const rowsBefore = await approverRows(standardNew.id);

    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, standardNew.id, standardNew.number);
    await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
    await dave.button(UI.buttons.proposeNewTime).click();
    // Nothing in the dialog depends on the type.
    await expect(dave.proposeDialog()).toContainText(UI.propose.notice);
    await expect(dave.proposeDialog()).not.toContainText(/internally/i);
    const zone = await dave.proposeTimeZone();
    const proposal = futureWindow(zone, { daysAhead: 35, startHour: 9, hours: LENGTH_HOURS });
    await dave.fillProposedStart(proposal.start);
    // Enter in the field submits, like the button (the field and the button are one form).
    await dave.proposedStart().press("Enter");
    await expect(dave.banner(UI.banners.proposed)).toBeVisible({ timeout: 20_000 });
    await expect(dave.proposeDialog()).toBeHidden();

    await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
    await expect(dave.proposalWaitingNote()).toContainText(UI.notes.waitingOwn);
    for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
      await expect(dave.button(name), `${name} stays for dave`).toBeVisible();
    }
    expect(await approverRows(standardNew.id), "a Standard change's proposal re-asks nobody either").toEqual(rowsBefore);
    expect((await changeRequestRow(standardNew.id)).state).toBe("CUSTOMER_APPROVAL");

    const accepted = await staffAcceptsProposal("alice", standardNew.id);
    expect(accepted.status, JSON.stringify(accepted.body)).toBe(200);
    const done = await changeRequestRow(standardNew.id);
    expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", proposal.startUtc, addHours(proposal.startUtc, LENGTH_HOURS)]);
    await dave.open(projectId, standardNew.id, standardNew.number);
    await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
    await expect(dave.windowAcceptedNote()).toContainText(UI.notes.windowAccepted);
  });

  test(`a plain Re-schedule by WSO2 (nothing proposed) asks the customers again of the new window: still Customer Approval, no CAB, fresh requests`, async ({
    page,
  }) => {
    const rowsBefore = await approverRows(approval.id);
    const window = futureWindow("UTC", { daysAhead: 50, startHour: 13, hours: 2 });
    const rescheduled = await staffReschedules("alice", approval.id, window);
    expect(rescheduled.status, JSON.stringify(rescheduled.body)).toBe(200);

    expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: window.startUtc, endUtc: window.endUtc });
    expect(await proposalRow(approval.id), "no proposal was made, none is recorded").toEqual({ proposedUtc: "", answer: "" });
    expect(await customerRows(approval.id)).toEqual(["dave.mendis|CANCELLED", "erin.jayawardena|CANCELLED", "dave.mendis|REQUESTED", "erin.jayawardena|REQUESTED"]);
    expect((await approverRows(approval.id)).filter((r) => r.stage !== "Customer Approval")).toEqual(rowsBefore.filter((r) => r.stage !== "Customer Approval"));

    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
    for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
      await expect(dave.button(name), name).toBeVisible();
    }
    // Nothing was proposed: no note about a proposal, and no internal review either.
    await expect(dave.proposalWaitingNote()).toHaveCount(0);
    await expect(dave.proposalNotAcceptedNote()).toHaveCount(0);
    await expect(dave.internalReviewNote()).toHaveCount(0);
    await dave.button(UI.buttons.approve).click();
    await expect(dave.banner(UI.banners.approved)).toBeVisible();
    expect(await changeRequestRow(approval.id)).toMatchObject({ state: "SCHEDULED", startUtc: window.startUtc, endUtc: window.endUtc });
  });

  test(`Approve while a proposal waits approves the CURRENT window, not the proposed time: Scheduled at the planned window, and the proposal stays unanswered history`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL, timezoneId: BROWSER_ZONE });
    try {
      const planned = await changeRequestRow(approval.id);
      const proposal = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
      await davePropose(approval.id, proposal.startUtc);

      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.proposalWaitingNote()).toContainText(UI.notes.approvingNow);
      await erin.button(UI.buttons.approve).click();
      await expect(erin.banner(UI.banners.approved)).toBeVisible();
      await expect(erin.currentStage()).toHaveText(UI.stages.scheduled);

      const done = await changeRequestRow(approval.id);
      expect([done.state, done.startUtc, done.endUtc], "the window erin was looking at").toEqual(["SCHEDULED", planned.startUtc, planned.endUtc]);
      expect(await proposalRow(approval.id), "the proposal was never answered").toEqual({ proposedUtc: proposal.startUtc, answer: "" });
      // Nothing says WSO2 accepted it, and the page does not show the proposal as waiting.
      const dave = new ChangeRequestDetailsPage(page);
      await dave.open(projectId, approval.id, approval.number);
      await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
      await expect(dave.proposalWaitingNote()).toHaveCount(0);
      await expect(dave.windowAcceptedNote()).toHaveCount(0);
      await expect(dave.windowProposedStart()).toHaveCount(0);
      expect((await customerApi("dave").get(approval.id)).body.customerProposal).toMatchObject({ answer: "unanswered" });
    } finally {
      await erinContext.close();
    }
  });

  test(`${approval.number} on hold: Propose New Time is off with the reason beside it, a hold placed after the dialog was opened is refused with the reason, and Approve is still taken`, async ({
    page,
  }) => {
    const planned = await changeRequestRow(approval.id);
    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await expect(dave.button(UI.buttons.proposeNewTime)).toBeEnabled();
    await expect(dave.holdNote()).toHaveCount(0);

    // The page was opened, and the dialog filled in, BEFORE WSO2 put the change on hold:
    // the late refusal is still there for a page that could not know.
    await dave.button(UI.buttons.proposeNewTime).click();
    const zone = await dave.proposeTimeZone();
    const proposal = futureWindow(zone, { daysAhead: 40, startHour: 11, hours: LENGTH_HOURS });
    await dave.fillProposedStart(proposal.start);
    await psql(`update change_request set is_on_hold = true, on_hold_reason = 'E2E hold' where id = '${approval.id}'`);
    await dave.submitProposalButton().click();

    // The dialog stays open, so the customer can read why; nothing changed. The refusal
    // is the backend's machine-readable code (change_request_on_hold), not its wording.
    await expect(dave.proposeDialog().getByRole("alert").filter({ hasText: UI.propose.errors.onHold })).toBeVisible();
    await expect(dave.proposeDialog()).toBeVisible();
    // Focus stays in the dialog, on the button that was pressed, so the customer can go on.
    await expect(dave.submitProposalButton()).toBeFocused();
    expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: planned.startUtc, endUtc: planned.endUtc });
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: "", answer: "" });
    expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer).toBe(true);
    await dave.proposeDialog().getByRole("button", { name: UI.propose.cancel, exact: true }).click();
    await expect(dave.proposeDialog()).toBeHidden();

    // A page opened NOW knows: the detail carries the hold (never its reason), Propose New
    // Time is switched off with the reason beside it, and nothing opens when it is clicked.
    const detail = await customerApi("dave").get(approval.id);
    expect(detail.body.isOnHold, "the detail says the change is held").toBe(true);
    expect(JSON.stringify(detail.body), "WSO2's reason for the hold stays internal").not.toContain("E2E hold");
    await dave.open(projectId, approval.id, approval.number);
    const propose = dave.button(UI.buttons.proposeNewTime);
    await expect(propose).toBeDisabled();
    await expect(dave.holdNote()).toBeVisible();
    await expect(propose).toHaveAccessibleDescription(UI.notes.onHold);
    await propose.click({ force: true });
    await expect(dave.proposeDialog()).toBeHidden();

    // The reject confirmation does not point at the switched-off button: it says why instead.
    await dave.button(UI.buttons.reject).click();
    const rejectDialog = dave.rejectDialog(UI.rejectConfirm.approvalTitle);
    await expect(rejectDialog).toContainText(UI.rejectConfirm.approvalMessage);
    await expect(rejectDialog).toContainText(UI.rejectConfirm.approvalHintOnHold);
    await expect(rejectDialog).not.toContainText(UI.rejectConfirm.approvalHint);
    await rejectDialog.getByRole("button", { name: UI.rejectConfirm.goBack, exact: true }).click();
    await expect(rejectDialog).toBeHidden();
    await expect(dave.button(UI.buttons.reject), "focus returns to the button that opened it").toBeFocused();

    // Approve and Reject are still there, and Approve is still taken while the change is on hold.
    await expect(dave.button(UI.buttons.reject)).toBeEnabled();
    await dave.button(UI.buttons.approve).click();
    await expect(dave.banner(UI.banners.approved)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
  });

  test(`a change request with NO planned window has nothing to move: Propose New Time is off with the reason beside it, the service refuses a proposal in words, and Approve is still taken`, async ({
    page,
  }) => {
    await psql(`update change_request set start_on = NULL, end_on = NULL where id = '${approval.id}'`);
    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    const propose = dave.button(UI.buttons.proposeNewTime);
    await expect(propose).toBeDisabled();
    await expect(dave.noWindowNote()).toBeVisible();
    await expect(propose).toHaveAccessibleDescription(UI.notes.noWindow);
    await expect(dave.holdNote()).toHaveCount(0);
    await propose.click({ force: true });
    await expect(dave.proposeDialog()).toBeHidden();

    // A proposal sent anyway is refused with the service's words, and writes nothing.
    const start = futureWindow("UTC", { daysAhead: 40, startHour: 11, hours: LENGTH_HOURS });
    const refused = await customerApi("dave").patch(approval.id, { plannedStartOn: start.startUtc, plannedEndOn: start.endUtc });
    expect(refused.status, JSON.stringify(refused.body)).toBe(409);
    expect(JSON.stringify(refused.body)).toMatch(/has no planned window to move, so a new time cannot be proposed for it/);
    expect((refused.body as { errorCode?: string }).errorCode, "the refusal is named by its code, which is what the page classifies it by").toBe("change_request_no_planned_window");
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: "", answer: "" });
    expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer).toBe(true);

    await expect(dave.button(UI.buttons.reject)).toBeEnabled();
    await dave.button(UI.buttons.approve).click();
    await expect(dave.banner(UI.banners.approved)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
  });

  test(`a page opened before ${approval.number} was re-scheduled cannot approve the new window: the schedule-changed message, nothing recorded, and the page shows the new window`, async ({
    browser,
    baseURL,
  }) => {
    // dave's tab stays on what it loaded (every GET of the change request is served the first
    // answer until his PATCH is sent): a tab left open, without a race against the page's refetching.
    const daveContext = await openLocalContext(test, browser, "dave", { baseURL, timezoneId: BROWSER_ZONE });
    try {
      const davePage = await daveContext.newPage();
      let frozenBody: string | undefined;
      let frozen = true;
      const detailUrl = `${(await stackEndpoints()).customerApi}/change-requests/${approval.id}`;
      await davePage.route(detailUrl, async (route) => {
        const method = route.request().method();
        if (method === "PATCH") frozen = false;
        if (method !== "GET") return route.continue();
        const response = await route.fetch();
        const body = await response.text();
        frozenBody ??= body;
        return route.fulfill({ response, body: frozen ? frozenBody : body });
      });
      const dave = new ChangeRequestDetailsPage(davePage);
      await dave.open(projectId, approval.id, approval.number);
      await expect(dave.button(UI.buttons.approve)).toBeVisible();

      // erin proposes a start, and WSO2 answers with a window of its own: the customers are asked again of THAT.
      const proposal = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
      await davePropose(approval.id, proposal.startUtc, "erin");
      const second = futureWindow("UTC", { daysAhead: 46, startHour: 14, hours: 2 });
      const countered = await staffCountersProposal("alice", approval.id, second);
      expect(countered.status, JSON.stringify(countered.body)).toBe(200);
      expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer, "dave is asked again").toBe(true);
      await expect(dave.button(UI.buttons.approve), "dave's stale tab still offers Approve").toBeVisible();

      // The stale page's Approve is for the window it showed: refused, with the reason.
      await dave.button(UI.buttons.approve).click();
      await expect(dave.banner(UI.banners.scheduleChanged)).toBeVisible();
      // A refused answer does not leave focus on a control that is about to be redrawn: it is on the heading.
      await expect(dave.heading()).toBeFocused();
      expect(await changeRequestRow(approval.id), "nothing was approved").toMatchObject({
        state: "CUSTOMER_APPROVAL",
        startUtc: second.startUtc,
        endUtc: second.endUtc,
      });
      expect(await customerRows(approval.id)).toEqual(["dave.mendis|CANCELLED", "erin.jayawardena|CANCELLED", "dave.mendis|REQUESTED", "erin.jayawardena|REQUESTED"]);

      // The page refreshed on the refusal: it now shows the new window, and Approve for THAT window is taken.
      await expect(dave.button(UI.buttons.approve)).toBeVisible();
      await dave.button(UI.buttons.approve).click();
      await expect(dave.banner(UI.banners.approved)).toBeVisible();
      await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
      expect(await changeRequestRow(approval.id)).toMatchObject({ state: "SCHEDULED", startUtc: second.startUtc, endUtc: second.endUtc });
    } finally {
      await daveContext.close();
    }
  });

  test(`the loop can be repeated: dave proposes, WSO2 counters, dave proposes again from WSO2's window (the end follows the start), WSO2 accepts, and the history keeps every round`, async ({
    page,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);

    // Round one: dave proposes T1, WSO2 counters with W2.
    const first = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
    await davePropose(approval.id, first.startUtc);
    const wso2 = futureWindow("UTC", { daysAhead: 44, startHour: 10, hours: 4 });
    expect((await staffCountersProposal("alice", approval.id, wso2)).status).toBe(200);

    // Round two: the dialog starts from WSO2's window and keeps ITS length (four hours); the end follows the new start.
    await dave.open(projectId, approval.id, approval.number);
    await expect(dave.proposalNotAcceptedNote()).toBeVisible();
    await dave.button(UI.buttons.proposeNewTime).click();
    const zone = await dave.proposeTimeZone();
    await expect(dave.proposedStart()).toHaveValue(wallTime(new Date(wso2.startUtc), zone));
    await expect(dave.proposedEnd()).toHaveValue(wallTime(new Date(wso2.endUtc), zone));
    const second = futureWindow(zone, { daysAhead: 48, startHour: 9, hours: 4 });
    await dave.fillProposedStart(second.start);
    await expect(dave.proposedEnd(), "the end follows the start, same length").toHaveValue(second.end);
    await dave.submitProposalButton().click();
    await expect(dave.banner(UI.banners.proposed)).toBeVisible({ timeout: 20_000 });
    expect(await proposalRow(approval.id), "the new proposal replaced the answered one").toEqual({ proposedUtc: second.startUtc, answer: "" });
    await expect(dave.proposalWaitingNote()).toContainText(UI.notes.waitingOwn);
    await expect(dave.proposalNotAcceptedNote()).toHaveCount(0);

    // WSO2 accepts the second proposal: Scheduled at it, with WSO2's four-hour length.
    expect((await staffAcceptsProposal("bob", approval.id)).status).toBe(200);
    const done = await changeRequestRow(approval.id);
    expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", second.startUtc, addHours(second.startUtc, 4)]);
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: second.startUtc, answer: "AGREE" });
    expect(
      (await approverRows(approval.id)).map((r) => `${r.stage}|${r.email.split("@")[0]}|${r.state}`),
      "every round stays on record: the customers' cancelled requests and the one WSO2's counter asked; no CAB stage at all",
    ).toEqual([
      "Customer Approval|dave.mendis|CANCELLED",
      "Customer Approval|erin.jayawardena|CANCELLED",
      "Customer Approval|dave.mendis|CANCELLED",
      "Customer Approval|erin.jayawardena|CANCELLED",
    ]);
    await dave.open(projectId, approval.id, approval.number);
    await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
    await expect(dave.windowAcceptedNote()).toContainText(UI.notes.windowAccepted);
  });

  test(`the service's refusals of a proposal, in its words, change nothing: the planned start, the standing proposal, an end that is not the planned length, an end alone, and a past start`, async () => {
    const dave = customerApi("dave");
    const planned = await changeRequestRow(approval.id);
    const rowsBefore = await approverRows(approval.id);
    const start = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
    const unchanged = async () => {
      expect(await changeRequestRow(approval.id)).toMatchObject({ state: "CUSTOMER_APPROVAL", startUtc: planned.startUtc, endUtc: planned.endUtc });
      expect(await approverRows(approval.id)).toEqual(rowsBefore);
    };

    const refusals: Array<[string, unknown, RegExp]> = [
      ["the planned start itself", { plannedStartOn: planned.startUtc, plannedEndOn: planned.endUtc }, /plannedStartOn is the planned start already: propose a different start/],
      ["an end alone", { plannedEndOn: start.endUtc }, /a proposed implementation time needs a new start: send plannedStartOn/],
      ["an end that is not the planned length", { plannedStartOn: start.startUtc, plannedEndOn: addHours(start.startUtc, LENGTH_HOURS + 3) }, /moves the start and keeps the planned length of .*: plannedEndOn must be /],
      ["a start in the past", { plannedStartOn: "2020-01-01 10:00:00", plannedEndOn: "2020-01-01 12:00:00" }, /in the past/],
    ];
    for (const [name, body, says] of refusals) {
      const refused = await dave.patch(approval.id, body);
      expect(refused.status, `${name}: ${JSON.stringify(refused.body)}`).toBe(400);
      expect(JSON.stringify(refused.body), name).toMatch(says);
      await unchanged();
    }
    expect(await proposalRow(approval.id), "no refusal wrote a proposal").toEqual({ proposedUtc: "", answer: "" });

    // The start alone is a proposal (the end is derived), and the same one again is refused as already waiting.
    const ok = await dave.patch(approval.id, { plannedStartOn: start.startUtc });
    expect(ok.status, JSON.stringify(ok.body)).toBe(200);
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: start.startUtc, answer: "" });
    const again = await dave.patch(approval.id, { plannedStartOn: start.startUtc, plannedEndOn: start.endUtc });
    expect(again.status, JSON.stringify(again.body)).toBe(400);
    expect(JSON.stringify(again.body)).toMatch(/that time is already proposed and is waiting for WSO2's response/);
  });

  test(`WSO2 cannot answer a proposal it did not see, or one that is not there: a stale proposal, a stale window and nothing waiting are 409s in the service's words, and nothing changes`, async () => {
    const rows = async () => ({ row: await changeRequestRow(approval.id), proposal: await proposalRow(approval.id), approvers: await approverRows(approval.id) });
    const alice = staffApi("alice");

    // Nothing waiting: neither Accept nor a counter that names a proposal is possible.
    const planned = await changeRequestRow(approval.id);
    const nothing = await alice.patch(approval.id, {
      confirmCustomerUpdatedDate: "agree",
      expectedCustomerUpdatedOn: planned.startUtc,
      expectedPlannedStartOn: planned.startUtc,
      expectedPlannedEndOn: planned.endUtc,
    });
    expect(nothing.status, JSON.stringify(nothing.body)).toBe(409);
    expect(JSON.stringify(nothing.body)).toMatch(/no new time proposed by the customer is waiting for a response on this change request/);

    const first = futureWindow("UTC", { daysAhead: 40, startHour: 16, hours: LENGTH_HOURS });
    await davePropose(approval.id, first.startUtc);
    const before = await rows();

    // A page opened before the customer proposed ANOTHER time never answers the new one.
    const second = futureWindow("UTC", { daysAhead: 41, startHour: 16, hours: LENGTH_HOURS });
    const stale = await alice.patch(approval.id, {
      confirmCustomerUpdatedDate: "agree",
      expectedCustomerUpdatedOn: second.startUtc,
      expectedPlannedStartOn: planned.startUtc,
      expectedPlannedEndOn: planned.endUtc,
    });
    expect(stale.status, JSON.stringify(stale.body)).toBe(409);
    expect(JSON.stringify(stale.body)).toMatch(/the customer's proposed time changed after you opened this change request/);
    // ...and a stale planned window is refused as well.
    const staleWindow = await alice.patch(approval.id, {
      confirmCustomerUpdatedDate: "agree",
      expectedCustomerUpdatedOn: first.startUtc,
      expectedPlannedStartOn: second.startUtc,
      expectedPlannedEndOn: second.endUtc,
    });
    expect(staleWindow.status, JSON.stringify(staleWindow.body)).toBe(409);
    expect(JSON.stringify(staleWindow.body)).toMatch(/read it again before responding/);
    // A counter that does not name the proposal it saw is refused too: an old page never answers a proposal it did not see.
    const blind = await alice.patch(approval.id, { state: "authorize", plannedStartOn: second.startUtc, plannedEndOn: second.endUtc });
    expect(blind.status, JSON.stringify(blind.body)).toBe(409);
    expect(JSON.stringify(blind.body)).toMatch(/the customer proposed a new time \(.*\) after you opened this change request/);
    // Accept is no bypass: a state named scheduled out of Customer Approval stays refused for staff, proposal or not.
    const bypass = await alice.patch(approval.id, { state: "scheduled" });
    expect(bypass.status, JSON.stringify(bypass.body)).toBeGreaterThanOrEqual(400);
    expect(await rows(), "nothing any refusal said changed anything").toEqual(before);

    // A customer cannot answer for WSO2: the field is not theirs to send.
    const forged = await customerApi("dave").patch(approval.id, {
      confirmCustomerUpdatedDate: "agree",
      expectedCustomerUpdatedOn: first.startUtc,
    });
    expect(forged.status, JSON.stringify(forged.body)).toBe(403);
    expect(await rows()).toEqual(before);
  });
});
