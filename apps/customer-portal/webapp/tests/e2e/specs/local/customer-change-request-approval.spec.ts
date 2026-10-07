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
// A customer ANSWERS a change request in the customer portal, against the LOCAL
// compose stack: dave.mendis@example.com, a registered contact of "Example Corp
// Production", approves the change waiting for him in Customer Approval, and
// confirms the one waiting in Customer Review.
//
// The whole chain is real — the webapp, customer-portal backend-v2 (its RBAC and its
// customer-level PATCH whitelist), entity-service (the customer-stage approval rows
// and the state machine), Postgres (row-level security, enforced because
// entity-service connects as the non-superuser csm_app) and the mock OIDC provider.
// Nothing is stubbed.
//
// ⚠ WRITES. Each test moves a seeded fixture on, and the seed is what puts it back:
//   docker-compose up -d migrate        # in the compose directory
// resets CHG-FIXED-007 to Customer Approval and CHG-FIXED-008 to Customer Review
// (and re-applies the project-type flags that show Operations). Until then a test
// finds its change request already answered and SKIPS, saying so — it never fails
// on a stack that was merely used since the last seed.
//
//   - CHG-FIXED-007, Customer Approval, asked of dave and erin
//       dave approves -> Scheduled; the success banner stays; Approve / Reject /
//       Propose New Time go; the list says Scheduled; erin, asked too, now finds
//       nothing left to answer.
//   - CHG-FIXED-008, Customer Review, asked of dave and erin
//       dave confirms "Successful" -> Closed.
//
// Why this and customer-change-request-answer.spec.ts both approve CHG-FIXED-007 and
// confirm CHG-FIXED-008: this one needs NOTHING but the stack and the sessions (no database
// access: it reaches the change requests through the list and skips a fixture an earlier run
// answered), so it can run against a stack whose Postgres container is not at hand, where
// the answer spec (which re-seeds before every test and reads rows back) skips. Where both
// can run, the answer spec is the one that proves the details; this is the minimum.
//
// (A rejection — Customer Approval -> Canceled, Customer Review -> Rollback — moves
// the same fixtures on, so one seed can only hold one of the two answers per
// fixture; the approve / confirm path is the one this spec keeps.)
//
// Skips cleanly — never fails — without a session bundle, with an expired one, when
// the bundle was minted for another origin, or when nothing answers at the base URL
// (see auth/localSessions.ts). To run it (from apps/customer-portal/webapp):
//   for p in dave erin; do E2E_LOCAL_PERSONA=$p node_modules/.bin/playwright test \
//     --config=playwright.local-auth.config.ts; done
//   node_modules/.bin/playwright test tests/e2e/specs/local/customer-change-request-approval.spec.ts \
//     --project=chromium
// Details: tests/e2e/README.md.
//

import {
  test,
  expect,
  hasSession,
  openContextAs,
  type Page,
} from "../../fixtures/test";
import {
  EXAMPLE_CORP_PROJECT_ID,
  LOCAL_PERSONAS,
  localSessionName,
  sessionMinutesLeft,
  withLocalSession,
} from "../../auth/localSessions";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import { ChangeRequestDetailsPage } from "../../pages/ChangeRequestDetailsPage";
import { CHANGE_REQUEST_DETAILS as UI } from "../../utils/selectors";

withLocalSession(test, "dave");

const SEED_HINT =
  "Re-run the seed (docker-compose up -d migrate in the compose directory) to put the fixture back.";

/** Matches a list row that is `number` in the given state. The row prints the state
 * right after the number (with line breaks in its innerText, none in its textContent,
 * hence `\s*`); matching the state's words anywhere in the row would also match the
 * fixtures' TITLES ("E2E fixture: change in Customer Approval ..."). */
function rowInState(number: string, state: string): RegExp {
  return new RegExp(`${number}\\s*${state}\\b`);
}

/**
 * Opens the project's change request list and the detail page of one fixture,
 * SKIPPING the test when the fixture is not waiting in the state it needs.
 *
 * The change request is reached the way a customer reaches it — through the list —
 * so nothing here depends on an id: the fixtures' numbers are fixed, their ids are
 * not part of the contract.
 */
