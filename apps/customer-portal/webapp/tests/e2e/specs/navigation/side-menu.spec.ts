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
// Side-menu visibility per project.
//
// ✅ READ-ONLY. Nothing is created or modified, so this is safe to run
// repeatedly.
//
// Which items render is driven by the project's **feature flags**, not its type
// label — SideBar.tsx removes Operations without service-request or
// change-request access, Engagements without engagements access, Updates without
// updates access, Security Center without SRA or component analysis, and Usage &
// Metrics unless both the project flag and the portal-wide flag are on. That is
// why expectations live per project in SIDE_NAV_VISIBILITY, verified live, rather
// than being derived from the type.
//
// Both presence and absence are asserted. A hidden item is as much a requirement
// as a visible one: Operations appearing on a project without operations access
// would be a real regression.
//
// The checks are soft (`expect.soft`) so a single run reports every mismatch
// rather than stopping at the first — with nine items, knowing the full picture
// matters more than failing fast. One page load per project keeps it quick.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SideNavPage } from "../../pages/SideNavPage";
import { SIDE_NAV } from "../../utils/selectors";
import {
  PROJECTS,
  ProjectType,
  SIDE_NAV_UNGATED_ITEMS,
  SIDE_NAV_VISIBILITY,
} from "../../config/testData";

withSession(test);

test.describe("Side Menu", () => {
  // A cold shell load, whose sidebar waits on the project's features.
  test.describe.configure({ timeout: 120_000 });

  for (const projectType of Object.values(ProjectType)) {
    const expected = SIDE_NAV_VISIBILITY[projectType];
    if (!expected) continue;

    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows the expected navigation items", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. Fill it in tests/e2e/config/testData.ts.`,
        );

        const nav = new SideNavPage(page);
        await nav.open(project.id);

        for (const [item, shouldBeVisible] of Object.entries(expected)) {
          const locator = nav.item(item);
          if (shouldBeVisible) {
            await expect
              .soft(locator, `"${item}" should be visible`)
              .toBeVisible();
          } else {
            // toHaveCount(0) rather than toBeHidden(): a hidden item is not
            // rendered at all here, and toHaveCount states that directly.
            await expect
              .soft(locator, `"${item}" should not be rendered`)
              .toHaveCount(0);
          }
        }
      });

      test("lists exactly those items and nothing else", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. Fill it in tests/e2e/config/testData.ts.`,
        );

        const sideNav = new SideNavPage(page);
        await sideNav.open(project.id);

        // The per-item test above proves every expected item is there. This one
        // proves nothing *unexpected* is: an item the suite has never heard of —
        // a new feature shipped without a visibility expectation, or one whose
        // gate stopped working — is invisible to a loop over known labels.
        const expectedItems = [
          ...Object.entries(expected)
            .filter(([, isVisible]) => isVisible)
            .map(([label]) => label),
          // Settings has no permission filter, so it is not in the gated table
          // but is always in the rendered list.
          ...SIDE_NAV_UNGATED_ITEMS,
        ].sort();

        const actual = (await sideNav.itemNames()).sort();

        expect(
          actual,
          "the sidebar should render exactly the expected items",
        ).toEqual(expectedItems);

        console.log(`Side nav (${projectType}): ${actual.length} items`);
      });

      test("opens the route behind every visible item", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. Fill it in tests/e2e/config/testData.ts.`,
        );

        const sideNav = new SideNavPage(page);
        await sideNav.open(project.id);

        // One shell load, then a click per item: the sidebar persists across
        // routes, so there is no need to return to the dashboard between them.
        //
        // Visibility is asserted above; this is about where each item actually
        // goes. An item rendered but wired to the wrong route — or to none —
        // passes every visibility check there is.
        const visible = [
          ...Object.entries(expected)
            .filter(([, isVisible]) => isVisible)
            .map(([label]) => label),
          ...SIDE_NAV_UNGATED_ITEMS,
        ];

        for (const label of visible) {
          const path = SIDE_NAV.paths[label];
          expect(path, `no route recorded for the "${label}" item`).toBeTruthy();

          // Unanchored: a section that redirects to a child route — a default tab,
          // say — has still done its job, and pinning the exact landing path would
          // make this assert routing internals rather than the item.
          await sideNav.clickItem(
            label,
            new RegExp(`/projects/${project.id}/${path}`),
          );
        }

        console.log(
          `Side nav (${projectType}): ${visible.length} items opened their routes`,
        );
      });
    });
  }
});
