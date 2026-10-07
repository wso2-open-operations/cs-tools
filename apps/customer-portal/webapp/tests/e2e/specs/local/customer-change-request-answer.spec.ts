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
// A customer ANSWERS a change request in the customer portal, on the real local
// stack: approves or rejects at Customer Approval, marks the change successful
// or unsuccessful at Customer Review.
//
// Who: dave.mendis@example.com and erin.jayawardena@example.com, the two registered
// contacts of "Example Corp Production" whom the seed asks on CHG-FIXED-007
// (Customer Approval) and CHG-FIXED-008 (Customer Review).
//
// What each test proves, end to end (browser -> customer backend -> entity-service
// -> Postgres), and what it reads back to be sure the UI did not merely look right:
//   - the buttons are on the page for a contact who is asked (they were not before
//     the backend told the page so: `hasCustomerApproved` is the OUTCOME stamp, false
//     for as long as the change waits);
//   - the answer is applied: success banner, the lifecycle panel and the list move
//     on, the buttons go, the database holds the new state and each contact's row;
//   - the answer that cannot be taken back (reject, "unsuccessful") asks first, and
//     "Go back" changes nothing;
//   - the other contact, whose tab was open before the answer, gets the plain-words
//     409 and the page refreshes to the new state, instead of a silent failure.
//
// ⚠️ STATE-CHANGING. Each test starts by re-running the seed in the stack's Postgres
// (`resetFixtures`), which is why the spec needs E2E_POSTGRES_CONTAINER and SKIPS
// without it. It leaves the fixtures answered: run the seed (docker-compose up -d
// migrate) to put them back for the read-only smoke spec.
//
//   E2E_BASE_URL=http://localhost:13000 E2E_POSTGRES_CONTAINER=csmenv-postgres-1 \
//     node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium
// after minting the dave and erin sessions (tests/e2e/README.md, "Local stack").
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
  resetFixtures,
  stackEndpoints,
  withFixtureStack,
} from "../../utils/localStack";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");
withFixtureStack(test);

const { approval, review, projectId } = FIXTURES;

