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
// The announcements list, reached the way a user reaches it: the side nav's
// Announcements item. Run against every project type — the item is visible on
// all three (see SIDE_NAV_VISIBILITY).
//
// ✅ READ-ONLY. Announcements are published to the project rather than raised
// from the portal, so nothing here creates or changes one.
//
// Opening an announcement by URL is already covered by view-announcement.spec.ts;
// what that spec cannot show is that the list actually leads there, which is the
// third test below.
//

import { test, expect, withSession } from "../../fixtures/test";
import { AnnouncementsPage } from "../../pages/AnnouncementsPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { PROJECTS, ProjectType } from "../../config/testData";
import { ANNOUNCEMENTS_LIST } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import {
  caseSearchWithSort,
  caseSearchWithStatusFilter,
} from "../../utils/listSearch";

withSession(test);

test.describe("Announcements", () => {
  // A shell load, a nav navigation and the list's own search request; the 30s
  // default is not enough.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("opens from the side menu", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);

        await expect(page).toHaveURL(
          new RegExp(
            `/projects/${project.id}/${ANNOUNCEMENTS_LIST.pathSegment}$`,
          ),
        );
        await expect(announcements.heading()).toBeVisible();
        await expect(announcements.description()).toBeVisible();
      });

      test("lists announcements with a results count", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);

        // Settle into rows or the empty copy first — the results bar renders
        // while the search is still in flight and reads zero until it lands.
        await announcements.waitForList();

        const rows = await announcements.rows().count();
        const counts = await announcements.resultsCounts();
        expect(counts, "the results bar should report counts").not.toBeNull();

        // The bar and the rows have to agree: the shown count is what is on this
        // page, so a mismatch means one of the two is stale.
        expect((counts as { shown: number }).shown).toBe(rows);
        expect((counts as { total: number }).total).toBeGreaterThanOrEqual(rows);

        console.log(
          `Announcements (${projectType}): ${rows} of ` +
            `${(counts as { total: number }).total} listed`,
        );
      });


      test("offers the status filter once the panel is opened", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);
        await announcements.waitForList();

        // The panel is collapsed on load and its contents unmounted, so the
        // select does not exist until it is opened — without this check the
        // assertion below would pass against an already-open panel.
        await expect(announcements.statusFilterSelect()).toHaveCount(0);
        await announcements.filtersButton().click();

        await expect(announcements.statusFilterSelect()).toBeVisible();
        await expect(
          page
            .getByText(ANNOUNCEMENTS_LIST.statusFilter.label, { exact: true })
            .first(),
        ).toBeVisible();

        // Status is the only filter announcements offer, and it has options to
        // choose from — an empty list would mean the metadata never arrived.
        const options = await announcements.statusFilterOptions();
        expect(options.length, "the status filter should offer options").toBeGreaterThan(0);

        console.log(
          `Announcements (${projectType}): status filter offers ` +
            `${options.length} options`,
        );
      });

      test("filters by status and clears it", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);
        await announcements.waitForList();

        await announcements.filtersButton().click();
        await expect(announcements.statusFilterSelect()).toBeVisible();

        const options = await announcements.statusFilterOptions();
        test.skip(
          options.length === 0,
          `${projectType} offers no status options to filter by.`,
        );

        // Armed before the choice: the status ids on the wire are what show the
        // filter was applied, rather than merely ticked in the menu.
        const filtered = caseSearchWithStatusFilter(page, true);
        await announcements.selectStatusFilter(options[0]);
        await expectSuccess(await filtered, "filtered announcements search");

        // The control turns into the clear action, counting what is active.
        await expect(announcements.clearFiltersButton(1)).toBeVisible();
        await expect(announcements.filtersButton()).toHaveCount(0);

        // Clearing drops the statuses from the request. Armed before the click,
        // because the page's own first load matches this predicate too.
        const cleared = caseSearchWithStatusFilter(page, false);
        await announcements.clearFiltersButton(1).click();
        await expectSuccess(await cleared, "announcements search after clearing");

        await expect(announcements.filtersButton()).toBeVisible();

        console.log(
          `Announcements (${projectType}): filtered by "${options[0]}" and cleared`,
        );
      });

      test("sorts by field and order", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);
        await announcements.waitForList();

        const { fields, orders } = ANNOUNCEMENTS_LIST.sort;

        // Switching the field to Status. The field travels in `sortBy`, at the
        // root of the request body rather than inside `filters`.
        const sortedByStatus = caseSearchWithSort(
          page,
          fields.status.value,
          orders.ordinalDesc.value,
        );
        await announcements.chooseSortOption(
          announcements.sortFieldSelect(),
          fields.status.label,
        );
        await expectSuccess(await sortedByStatus, "announcements sorted by status");

        // The order options are worded for the field: an ordinal sort reads
        // Descending/Ascending where a chronological one reads Newest/Oldest
        // first, so the label changes with the field above.
        const ascending = caseSearchWithSort(
          page,
          fields.status.value,
          orders.ordinalAsc.value,
        );
        await announcements.chooseSortOption(
          announcements.sortOrderSelect(),
          orders.ordinalAsc.label,
        );
        await expectSuccess(await ascending, "announcements sorted ascending");

        // And back to the chronological field, whose order labels differ again.
        const byDate = caseSearchWithSort(
          page,
          fields.updatedDate.value,
          orders.chronologicalAsc.value,
        );
        await announcements.chooseSortOption(
          announcements.sortFieldSelect(),
          fields.updatedDate.label,
        );
        await expectSuccess(await byDate, "announcements sorted by updated date");

        console.log(
          `Announcements (${projectType}): sorted by status and by updated date`,
        );
      });

      test("pages the list and changes its page size", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);
        await announcements.waitForList();

        const range = await announcements.displayedRange();
        test.skip(
          range === null,
          `${projectType} shows no pagination range — the list is empty.`,
        );

        const { from, total } = range as { from: number; total: number };
        expect(from).toBe(1);
        await expect(announcements.previousPageButton()).toBeDisabled();

        const next = announcements.nextPageButton();

        // A project whose announcements fit on one page has nothing to page
        // through, and a disabled control is the right behaviour there.
        if (await next.isEnabled()) {
          await next.click();
          await expect
            .poll(async () => (await announcements.displayedRange())?.from, {
              timeout: 30_000,
            })
            .toBe(ANNOUNCEMENTS_LIST.defaultRowsPerPage + 1);
          await expect(announcements.previousPageButton()).toBeEnabled();

          await announcements.previousPageButton().click();
          await expect
            .poll(async () => (await announcements.displayedRange())?.from, {
              timeout: 30_000,
            })
            .toBe(1);
          await expect(announcements.previousPageButton()).toBeDisabled();
        } else {
          console.log(
            `Announcements (${projectType}): ${total} fit on one page`,
          );
        }

        // A larger page shows up to however many exist — the expectation comes
        // from the list's own total rather than assuming there is more than one
        // page's worth.
        //
        // Raising the size can also take the pagination away: ListPagination
        // renders nothing once `totalRecords <= rowsPerPage`. So the count comes
        // from the range while there is one, and from the rows themselves once
        // everything fits on a single page — waiting for a range that cannot
        // appear would just time out.
        const larger = 25;
        await announcements.selectRowsPerPage(larger);
        await expect
          .poll(
            async () => {
              const current = await announcements.displayedRange();
              if (current) return current.to - current.from + 1;
              return announcements.rows().count();
            },
            {
              message:
                "the resized page should show every announcement it can fit",
              timeout: 30_000,
            },
          )
          .toBe(Math.min(larger, total));

        console.log(
          `Announcements (${projectType}): paged and resized to ${larger} per page`,
        );
      });

      test("opens an announcement from the list", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const announcements = new AnnouncementsPage(page);
        await announcements.openViaSideNav(project.id);
        await announcements.waitForList();

        const rowCount = await announcements.rows().count();
        test.skip(
          rowCount === 0,
          `${projectType} has no announcements to open. This test needs at ` +
            `least one published to the project.`,
        );

        // Read the number off the row before clicking, so the detail page can be
        // checked against the row that was actually opened rather than against a
        // sysid from config — which is what the view spec already does.
        const number = await announcements.rowNumber(0);
        expect(number, "the row should carry a case number").toBeTruthy();

        await announcements.rows().first().click();

        // An announcement sysid is a 32-character hex string; anchoring rules out
        // landing back on the list.
        await expect(page).toHaveURL(
          new RegExp(
            `/projects/${project.id}/${ANNOUNCEMENTS_LIST.pathSegment}/[0-9a-f]{32}$`,
          ),
        );

        // The announcement that was clicked, not merely some announcement. The
        // detail page reuses the case header, so the number renders there the
        // same way.
        const detail = new CaseDetailPage(page);
        await expect(detail.caseNumber()).toHaveText(number as string, {
          timeout: 30_000,
        });

        console.log(`Announcements (${projectType}): opened ${number}`);
      });
    });
  }
});
