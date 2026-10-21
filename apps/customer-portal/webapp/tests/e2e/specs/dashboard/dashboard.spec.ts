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
// The project dashboard, reached the way a user reaches it: the side nav's
// Dashboard item. Run against every project type.
//
// Read-only — nothing here creates or changes a record.
//
// The four stat cards come from DASHBOARD_STATS and are not feature-gated, so
// every project type gets the same four. Three of them open a list; Avg.
// Response Time is named in the grid's `nonClickableKeys` and is a plain card,
// which the test asserts rather than leaves to chance — a card that quietly
// became clickable would navigate somewhere on a stray click.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { DashboardPage } from "../../pages/DashboardPage";
import {
  DASHBOARD_OPERATIONS_VISIBILITY,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { DASHBOARD } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import {
  caseSearchWithPagination,
  caseSearchWithSeverity,
  caseSearchWithoutSeverity,
  myCasesSearchResponse,
} from "../../utils/listSearch";
import {
  RECORD_ID_PATTERN,
  projectPathPattern,
} from "../../utils/ids";

withSession(test);

test.describe("Dashboard", () => {
  // A shell load, a nav navigation and the dashboard's own stats queries — well
  // past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];

    // Named in the test title rather than only asserted, so a run's output says
    // which projects are expected to carry the Operations chart without anyone
    // having to open the config.
    const operationsExpectation = DASHBOARD_OPERATIONS_VISIBILITY[projectType]
      ? "present"
      : "withheld";

    test.describe(projectType, () => {
      test("shows the four overview cards", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        // Soft, so one missing card does not hide the state of the other three.
        for (const card of DASHBOARD.statCards) {
          await expect.soft(dashboard.statCard(card.label)).toBeVisible();
        }

        // Exactly four — a fifth card appearing is as much a change as one going
        // missing, and the loop above would not notice it.
        await expect(dashboard.statCardLabels()).toHaveCount(
          DASHBOARD.statCards.length,
        );

        // Which cards open a list and which do not is part of the contract.
        for (const card of DASHBOARD.statCards) {
          await expect
            .soft(
              dashboard.statCardButton(card.label),
              card.clickable
                ? `${card.label} should be clickable`
                : `${card.label} should not be clickable`,
            )
            .toHaveCount(card.clickable ? 1 : 0);
        }

        console.log(
          `Dashboard (${projectType}): ${DASHBOARD.statCards.length} cards`,
        );
      });

      test(`shows the outstanding cases and engagements sections with operations ${operationsExpectation}`, async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        // Asserted as "at least one" rather than by visibility: the Outstanding
        // Support Cases title is used twice — by the severity donut and by the
        // cases table below it — and a single-element locator would be a
        // strict-mode violation.
        await expect
          .soft(dashboard.section(DASHBOARD.sections.outstandingCases))
          .not.toHaveCount(0);

        // Engagements is feature-gated, so this asserts the flag is on for the
        // fixture projects as much as it asserts the section renders. A project
        // without engagements would legitimately have no such section — see
        // SIDE_NAV_VISIBILITY, which records the same flag per project.
        await expect
          .soft(dashboard.section(DASHBOARD.sections.outstandingEngagements))
          .not.toHaveCount(0);

        // Outstanding Operations is the one section that differs by project: it
        // needs service-request or change-request access, which not every
        // project has. Asserting its absence is safe here because the two
        // assertions above have already established the dashboard rendered.
        const operations = dashboard.section(
          DASHBOARD.sections.outstandingOperations,
        );
        if (DASHBOARD_OPERATIONS_VISIBILITY[projectType]) {
          await expect
            .soft(
              operations,
              `${projectType} has SR/CR access, so Outstanding Operations ` +
                `should render`,
            )
            .not.toHaveCount(0);
        } else {
          await expect
            .soft(
              operations,
              `${projectType} has no SR/CR access, so Outstanding Operations ` +
                `should be withheld`,
            )
            .toHaveCount(0);
        }

        console.log(
          `Dashboard (${projectType}): outstanding cases and engagements ` +
            `present, operations ` +
            `${DASHBOARD_OPERATIONS_VISIBILITY[projectType] ? "present" : "withheld"}`,
        );
      });


      test("opens a case from the Outstanding Support Cases table", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        // The subtitle, not the title: the title is shared with the severity
        // donut above, so it would not tell the two cards apart.
        await expect(
          dashboard.section(DASHBOARD.sections.outstandingCases).first(),
        ).toBeVisible();
        await expect(
          page.getByText(DASHBOARD.casesTable.subtitle, { exact: true }),
        ).toBeVisible();

        const firstRow = await dashboard.waitForCasesTable();

        // Read the case number off the row before clicking, so the detail page
        // can be checked against the row that was actually clicked rather than
        // against a case id from config.
        const rowText = await firstRow.innerText();
        const caseNumber = DASHBOARD.casesTable.rowCaseNumberPattern.exec(
          rowText,
        )?.[1];
        expect(caseNumber, "row should carry a case number").toBeTruthy();

        await firstRow.click();

        // A case sysid is a 32-character hex string; anchoring rules out landing
        // on the list route instead of a case.
        await expect(page).toHaveURL(
          projectPathPattern(
            project.id,
            `support/cases/${RECORD_ID_PATTERN}$`,
          ),
        );

        // The case that was clicked, not merely some case: the detail page shows
        // the number bare, where the row prefixes it with "ID: ".
        const caseDetail = new CaseDetailPage(page);
        await expect(caseDetail.caseNumber()).toHaveText(caseNumber as string);

        console.log(
          `Dashboard (${projectType}): opened ${caseNumber} from the cases table`,
        );
      });


      test("loads my cases in the Outstanding Support Cases table", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        await expect(
          page.getByText(DASHBOARD.casesTable.subtitle, { exact: true }),
        ).toBeVisible();

        const myCases = dashboard.casesTableViewTab(
          DASHBOARD.casesTable.viewTabs.myCases,
        );
        const allCases = dashboard.casesTableViewTab(
          DASHBOARD.casesTable.viewTabs.allCases,
        );

        // All Cases is where the table starts, so the switch below is a real
        // change of state rather than a no-op.
        await expect(allCases).toHaveAttribute("aria-selected", "true");
        await expect(myCases).toHaveAttribute("aria-selected", "false");

        // Armed before the click: the table refetches as soon as the tab
        // changes, and a listener registered afterwards can miss the response.
        const searchResponse = myCasesSearchResponse(page);
        await myCases.click();

        // Matching this response at all is the proof that the table re-queried
        // with createdByMe — the predicate is what selected it.
        await expectSuccess(await searchResponse, "my cases search");

        await expect(myCases).toHaveAttribute("aria-selected", "true");

        // Still on the dashboard: this switch filters the table in place, unlike
        // Support Center's "View my cases", which opens a separate list page.
        await expect(page).toHaveURL(
          new RegExp(`/projects/${project.id}/${DASHBOARD.pathSegment}$`),
        );

        // Every remaining row is the signed-in user's. Tolerant of an empty
        // result: an account that raised no cases on this project sees "No
        // outstanding cases." here, which is the filter working, not failing —
        // so requiring rows would fail on correct behaviour.
        const rowCount = await dashboard.waitForCasesTableSettled();

        if (rowCount === 0) {
          await expect(
            page.getByText(DASHBOARD.casesTable.emptyMessage),
            "an empty My Cases view should say so rather than hang",
          ).toBeVisible();

          console.log(
            `Dashboard (${projectType}): my cases is empty — this account ` +
              "raised no cases on this project, so the per-row creator check " +
              "has nothing to assert. The filter itself is proven by the " +
              "createdByMe request above.",
          );
          return;
        }

        // Counted, not named: the creators are real email addresses, and a test
        // report is no place to put them. The distinct count is what carries the
        // assertion — more than one means the filter let another user's cases
        // through.
        const creators = await dashboard.casesTableCreators();
        const distinctCreators = new Set(creators).size;

        expect(creators.length, "no row named a creator").toBeGreaterThan(0);
        expect(
          distinctCreators,
          "every row should name the same creator — more than one means the " +
            "createdByMe filter did not hold",
        ).toBe(1);

        console.log(
          `Dashboard (${projectType}): my cases table shows ${creators.length} ` +
            `rows from ${distinctCreators} creator`,
        );
      });


      test("changes the cases table page size to 10", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        await expect(
          page.getByText(DASHBOARD.casesTable.subtitle, { exact: true }),
        ).toBeVisible();

        const pagination = DASHBOARD.casesTable.pagination;

        // The table starts at 5, so switching to 10 is a real change.
        const select = dashboard.rowsPerPageSelect();
        await expect(select).toContainText(String(pagination.defaultRowsPerPage));

        // Wait for the rows before counting them. The pagination control renders
        // while the first search is still in flight, so an immediate count reads
        // 0 — which would satisfy both bounds below and make this test pass
        // without ever comparing two loaded pages.
        await dashboard.waitForCasesTable();

        const rowsBefore = await dashboard.casesTableRows().count();
        expect(
          rowsBefore,
          `the default page size is ${pagination.defaultRowsPerPage}, so the ` +
            `table should show no more than that`,
        ).toBeLessThanOrEqual(pagination.defaultRowsPerPage);
        expect(rowsBefore, "the table should have loaded rows").toBeGreaterThan(
          0,
        );

        // Armed before the change: the page size travels as `pagination.limit`,
        // so this both waits for the refetch and proves the new size reached the
        // backend rather than only the control.
        const searchResponse = caseSearchWithPagination(page, { limit: 10 });
        await dashboard.selectRowsPerPage(10);
        await expectSuccess(await searchResponse, "cases search at 10 per page");

        await expect(select).toContainText("10");

        // A larger page cannot show fewer rows, and must not exceed the size
        // asked for. Bounds rather than an exact count, because how many rows
        // exist is environment data.
        //
        // Polled, not counted once: the response arriving is not the table
        // having re-rendered. React clears the rows while rebuilding at the new
        // page size, so a single count taken the moment the request resolves
        // can read 0 — which is what made this fail intermittently, and only
        // inside a full run where the timing shifted.
        await expect
          .poll(() => dashboard.casesTableRows().count(), {
            timeout: 60_000,
            message:
              "a larger page size should not show fewer rows than the smaller one",
          })
          .toBeGreaterThanOrEqual(rowsBefore);

        const rowsAfter = await dashboard.casesTableRows().count();
        expect(rowsAfter).toBeLessThanOrEqual(10);

        console.log(
          `Dashboard (${projectType}): page size 5 → 10, rows ${rowsBefore} → ${rowsAfter}`,
        );
      });


      test("pages the cases table forward and back", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        await expect(
          page.getByText(DASHBOARD.casesTable.subtitle, { exact: true }),
        ).toBeVisible();

        const pageSize = DASHBOARD.casesTable.pagination.defaultRowsPerPage;

        // Wait for the first page to load before reading the controls: the
        // pagination renders while the search is still in flight, when next is
        // disabled simply because the count is not known yet.
        await dashboard.waitForCasesTable();
        await expect(dashboard.displayedRows()).toBeVisible();
        expect(await dashboard.displayedFromRow()).toBe(1);

        // Previous is always dead on the first page, whatever the count.
        await expect(dashboard.previousPageButton()).toBeDisabled();

        const next = dashboard.nextPageButton();
        await expect(next).toBeVisible();

        // "if active" — a project whose outstanding cases fit on one page has
        // nothing to page through, and that is a pass rather than a skip: the
        // control is correctly disabled.
        if (!(await next.isEnabled())) {
          console.log(
            `Dashboard (${projectType}): outstanding cases fit on one page, ` +
              `next is disabled`,
          );
          return;
        }

        // Forward. The offset is what asks the backend for the second page, so
        // matching it proves the click did more than move the highlight.
        const forward = caseSearchWithPagination(page, { offset: pageSize });
        await next.click();
        await expectSuccess(await forward, "cases search for page 2");

        expect(await dashboard.displayedFromRow()).toBe(pageSize + 1);
        await expect(dashboard.previousPageButton()).toBeEnabled();

        // And back, which must return to offset 0 and the original range.
        const backward = caseSearchWithPagination(page, { offset: 0 });
        await dashboard.previousPageButton().click();
        await expectSuccess(await backward, "cases search for page 1");

        expect(await dashboard.displayedFromRow()).toBe(1);
        await expect(dashboard.previousPageButton()).toBeDisabled();

        console.log(
          `Dashboard (${projectType}): paged forward to row ${pageSize + 1} and back to 1`,
        );
      });


      test("renders the Outstanding Engagements breakdown", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        const heading = DASHBOARD.sections.outstandingEngagements;

        // The card is gated on `permissions.hasEngagements`, the same flag as
        // the Engagements nav item — so its absence is a permission state, not a
        // failure, and is skipped rather than asserted away.
        //
        // Keyed on the first CATEGORY, not on the heading: "Outstanding
        // Engagements" also labels a stat in the row above, so a heading check
        // passes on a project that has no card — the same ambiguity this file
        // already documents for "Outstanding Support Cases". Waiting for a
        // category both resolves that and gives the chart time to render, which
        // a bare visibility read would not.
        const firstCategory = DASHBOARD.sections.engagementCategories[0];
        const present = await dashboard
          .legendEntry(firstCategory)
          .first()
          .waitFor({ state: "visible", timeout: 60_000 })
          .then(() => true)
          .catch(() => false);
        test.skip(
          !present,
          `${projectType} does not expose the Outstanding Engagements ` +
            `breakdown (permissions.hasEngagements).`,
        );

        // All four categories, every time. They are rendered from a fixed list
        // (OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA) rather than from the
        // data, so one with no engagements still appears with a zero —
        // asserting only the non-empty ones would pass on a chart that had
        // silently dropped a category.
        //
        // Soft, so a missing category does not hide the state of the others.
        for (const category of DASHBOARD.sections.engagementCategories) {
          await expect
            .soft(
              dashboard.legendEntry(category).first(),
              `the "${category}" category should be listed`,
            )
            .toBeVisible({ timeout: 60_000 });
        }

        // Slices are drawn only for non-zero categories — Recharts gives a
        // zero-value series no slice at all — so the count is bounded by the
        // legend rather than equal to it.
        const slices = await dashboard
          .chartSlices(heading)
          .count()
          .catch(() => 0);
        expect(
          slices,
          "the donut should not draw more slices than it has categories",
        ).toBeLessThanOrEqual(
          DASHBOARD.sections.engagementCategories.length,
        );

        console.log(
          `Dashboard (${projectType}): Outstanding Engagements lists ` +
            `${DASHBOARD.sections.engagementCategories.length} categories, ` +
            `${slices} with engagements`,
        );
      });

      test("filters the cases table by severity and clears it", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        await expect(
          page.getByText(DASHBOARD.casesTable.subtitle, { exact: true }),
        ).toBeVisible();

        // Wait for the unfiltered table first, so the filtered result below is a
        // change from a known state rather than from a still-loading one.
        await dashboard.waitForCasesTable();

        const severity = DASHBOARD.casesTable.filters.severity;

        // The panel is collapsed on load and its contents are unmounted, so the
        // Select does not exist until this is clicked.
        await expect(dashboard.severityFilterSelect()).toHaveCount(0);
        await dashboard.casesTableFiltersButton().click();
        await expect(dashboard.severityFilterSelect()).toBeVisible();
        await expect(
          page.getByText(severity.label, { exact: true }).first(),
        ).toBeVisible();

        // Armed before the choice: the id on the wire is what shows the filter
        // was applied, rather than merely ticked in the menu.
        const searchResponse = caseSearchWithSeverity(page, severity.option.id);
        await dashboard.selectSeverityFilter(severity.option.label);
        await expectSuccess(
          await searchResponse,
          `cases search filtered to ${severity.option.label}`,
        );

        // The control reports the choice back.
        await expect(dashboard.severityFilterSelect()).toContainText(
          severity.option.label,
        );

        // Every row left is at that severity. Not asserted to be non-empty: a
        // project with no outstanding S4 cases legitimately shows none, and the
        // request assertion above is what proves the filter ran.
        const rows = await dashboard.casesTableRows().allInnerTexts();
        for (const [index, row] of rows.entries()) {
          expect(
            row,
            `row ${index + 1} should be ${severity.option.label}`,
          ).toContain(severity.option.label);
        }

        // The Filters button turns into the clear control once a filter is on,
        // counting what is active.
        const clearFilters = dashboard.clearFiltersButton(1);
        await expect(clearFilters).toBeVisible();
        await expect(dashboard.casesTableFiltersButton()).toHaveCount(0);

        // Clearing drops the severity from the request. Armed before the click,
        // because the page's own first load matches this predicate too.
        const clearedResponse = caseSearchWithoutSeverity(page);
        await clearFilters.click();
        await expectSuccess(await clearedResponse, "cases search after clearing");

        // The control goes back to offering the panel, and the Select no longer
        // holds the choice.
        await expect(dashboard.casesTableFiltersButton()).toBeVisible();
        await expect(dashboard.severityFilterSelect()).not.toContainText(
          severity.option.label,
        );

        console.log(
          `Dashboard (${projectType}): filtered to ${severity.option.label} ` +
            `(${rows.length} rows), then cleared`,
        );
      });


      for (const target of DASHBOARD.statCardTargets) {
        test(`opens the ${target.title} list from the ${target.label} card`, async ({
          page,
        }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const dashboard = new DashboardPage(page);
          await dashboard.openViaSideNav(project.id);

          await dashboard.statCardButton(target.label).click();

          // Nested under the dashboard route, so the path keeps that segment.
          await expect(page).toHaveURL(
            new RegExp(`/projects/${project.id}/${target.pathSegment}$`),
          );

          const main = page.getByTestId("app-main");
          await expect(
            main.getByRole("heading", { name: target.title, exact: true }),
          ).toBeVisible();
          await expect(
            main.getByText(target.description, { exact: true }),
          ).toBeVisible();

          // Wait for the list area to settle into one of its two real states — a
          // row, or this mode's empty copy. Without this the test would pass
          // against a page still showing skeletons, which is what "see the list
          // initialise" has to rule out.
          await expect(
            dashboard
              .itemRows()
              .first()
              .or(dashboard.emptyItemsMessage(target.emptyMessage)),
          ).toBeVisible();

          // How many items each list holds is environment data, so an empty one
          // is a legitimate result — what matters is that the list rendered.
          const rows = await dashboard.itemRows().count();

          console.log(
            `Dashboard (${projectType}): ${target.title} listed ${rows} items`,
          );
        });
      }

      test(`the ${DASHBOARD.nonClickableStatCard} card does not drill down`, async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        const label = DASHBOARD.nonClickableStatCard;
        const card = dashboard.statCard(label);
        await expect(card).toBeVisible();

        // Not a button, unlike the other three: it is named in the grid's
        // `nonClickableKeys`, so it reports a value and nothing more.
        await expect(dashboard.statCardButton(label)).toHaveCount(0);

        // Clicking it anyway must leave the dashboard where it is. This is the
        // behavioural half of the assertion above — a card could lose its button
        // role and still navigate through some other handler.
        await card.click();
        await expect(page).toHaveURL(
          new RegExp(`/projects/${project.id}/${DASHBOARD.pathSegment}$`),
        );

        // And the dashboard is still the dashboard, not a half-navigated shell.
        await expect(
          dashboard.statCard(DASHBOARD.statCards[0].label),
        ).toBeVisible();

        console.log(
          `Dashboard (${projectType}): ${label} is not clickable, as intended`,
        );
      });


      test("drills down from every slice of the Outstanding Support Cases donut", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        // Wait for the donut itself, not just its heading: the chart renders on
        // its own query, well after the stat cards, so counting slices as soon
        // as the heading appears reads zero.
        await expect(dashboard.severityChartSlices().first()).toBeVisible();

        // How many slices the donut draws depends on the data — a severity with
        // no outstanding cases may or may not get a sector — so the count is
        // bounded rather than pinned, and no slice is assumed to be a particular
        // severity by its position.
        const sliceCount = await dashboard.severityChartSlices().count();
        expect(sliceCount, "the donut should have slices").toBeGreaterThan(0);
        expect(
          sliceCount,
          "the donut should not draw more slices than the legend has entries",
        ).toBeLessThanOrEqual(DASHBOARD.severityLegend.length);

        // Which severity each slice belongs to is read back from where it lands,
        // not assumed from its index: if the chart ever skips a zero-count
        // severity, index n stops meaning legend entry n, and a test that
        // assumed otherwise would assert the wrong list against the wrong slice.
        const bySeverityId = new Map<
          string,
          { severityId: string; title: string; description: string }
        >(
          DASHBOARD.severityLegend.map((entry) => [entry.severityId, entry]),
        );
        const visited: string[] = [];

        for (let index = 0; index < sliceCount; index += 1) {
          // Back to the dashboard between slices: each drill-down navigates
          // away, and returning through the nav re-renders the chart.
          if (index > 0) {
            await dashboard.openViaSideNav(project.id);
            await expect(dashboard.severityChartSlices().first()).toBeVisible();
          }

          await dashboard.clickSeverityChartSlice(index);

          // Every slice must open a severity-filtered cases list.
          await expect(page).toHaveURL(
            new RegExp(
              `/projects/${project.id}/support/cases\\?severityId=\\d+$`,
            ),
          );

          const severityId = new URL(page.url()).searchParams.get("severityId");
          const entry = severityId
            ? bySeverityId.get(severityId)
            : undefined;
          expect(
            entry,
            `slice ${index + 1} opened severityId ${severityId}, which is not ` +
              `one of the severities in the legend`,
          ).toBeDefined();
          visited.push(severityId as string);

          const main = page.getByTestId("app-main");
          const expected = entry as { title: string; description: string };
          await expect(
            main.getByRole("heading", { name: expected.title, exact: true }),
          ).toBeVisible();
          await expect(
            main.getByText(expected.description, { exact: true }),
          ).toBeVisible();
        }

        // Each slice is its own severity — two slices landing on the same list
        // would mean the chart is mis-wired even though every click "worked".
        expect(
          new Set(visited).size,
          `slices opened ${new Set(visited).size} distinct severities out of ${sliceCount}`,
        ).toBe(sliceCount);

        console.log(
          `Dashboard (${projectType}): all ${sliceCount} donut slices drilled ` +
            `down to their severity lists`,
        );
      });

      for (const entry of DASHBOARD.severityLegend) {
        test(`the ${entry.label} legend entry opens its list`, async ({
          page,
        }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const dashboard = new DashboardPage(page);
          await dashboard.openViaSideNav(project.id);

          await dashboard.legendEntry(entry.label).click();

          // The severity id is what filters the list; the heading below merely
          // reports it, and all four entries land on the same route.
          await expect(page).toHaveURL(
            new RegExp(
              `/projects/${project.id}/support/cases\\?severityId=${entry.severityId}$`,
            ),
          );

          // Both come from the id rather than from the data, so they hold even
          // for a severity this project has no outstanding cases at — which is
          // why this asserts the heading rather than the rows.
          const main = page.getByTestId("app-main");
          await expect(
            main.getByRole("heading", { name: entry.title, exact: true }),
          ).toBeVisible();
          await expect(
            main.getByText(entry.description, { exact: true }),
          ).toBeVisible();

          console.log(
            `Dashboard (${projectType}): ${entry.label} → ${entry.title}`,
          );
        });
      }

      // Only where the Operations chart renders at all — a project without SR or
      // CR access has no such legend to click.
      if (DASHBOARD_OPERATIONS_VISIBILITY[projectType]) {

        test("drills down from every slice of the Outstanding Operations donut", async ({
          page,
        }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const dashboard = new DashboardPage(page);
          await dashboard.openViaSideNav(project.id);

          const heading = DASHBOARD.sections.outstandingOperations;
          await expect(
            dashboard.section(heading).first(),
            `${projectType} should carry the Outstanding Operations chart`,
          ).toBeVisible();

          // Wait for the donut itself: the charts render on their own queries,
          // well after the stat cards, so counting slices any earlier reads zero.
          await expect(dashboard.chartSlices(heading).first()).toBeVisible();

          // Both series carry work on this project, so there is a slice for each
          // legend row and index n means legend entry n. Pinned rather than
          // bounded — unlike the severity donut, which has to allow for a
          // severity with no cases — because if either series ever emptied, this
          // count failing is the signal to revisit the mapping below rather than
          // something to absorb silently.
          const sliceCount = await dashboard.chartSlices(heading).count();
          expect(
            sliceCount,
            "the operations donut should have one slice per legend row",
          ).toBe(DASHBOARD.operationsLegend.length);

          for (const [index, entry] of DASHBOARD.operationsLegend.entries()) {
            // Back to the dashboard between slices: each drill-down navigates
            // away, and returning through the nav re-renders the chart.
            if (index > 0) {
              await dashboard.openViaSideNav(project.id);
              await expect(dashboard.chartSlices(heading).first()).toBeVisible();
            }

            await dashboard.clickChartSlice(heading, index);

            await expect(page).toHaveURL(
              new RegExp(`/projects/${project.id}/${entry.pathSegment}$`),
            );

            // As with the legend, the path is the one the Operations hub uses for
            // the whole list — the "Outstanding" title is what shows the chart
            // handed over its `outstandingOnly` state, which travels outside the
            // URL.
            const main = page.getByTestId("app-main");
            await expect(
              main.getByRole("heading", { name: entry.title, exact: true }),
            ).toBeVisible();
            await expect(
              main.getByText(entry.description, { exact: true }),
            ).toBeVisible();
          }

          console.log(
            `Dashboard (${projectType}): all ${sliceCount} operations donut ` +
              `slices drilled down to their lists`,
          );
        });

        for (const entry of DASHBOARD.operationsLegend) {
          test(`the ${entry.label} legend entry opens its list`, async ({
            page,
          }) => {
            test.skip(!project.id, `${projectType} needs a project id.`);

            const dashboard = new DashboardPage(page);
            await dashboard.openViaSideNav(project.id);

            await dashboard.legendEntry(entry.label).click();

            await expect(page).toHaveURL(
              new RegExp(`/projects/${project.id}/${entry.pathSegment}$`),
            );

            // The path is the same one the Operations hub uses for the whole
            // list; the "Outstanding" title is what shows the chart handed over
            // its `outstandingOnly` state, which travels outside the URL.
            const main = page.getByTestId("app-main");
            await expect(
              main.getByRole("heading", { name: entry.title, exact: true }),
            ).toBeVisible();
            await expect(
              main.getByText(entry.description, { exact: true }),
            ).toBeVisible();

            console.log(
              `Dashboard (${projectType}): ${entry.label} → ${entry.title}`,
            );
          });
        }
      }
    });
  }
});