test.describe("Local stack — a customer answers a change request", () => {
  // A cold shell load behind the project's features, then the page, per test.
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(async () => {
    await resetFixtures();
  });

  test(`${LOCAL_PERSONAS.dave.email} approves ${approval.number}: banner, Scheduled in the page and the list, and ${LOCAL_PERSONAS.erin.email} no longer sees Approve`, async ({
    page,
    browser,
    baseURL,
  }) => {
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL });
    try {
      // erin is asked too: her page offers the answer while the change waits.
      const erin = new ChangeRequestDetailsPage(await erinContext.newPage());
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.button(UI.buttons.approve)).toBeVisible();

      const dave = new ChangeRequestDetailsPage(page);
      await dave.open(projectId, approval.id, approval.number);
      await expect(dave.currentStage()).toHaveText(UI.stages.customerApproval);
      for (const name of [UI.buttons.proposeNewTime, UI.buttons.approve, UI.buttons.reject]) {
        await expect(dave.button(name), `${name} is not offered to dave`).toBeVisible();
      }
      // Customer Approval's buttons, not Customer Review's.
      await expect(dave.reviewButtons()).toHaveCount(0);

      // The three buttons are one named group, and the window is a plan until it is scheduled.
      await expect(dave.answerGroup(UI.notes.approvalGroup).getByRole("button")).toHaveCount(3);
      await expect(page.getByText(UI.windowCard.planned, { exact: true })).toBeVisible();

      // Approving needs no confirmation: one click.
      await dave.button(UI.buttons.approve).click();
      await expect(dave.banner(UI.banners.approved)).toBeVisible();
      await expect(dave.currentStage()).toHaveText(UI.stages.scheduled);
      await expect(dave.answerButtons(), "the answer buttons stay after answering").toHaveCount(0);
      // The buttons went with the answer; focus is on the page's heading, not lost to <body>.
      await expect(dave.heading()).toBeFocused();
      await expect(page.getByText(UI.windowCard.scheduled, { exact: true })).toBeVisible();

      // What the stack recorded, not what the page shows.
      const afterApi = await customerApi("dave").get(approval.id);
      expect(afterApi.body.state?.label).toBe("Scheduled");
      expect(afterApi.body.customerCanAnswer, "nothing left for dave to answer").toBe(false);
      expect((await changeRequestRow(approval.id)).state).toBe("SCHEDULED");
      const rows = await approverRows(approval.id);
      expect(
        rows.map((r) => `${r.stage}|${r.email}|${r.state}`),
        "dave's row approved, the sibling's cancelled",
      ).toEqual([
        "Customer Approval|dave.mendis@example.com|APPROVED",
        "Customer Approval|erin.jayawardena@example.com|CANCELLED",
      ]);

      // The list moved too (the patch hook invalidates it).
      await page.getByRole("button", { name: "Back to Change Requests" }).click();
      await expect(new ChangeRequestsPage(page).rowByNumber(approval.number)).toContainText(
        UI.stages.scheduled,
        { timeout: 30_000 },
      );

      // erin, whose tab showed Approve: after a reload there is nothing to answer.
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.currentStage()).toHaveText(UI.stages.scheduled);
      await expect(erin.answerButtons()).toHaveCount(0);
    } finally {
      await erinContext.close();
    }
  });

  test(`${LOCAL_PERSONAS.erin.email}'s open tab still offers Approve when dave answers first: she gets the plain-words 409 and the page refreshes`, async ({
    browser,
    baseURL,
  }) => {
    // erin's tab stays on what it loaded before dave answered: every GET of the change
    // request is served the first answer until her PATCH is sent. That is the real
    // situation (a tab left open) without a race against the page's own refetching.
    const erinContext = await openLocalContext(test, browser, "erin", { baseURL });
    try {
      const erinPage = await erinContext.newPage();
      let frozenBody: string | undefined;
      let frozen = true;
      // The API's address only: the page's own address ends in the same id.
      const detailUrl = `${(await stackEndpoints()).customerApi}/change-requests/${approval.id}`;
      await erinPage.route(detailUrl, async (route) => {
        const method = route.request().method();
        if (method === "PATCH") frozen = false;
        if (method !== "GET") return route.continue();
        const response = await route.fetch();
        const body = await response.text();
        frozenBody ??= body;
        return route.fulfill({ response, body: frozen ? frozenBody : body });
      });

      const erin = new ChangeRequestDetailsPage(erinPage);
      await erin.open(projectId, approval.id, approval.number);
      await expect(erin.button(UI.buttons.approve)).toBeVisible();

      // dave answers first, from his own session (the page is not needed for that).
      const daves = await customerApi("dave").patch(approval.id, { isCustomerApproved: true });
      expect(daves.status, JSON.stringify(daves.body)).toBe(200);
      await expect(erin.button(UI.buttons.approve), "erin's stale tab still offers Approve").toBeVisible();

      await erin.button(UI.buttons.approve).click();
      await expect(erin.banner(UI.banners.alreadyAnswered)).toBeVisible();
      // ...and the page now shows the truth: Scheduled, nothing left to answer.
      await expect(erin.currentStage()).toHaveText(UI.stages.scheduled);
      await expect(erin.answerButtons()).toHaveCount(0);
      // The refusal took her buttons away: focus is on the page's heading, not lost to <body>.
      await expect(erin.heading()).toBeFocused();

      // erin's own row was not turned into an approval by her late click.
      const rows = await approverRows(approval.id);
      expect(rows.find((r) => r.email === LOCAL_PERSONAS.erin.email)?.state).toBe("CANCELLED");
    } finally {
      await erinContext.close();
    }
  });

  test(`on a 390px screen the banner that answers ${LOCAL_PERSONAS.dave.email}'s click is not clipped on the left`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await dave.button(UI.buttons.approve).click();

    const banner = dave.banner(UI.banners.approved);
    await expect(banner).toBeVisible();
    const box = await banner.boundingBox();
    expect(box, "the banner has no box").not.toBeNull();
    expect(box!.x, `the banner starts at x=${box!.x}: its left side is off screen`).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width, "the banner runs past the right edge").toBeLessThanOrEqual(390);
  });

  test(`${LOCAL_PERSONAS.dave.email} rejects ${approval.number}: the confirmation comes first ("Go back" changes nothing), then the change is Canceled`, async ({
    page,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, approval.id, approval.number);
    await dave.button(UI.buttons.reject).click();

    // The confirmation says what rejecting does, and points to Propose New Time.
    const dialog = dave.rejectDialog(UI.rejectConfirm.approvalTitle);
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(UI.rejectConfirm.approvalMessage);
    await expect(dialog).toContainText(UI.rejectConfirm.approvalHint);

    // "Go back" is a no-op: no request was made, the change still waits.
    await dialog.getByRole("button", { name: UI.rejectConfirm.goBack, exact: true }).click();
    await expect(dialog).toBeHidden();
    await expect(dave.button(UI.buttons.reject)).toBeVisible();
    expect((await customerApi("dave").get(approval.id)).body.customerCanAnswer).toBe(true);
    expect((await changeRequestRow(approval.id)).state).toBe("CUSTOMER_APPROVAL");

    // Confirming rejects.
    await dave.button(UI.buttons.reject).click();
    await dave.rejectConfirmButton(UI.rejectConfirm.approvalConfirm).click();
    await expect(dave.banner(UI.banners.rejected)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.canceled);
    await expect(dave.answerButtons()).toHaveCount(0);

    expect((await customerApi("dave").get(approval.id)).body.state?.label).toBe("Canceled");
    expect((await changeRequestRow(approval.id)).state).toBe("CANCELED");
    const rows = await approverRows(approval.id);
    expect(rows.map((r) => `${r.email}|${r.state}`)).toEqual([
      "dave.mendis@example.com|REJECTED",
      "erin.jayawardena@example.com|CANCELLED",
    ]);
  });

  test(`${LOCAL_PERSONAS.dave.email} marks ${review.number} Successful in Customer Review: the change is Closed`, async ({
    page,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, review.id, review.number);
    await expect(dave.currentStage()).toHaveText(UI.stages.customerReview);

    // Customer Review has its own two buttons, and no Propose New Time.
    await expect(dave.button(UI.buttons.successful)).toBeVisible();
    await expect(dave.button(UI.buttons.unsuccessful)).toBeVisible();
    await expect(dave.approvalButtons()).toHaveCount(0);
    // They answer a question that is on the page, and sit in a group named by it.
    await expect(page.getByText(UI.notes.reviewPrompt, { exact: true })).toBeVisible();
    await expect(dave.answerGroup(UI.notes.reviewPrompt).getByRole("button")).toHaveCount(2);

    // Successful is one click, like Approve.
    await dave.button(UI.buttons.successful).click();
    await expect(dave.banner(UI.banners.markedSuccessful)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.closed);
    await expect(dave.answerButtons()).toHaveCount(0);

    expect((await customerApi("dave").get(review.id)).body.state?.label).toBe("Closed");
    expect((await changeRequestRow(review.id)).state).toBe("CLOSED");
  });

  test(`${LOCAL_PERSONAS.dave.email} marks ${review.number} Unsuccessful: the confirmation comes first, then the change is in Rollback`, async ({
    page,
  }) => {
    const dave = new ChangeRequestDetailsPage(page);
    await dave.open(projectId, review.id, review.number);
    await dave.button(UI.buttons.unsuccessful).click();

    const dialog = dave.rejectDialog(UI.rejectConfirm.reviewTitle);
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(UI.rejectConfirm.reviewMessage);
    // The "use Propose New Time instead" hint belongs to Customer Approval only.
    await expect(dialog).not.toContainText(UI.rejectConfirm.approvalHint);

    await dialog.getByRole("button", { name: UI.rejectConfirm.goBack, exact: true }).click();
    await expect(dialog).toBeHidden();
    expect((await changeRequestRow(review.id)).state).toBe("CUSTOMER_REVIEW");

    await dave.button(UI.buttons.unsuccessful).click();
    await dave.rejectConfirmButton(UI.rejectConfirm.reviewConfirm).click();
    await expect(dave.banner(UI.banners.markedUnsuccessful)).toBeVisible();
    await expect(dave.currentStage()).toHaveText(UI.stages.rollback);
    await expect(dave.answerButtons()).toHaveCount(0);

    expect((await customerApi("dave").get(review.id)).body.state?.label).toBe("Rollback");
    expect((await changeRequestRow(review.id)).state).toBe("ROLLBACK");
  });
});
