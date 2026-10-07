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
// WHO MAY NOT: the negative side of a customer answering a change request, on the
// real local stack.
//
// A customer sees a change request exactly when it was DESIGNATED to them: it reached
// Customer Approval and/or Customer Review and they were one of the contacts asked
// (they hold, or ever held, an approver row on a Customer Approval / Customer Review
// stage of it, in any status), and it is still on a project they are a registered
// contact of. Once designated it stays visible in every later state. Anything else is
// not there for them at all: absent from the list and the counts, and a 404 on the
// detail, the approvals, the decision and the PATCH, whatever address they type.
//
//   1. A customer of ANOTHER project (mira.santos, "Lumen Works Platform") cannot see
//      Example Corp's CHG-FIXED-007 by any route: 404 on the by-id read, the approvals
//      and the answer, absent from every list, an error page at either address she
//      types. (She is refused on the application's own visibility rule, so this holds
//      whether or not the database enforces row-level security for the role
//      entity-service connects as.)
//   2. A change request that was NEVER ASKED of dave is not there for him, whatever
//      state it is in: CHG-FIXED-005 (New) and CHG-FIXED-006 (Review, Customer Review
//      ticked, nobody asked yet). Not opened by its address (the not-found page), not
//      listed, not answerable (404, not a refusal that admits it exists). And moving
//      one on, to Customer Approval, does not designate anybody: the state alone never
//      shows a change request to a customer.
//   3. A designated change request STAYS visible, in every later state: Authorize (after
//      a proposed time), Scheduled, Implement, Review, Customer Review, Rollback, Closed,
//      Canceled. And a contact whose OWN request was cancelled (a colleague answered, or
//      the change was re-scheduled) still sees it, with nothing to answer: her answer is
//      refused.
//   4. A direct PATCH from a customer's token that carries anything but an answer, or a
//      proposed time, is refused (403), and the two mixed forms are refused (400);
//      nothing about the change moves.
//
// ⚠️ STATE-CHANGING in the small (it re-seeds first, and some tests move a fixture on or
// cancel erin's request in the database), so it needs E2E_POSTGRES_CONTAINER and SKIPS
// without it. It needs an entity-service that applies the visibility rule (and a seed
// whose Customer Approval / Customer Review fixtures carry their approver rows).
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, openLocalContext, withLocalSession } from "../../auth/localSessions";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import {
  FIXTURES,
  approverRows,
  changeRequestRow,
  customerApi,
  psql,
  resetFixtures,
  withFixtureStack,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test);

const { approval, review, inReview, standardNew, projectId } = FIXTURES;

/** What the Lumen Works Platform project is called (its id is random per database). */
const LUMEN_PROJECT = LOCAL_PERSONAS.mira.project;