async function openFixture(
  page: Page,
  number: string,
  waitingState: string,
): Promise<ChangeRequestDetailsPage> {
  const list = new ChangeRequestsPage(page);
  await list.open(EXAMPLE_CORP_PROJECT_ID);
  await expect(list.allRows().first()).toBeVisible({ timeout: 60_000 });

  const row = list.rowByNumber(number);
  await expect(
    row,
    `${number} is not listed for ${LOCAL_PERSONAS.dave.email}: is the stack seeded? ${SEED_HINT}`,
  ).toHaveCount(1);
  test.skip(
    !rowInState(number, waitingState).test(await row.innerText()),
    `${number} is no longer in ${waitingState} (an earlier run answered it). ${SEED_HINT}`,
  );

  await row.click();
  return new ChangeRequestDetailsPage(page);
}

test.describe("Local stack — a customer answers a change request", () => {
  // A cold shell load behind the project's features, the list, the detail, the answer.
  test.describe.configure({ timeout: 180_000 });

  test("dave approves CHG-FIXED-007 in Customer Approval: Scheduled, banner kept, buttons gone, erin has nothing left to answer", async ({
    page,
    browser,
  }) => {
    const detail = await openFixture(page, "CHG-FIXED-007", "Customer Approval");

    // Waiting for him: all three actions are offered while nobody has answered.
    // (A change request's hasCustomerApproved is the recorded OUTCOME, false until
    // this click, so the buttons cannot depend on it.)
    await expect(detail.button(UI.buttons.approve)).toBeVisible({ timeout: 60_000 });
    await expect(detail.button(UI.buttons.reject)).toBeVisible();
    await expect(detail.button(UI.buttons.proposeNewTime)).toBeVisible();
    await expect(detail.currentStage()).toContainText("Customer Approval");
    const detailUrl = page.url();

    await detail.button(UI.buttons.approve).click();

    // The answer is applied, and the page says so without a reload throwing the
    // banner away.
    await expect(
      detail.banner(UI.banners.approved),
    ).toBeVisible({ timeout: 30_000 });
    await expect(detail.currentStage()).toContainText("Scheduled", {
      timeout: 30_000,
    });
    await expect(detail.button(UI.buttons.approve)).toHaveCount(0);
    await expect(detail.button(UI.buttons.reject)).toHaveCount(0);
    await expect(detail.button(UI.buttons.proposeNewTime)).toHaveCount(0);

    // The list a customer goes back to agrees.
    await detail.backButton().click();
    const list = new ChangeRequestsPage(page);
    await expect(list.rowByNumber("CHG-FIXED-007")).toContainText(
      rowInState("CHG-FIXED-007", "Scheduled"),
      { timeout: 60_000 },
    );

    // erin was asked too. dave's answer settled it: she finds nothing to answer.
    const erin = localSessionName("erin");
    if (hasSession(erin) && (sessionMinutesLeft(erin) ?? 0) >= 2) {
      const context = await openContextAs(browser, erin);
      try {
        const erinPage = await context.newPage();
        await erinPage.goto(detailUrl);
        const erinDetail = new ChangeRequestDetailsPage(erinPage);
        await expect(erinDetail.currentStage()).toContainText("Scheduled", {
          timeout: 60_000,
        });
        await expect(erinDetail.button(UI.buttons.approve)).toHaveCount(0);
        await expect(erinDetail.button(UI.buttons.reject)).toHaveCount(0);
      } finally {
        await context.close();
      }
    }
  });

  test("dave confirms CHG-FIXED-008 in Customer Review as Successful: Closed", async ({
    page,
  }) => {
    const detail = await openFixture(page, "CHG-FIXED-008", "Customer Review");

    // Customer Review words the choice as Successful / Unsuccessful; there is no
    // new implementation time to propose once the work is done.
    await expect(detail.button(UI.buttons.successful)).toBeVisible({
      timeout: 60_000,
    });
    await expect(detail.button(UI.buttons.unsuccessful)).toBeVisible();
    await expect(detail.button(UI.buttons.approve)).toHaveCount(0);
    await expect(detail.button(UI.buttons.proposeNewTime)).toHaveCount(0);
    await expect(detail.currentStage()).toContainText("Customer Review");

    await detail.button(UI.buttons.successful).click();

    await expect(
      detail.banner(UI.banners.markedSuccessful),
    ).toBeVisible({ timeout: 30_000 });
    await expect(detail.currentStage()).toContainText("Closed", {
      timeout: 30_000,
    });
    await expect(detail.button(UI.buttons.successful)).toHaveCount(0);
    await expect(detail.button(UI.buttons.unsuccessful)).toHaveCount(0);

    await detail.backButton().click();
    const list = new ChangeRequestsPage(page);
    await expect(list.rowByNumber("CHG-FIXED-008")).toContainText(
      rowInState("CHG-FIXED-008", "Closed"),
      { timeout: 60_000 },
    );
  });
});
