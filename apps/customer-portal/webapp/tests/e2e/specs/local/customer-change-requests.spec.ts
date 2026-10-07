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
// Smoke test of the LOCAL stack as a customer: dave.mendis@example.com, a
// registered contact of "Example Corp Production", opens Operations > Change
// requests and sees the change request waiting for his approval — and nothing that
// belongs to another customer.
//
// ✅ READ-ONLY. Nothing here approves, rejects or otherwise changes a request: the
// approval click is a separate spec, because it moves a seeded fixture on and the
// seed has to be re-run (docker-compose up -d migrate) to put it back.
//
// What it proves, in order:
//   - the project's type grants Operations (the seed sets the project type's
//     change request / service request read flags, which is what
//     GET /projects/{id}/features reports and what shows the menu item);
//   - the list is reachable the way a customer reaches it — side nav, Operations
//     hub, "View all change requests";
//   - CHG-FIXED-007 (seeded in Customer Approval, waiting for dave and erin) is
//     listed and says where it is: Customer Approval on a fresh seed, and Scheduled
//     or Canceled once a state-changing spec has answered it (the specs that sort
//     before this file leave it so; that the buttons are offered, and what an answer
//     does, is what those specs assert, not this one);
//   - every change request the list API returns belongs to Example Corp's project.
//     Other customers' change requests exist in the same database (the
//     seed-generator creates `CR-####` ones on its own projects, and the CSM portal
//     numbers its own `CS-PORTAL-######`), so a row of another project is a scoping
//     leak. Judged on the response's project id, not on the number's shape: change
//     requests that people legitimately create on Example Corp's project later
//     must not fail this spec.
//
// Skips cleanly — never fails — without a session bundle, with an expired one, when
// the bundle was minted for another origin, or when nothing answers at the base URL
// (see auth/localSessions.ts). To run it:
//   E2E_BASE_URL=http://localhost:3000 \
//     node_modules/.bin/playwright test --config=playwright.local-auth.config.ts   # mint dave
//   node_modules/.bin/playwright test tests/e2e/specs/local --project=chromium
// (E2E_NO_WEBSERVER=1 is already the default in .env.e2e). Details: tests/e2e/README.md.
//

import { test, expect } from "../../fixtures/test";
import {
  EXAMPLE_CORP_PROJECT_ID,
  LOCAL_PERSONAS,
  withLocalSession,
} from "../../auth/localSessions";
import { SideNavPage } from "../../pages/SideNavPage";
import { ChangeRequestsPage } from "../../pages/ChangeRequestsPage";
import { CHANGE_REQUESTS_LIST, SIDE_NAV } from "../../utils/selectors";
import { projectPathPattern } from "../../utils/ids";

withLocalSession(test, "dave");

/** A change request as the list API returns it: only what this spec reads. */
interface ListedChangeRequest {
  number: string;
  project?: { id?: string } | null;
}

test.describe("Local stack — customer change requests", () => {
  // A cold shell load behind the project's features, then the hub, then the list.
  test.describe.configure({ timeout: 180_000 });

  test(`${LOCAL_PERSONAS.dave.email} lists CHG-FIXED-007 with its state (Customer Approval on a fresh seed), and only his own project's change requests`, async ({
    page,
  }) => {
    // Every change request the list API sends the page, whatever page of it.
    const returned: ListedChangeRequest[] = [];
    page.on("response", async (response) => {
      if (
        response.request().method() !== "POST" ||
        !response.url().includes(
          `/projects/${EXAMPLE_CORP_PROJECT_ID}/change-requests/search`,
        ) ||
        !response.ok()
      ) {
        return;
      }
      const body = (await response.json().catch(() => null)) as {
        changeRequests?: ListedChangeRequest[];
      } | null;
      returned.push(...(body?.changeRequests ?? []));
    });

    const nav = new SideNavPage(page);
    await nav.open(EXAMPLE_CORP_PROJECT_ID);

    // The menu itself: absent unless the project type grants service-request or
    // change-request read access, which the stock fixture row does not.
    await expect(
      nav.item(SIDE_NAV.items.operations),
      "Operations is missing from the side nav: the project's type grants neither " +
        "change request nor service request read access (re-run the seed: " +
        "docker-compose up -d migrate)",
    ).toBeVisible({ timeout: 30_000 });
    await nav.clickItem(
      SIDE_NAV.items.operations,
      projectPathPattern(EXAMPLE_CORP_PROJECT_ID, "operations"),
    );

    await page
      .getByRole("button", { name: CHANGE_REQUESTS_LIST.hubViewAllButton })
      .click();
    await expect(page).toHaveURL(
      projectPathPattern(
        EXAMPLE_CORP_PROJECT_ID,
        CHANGE_REQUESTS_LIST.pathSegment,
      ),
    );

    const changeRequests = new ChangeRequestsPage(page);
    await expect(
      changeRequests.heading(CHANGE_REQUESTS_LIST.titles.all),
    ).toBeVisible({ timeout: 60_000 });
    await expect(changeRequests.allRows().first()).toBeVisible({
      timeout: 60_000,
    });

    // The change request waiting for dave, and what it is waiting for.
    const waiting = changeRequests.rowByNumber("CHG-FIXED-007");
    await expect(
      waiting,
      "CHG-FIXED-007 is not listed — is the stack seeded, and is the fixture still in " +
        "Customer Approval (re-run the seed to reset it)?",
    ).toHaveCount(1);
    // Customer Approval until somebody answers it. customer-change-request-approval.spec.ts
    // answers it (Scheduled; Canceled if rejected) and runs before this file, so a
    // seed that has already been used is accepted too: what this proves is that the
    // fixture is listed and says where it is, not that nobody has touched it.
    // (The state is printed right after the number; the fixture's TITLE also says
    // "Customer Approval", so the state is read from there, not from anywhere in the row.)
    await expect(waiting).toContainText(
      /CHG-FIXED-007\s*(Customer Approval|Scheduled|Canceled)\b/,
    );

    // Nothing of another customer's: whatever else the database holds, every change
    // request the API returned for this project is this project's.
    expect(
      returned.length,
      "the change request search returned nothing for the project",
    ).toBeGreaterThan(0);
    for (const changeRequest of returned) {
      expect(
        changeRequest.project?.id,
        `${changeRequest.number} was returned for ${LOCAL_PERSONAS.dave.email}'s project but ` +
          "belongs to another one — another customer's change request leaked",
      ).toBe(EXAMPLE_CORP_PROJECT_ID);
    }
  });
});