test.describe("Local stack — who may not answer a change request", () => {
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(async () => {
    await resetFixtures();
  });

  test(`${LOCAL_PERSONAS.mira.email}, a customer of another project, is never offered the answer, cannot list ${approval.number} and is refused when she tries to answer or propose`, async ({
    browser,
    baseURL,
  }) => {
    const mira = customerApi("mira");
    const lumenId = await mira.projectIdByName(LUMEN_PROJECT);
    expect(lumenId, `${LUMEN_PROJECT} is not one of mira's projects: is the stack seeded?`).toBeTruthy();

    // 1. The API. Another project's change request does not exist for her: a 404 by id and in
    // its approvals, on every stack (it is the application's own visibility rule, not the
    // database's row-level security, that says so).
    const read = await mira.get(approval.id);
    expect(read.status, "another project's change request must not exist for her").toBe(404);
    const stages = await mira.approvals(approval.id);
    expect(stages.status, "another project's approvals must not be readable for her (and never an empty 200)").toBe(404);
    const decided = await mira.decision(approval.id, { decision: "approved" });
    expect(decided.status, "another project's approval cannot be decided by her").toBe(404);

    // 2. She cannot list it: neither under her own project nor under Example Corp's.
    for (const id of [lumenId!, projectId]) {
      const numbers = await mira.listedNumbers(id);
      expect(
        numbers.filter((n) => n.startsWith("CHG-FIXED")),
        `Example Corp's change requests are listed for mira under project ${id}`,
      ).toEqual([]);
    }

    // 3. She cannot answer it, and cannot propose a time for it.
    const attempts: [string, unknown][] = [
      ["approve", { isCustomerApproved: true }],
      ["reject", { isCustomerApproved: false }],
      ["propose", { plannedStartOn: "2027-03-01 10:00:00", plannedEndOn: "2027-03-01 12:00:00" }],
    ];
    for (const [what, body] of attempts) {
      const result = await mira.patch(approval.id, body);
      expect(result.status, `mira's "${what}" answered ${result.status}: ${JSON.stringify(result.body)}`).toBe(404);
    }
    const untouched = await changeRequestRow(approval.id);
    expect([untouched.state, untouched.startUtc, untouched.endUtc]).toEqual(["CUSTOMER_APPROVAL", "", ""]);
    expect(
      (await approverRows(approval.id)).map((r) => `${r.email}|${r.state}`),
      "the two contacts' requests are untouched",
    ).toEqual(["dave.mendis@example.com|REQUESTED", "erin.jayawardena@example.com|REQUESTED"]);

    // 4. The browser, at the address she would type: under her own project and under Example Corp's.
    const context = await openLocalContext(test, browser, "mira", { baseURL });
    try {
      const page = await context.newPage();
      const details = new ChangeRequestDetailsPage(page);
      const own = await details.openAndSettle(lumenId!, approval.id, approval.number);
      await expect(details.answerButtons(), "buttons offered to a customer of another project").toHaveCount(0);
      expect(own.loaded, "another project's change request rendered").toBe(false);

      // Example Corp's address: the project itself is refused her (404), so no page is built.
      const [projectRefusal] = await Promise.all([
        page.waitForResponse(
          (r) => new URL(r.url()).pathname.endsWith(`/projects/${projectId}`) && r.request().method() === "GET",
          { timeout: 60_000 },
        ),
        page.goto(`/projects/${projectId}/operations/change-requests/${approval.id}`),
      ]);
      expect(projectRefusal.status(), "Example Corp's project must be refused to a customer who is not its contact").toBe(404);
      await expect(details.answerButtons(), "buttons offered under Example Corp's address").toHaveCount(0);

      // Her own list shows none of Example Corp's change requests (when her project has the page at all).
      const list = new ChangeRequestsPage(page);
      const listed = await list.open(lumenId!).then(() => true, () => false);
      if (listed) {
        await list.waitForList();
        await expect(page.getByText(/CHG-FIXED-\d+/)).toHaveCount(0);
      } else {
        test.info().annotations.push({
          type: "note",
          description: `${LUMEN_PROJECT} has no change requests page; the API listing above is the assertion`,
        });
      }
    } finally {
      await context.close();
    }
  });

  test(`${standardNew.number} (New) and ${inReview.number} (Review, nobody asked yet) were never asked of dave: they are not there for him, by address, in a list or by any call, and moving one on does not show it`, async ({
    page,
  }) => {
    const dave = customerApi("dave");
    const details = new ChangeRequestDetailsPage(page);

    for (const change of [standardNew, inReview]) {
      // The API: 404 on the read, the approvals (never an empty 200), the decision and the PATCH.
      expect((await dave.get(change.id)).status, `${change.number} read`).toBe(404);
      expect((await dave.approvals(change.id)).status, `${change.number} approvals`).toBe(404);
      expect((await dave.decision(change.id, { decision: "approved" })).status, `${change.number} decision`).toBe(404);
      for (const body of [{ isCustomerApproved: true }, { isCustomerReviewed: true }, { plannedStartOn: "2030-03-01 10:00:00" }]) {
        const result = await dave.patch(change.id, body);
        expect(result.status, `${change.number} ${JSON.stringify(body)}: ${JSON.stringify(result.body)}`).toBe(404);
      }

      // The page, at the address he would type: the not-found page, nothing of the change request.
      const opened = await details.openAndSettle(projectId, change.id, change.number);
      expect(opened.loaded, `${change.number} rendered for a customer it was never asked of`).toBe(false);
      await expect(details.answerButtons(), `${change.number} offers an answer`).toHaveCount(0);
    }

    // Not in the list, not in any state filter (Review is id 0, New is -5), not in the counts.
    for (const filters of [{}, { stateKeys: [0] }, { stateKeys: [-5] }, { stateKeys: [-5, -4, -3, 5, -2, -1, 0, 1, 2, 3, 4] }]) {
      const listed = await dave.listedNumbers(projectId, filters);
      for (const change of [standardNew, inReview]) {
        expect(listed, `${change.number} listed for dave under ${JSON.stringify(filters)}`).not.toContain(change.number);
      }
    }

    // Moving a change request to a customer state does not designate anybody: the state alone
    // shows nothing to a customer. (inReview, with no customer stage, is put into Customer
    // Review straight in the database, which asks nobody.)
    await psql(`update change_request set state = 'CUSTOMER_REVIEW' where id = '${inReview.id}'`);
    await psql(`update change_request set state = 'CUSTOMER_APPROVAL' where id = '${standardNew.id}'`);
    for (const change of [standardNew, inReview]) {
      expect((await dave.get(change.id)).status, `${change.number} read in a customer state`).toBe(404);
      expect((await dave.patch(change.id, { isCustomerApproved: true })).status).toBe(404);
      expect(await dave.listedNumbers(projectId), `${change.number} listed in a customer state`).not.toContain(change.number);
    }
    expect((await changeRequestRow(standardNew.id)).state).toBe("CUSTOMER_APPROVAL");
    expect((await changeRequestRow(inReview.id)).state).toBe("CUSTOMER_REVIEW");
  });

  test(`${approval.number}, once asked of dave, stays visible to him in every later state, Authorize and the terminal ones included, with nothing to answer`, async ({
    page,
  }) => {
    const dave = customerApi("dave");
    const details = new ChangeRequestDetailsPage(page);
    const states = [
      ["AUTHORIZE", UI.stages.authorize],
      ["SCHEDULED", UI.stages.scheduled],
      ["IMPLEMENT", UI.stages.implement],
      ["REVIEW", UI.stages.review],
      ["CUSTOMER_REVIEW", UI.stages.customerReview],
      ["ROLLBACK", UI.stages.rollback],
      ["CLOSED", UI.stages.closed],
      ["CANCELED", UI.stages.canceled],
    ] as const;
    for (const [state, stage] of states) {
      await psql(`update change_request set state = '${state}' where id = '${approval.id}'`);
      const read = await dave.get(approval.id);
      expect(read.status, `${approval.number} in ${state}: ${JSON.stringify(read.body)}`).toBe(200);
      // Visible, but nothing to answer: the only request ever made of dave belongs to the state it
      // was made in (Customer Approval), so no other state offers him an answer.
      expect(read.body.customerCanAnswer, `${approval.number} in ${state}`).toBe(false);
      expect((await dave.approvals(approval.id)).status, `${approval.number} approvals in ${state}`).toBe(200);
      expect(await dave.listedNumbers(projectId), `${approval.number} listed in ${state}`).toContain(approval.number);
      expect(await dave.listedNumbers(projectId, { stateKeys: [-3, 5, -2, -1, 0, 1, 2, 3, 4] }), `${approval.number} listed by state in ${state}`)
        .toContain(approval.number);
      await details.open(projectId, approval.id, approval.number);
      await expect(details.currentStage(), `${approval.number} in ${state}`).toHaveText(stage);
      await expect(details.answerButtons()).toHaveCount(0);
    }
  });

  test(`a contact whose own request on ${approval.number} was cancelled still sees it, with nothing to answer, and her answer is refused`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);

    // In Customer Approval, but erin's own request was cancelled while dave's stands
    // (what a colleague's answer or a re-schedule leaves behind): the offer is per viewer, and
    // the cancelled row still designates her.
    await psql(
      `update approval_stage_approver a set state = 'CANCELLED' from "user" u ` +
        `where a.approver_user_id = u.id and a.work_item_id = '${approval.id}' ` +
        `and u.email = '${LOCAL_PERSONAS.erin.email}'`,
    );
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL });
    try {
      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.customerApproval);
      await expect(erin.answerButtons(), "erin has no pending request but was offered an answer").toHaveCount(0);
      expect((await customerApi("erin").get(approval.id)).body.customerCanAnswer).toBe(false);
      expect(await customerApi("erin").listedNumbers(projectId), "a cancelled request un-designated her").toContain(approval.number);

      await dave.open(projectId, approval.id, approval.number);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(dave.button(name), `${name} is missing for the contact who is still asked`).toBeVisible();
      }
      expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer).toBe(true);

      // And the backend agrees with the page, not just the page with itself: erin's answer is refused.
      const refused = await customerApi("erin").patch(approval.id, { isCustomerApproved: true });
      expect([403, 409], `erin's answer without a pending request: ${JSON.stringify(refused.body)}`).toContain(
        refused.status,
      );
      expect((await changeRequestRow(approval.id)).state).toBe("CUSTOMER_APPROVAL");
    } finally {
      await erinContext.close();
    }
  });

  test(`at Customer Review (${review.number}) a contact whose own request was cancelled is offered neither Successful nor Unsuccessful, and her answer is refused, while dave who is still asked is`, async ({
    page,
    browser,
    baseURL,
  }) => {
    // The gate at Customer Approval is per viewer (customerCanAnswer), and so is Customer Review's: it
    // is not "always on". erin's request on this review is cancelled (what a sibling's answer leaves);
    // dave's stands.
    await psql(
      `update approval_stage_approver a set state = 'CANCELLED' from "user" u ` +
        `where a.approver_user_id = u.id and a.work_item_id = '${review.id}' ` +
        `and u.email = '${LOCAL_PERSONAS.erin.email}'`,
    );
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL });
    try {
      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      await erin.open(projectId, review.id, review.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.customerReview);
      await expect(erin.answerButtons(), "erin has no pending request but was offered an answer").toHaveCount(0);
      expect((await customerApi("erin").get(review.id)).body.customerCanAnswer).toBe(false);
      expect(await customerApi("erin").listedNumbers(projectId), "a cancelled request un-designated her").toContain(review.number);

      const dave = new ChangeRequestDetailsPage(page);
      await dave.open(projectId, review.id, review.number);
      await expect(dave.button(UI.buttons.successful)).toBeVisible();
      await expect(dave.button(UI.buttons.unsuccessful)).toBeVisible();
      expect((await customerApi("dave").get(review.id)).body.customerCanAnswer).toBe(true);

      // The backend agrees with the page: erin's answer is refused and moves nothing.
      const refused = await customerApi("erin").patch(review.id, { isCustomerReviewed: true });
      expect([403, 409], `erin's review without a pending request: ${JSON.stringify(refused.body)}`).toContain(
        refused.status,
      );
      expect((await changeRequestRow(review.id)).state).toBe("CUSTOMER_REVIEW");
    } finally {
      await erinContext.close();
    }
  });

  test(`a direct PATCH from dave's token with anything but an answer or a proposed time is refused, and nothing about ${approval.number} moves`, async () => {
    const dave = customerApi("dave");
    const window = { plannedStartOn: "2027-03-01 10:00:00", plannedEndOn: "2027-03-01 12:00:00" };

    const forbidden: [string, unknown][] = [
      ["an answer with a title change", { isCustomerApproved: true, title: "hijacked" }],
      ["a title change alone", { title: "hijacked" }],
      ["a state change", { state: "scheduled" }],
      ["a state change to rollback", { state: "rollback" }],
      ["a proposed window with a title change", { ...window, title: "hijacked" }],
      ["a proposed window with a state", { ...window, state: "authorize" }],
      ["a description change", { description: "hijacked" }],
    ];
    for (const [what, body] of forbidden) {
      const result = await dave.patch(approval.id, body);
      expect(result.status, `${what}: ${JSON.stringify(result.body)}`).toBe(403);
    }

    // The two mixed forms are told to be sent as separate requests / one answer at a time.
    const mixedProposal = await dave.patch(approval.id, { isCustomerApproved: true, ...window });
    expect(mixedProposal.status, JSON.stringify(mixedProposal.body)).toBe(400);
    const bothAnswers = await dave.patch(approval.id, { isCustomerApproved: true, isCustomerReviewed: true });
    expect(bothAnswers.status, JSON.stringify(bothAnswers.body)).toBe(400);

    // An answer for the wrong stage is a conflict, not an approval of this one.
    const wrongStage = await dave.patch(approval.id, { isCustomerReviewed: true });
    expect(wrongStage.status, JSON.stringify(wrongStage.body)).toBe(409);

    // Nothing moved.
    const row = await changeRequestRow(approval.id);
    expect([row.state, row.startUtc, row.endUtc, row.title]).toEqual([
      "CUSTOMER_APPROVAL",
      "",
      "",
      "E2E fixture: change in Customer Approval with a pending customer group approval",
    ]);
    expect(
      (await approverRows(approval.id)).map((r) => `${r.email}|${r.state}`),
      "no request was answered or cancelled",
    ).toEqual(["dave.mendis@example.com|REQUESTED", "erin.jayawardena@example.com|REQUESTED"]);
    const after = await dave.get(approval.id);
    expect(after.body.state?.label).toBe("Customer Approval");
    expect(after.body.customerCanAnswer, "dave still has his answer to give").toBe(true);
  });
});
