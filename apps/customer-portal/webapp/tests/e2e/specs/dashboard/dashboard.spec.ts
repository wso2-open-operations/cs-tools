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

import {
  test,
  expect,
  withSession,
  type Locator,
  type Page,
  type Response,
} from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { DashboardPage } from "../../pages/DashboardPage";
import { EngagementsPage } from "../../pages/EngagementsPage";
import { SideNavPage } from "../../pages/SideNavPage";
import { ProjectSwitcherPage } from "../../pages/ProjectSwitcherPage";
import {
  DASHBOARD_OPERATIONS_VISIBILITY,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import {
  ACCOUNT_MENU,
  DASHBOARD,
  ENGAGEMENTS,
  PORTAL_HOME,
  PROJECT_SWITCHER,
  SIDE_NAV,
} from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import {
  type CaseSearchFilters,
  type CaseSearchResult,
  type CaseSearchResultItem,
  caseSearchFiltersOf,
  caseSearchWithPagination,
  caseSearchWithSeverity,
  caseSearchWithoutSeverity,
  dashboardCasesSearch,
  engagementChartSearch,
  isUnfilteredDashboardSearch,
  myCasesSearchResponse,
  recordDashboardCasesSearches,
  sameIds,
} from "../../utils/listSearch";
import {
  type FilterOption,
  type ProjectContact,
  type ProjectFiltersBody,
  engagementStatsResponse,
  hasDeployments,
  projectContactsResponse,
  projectFeaturesResponse,
  projectFiltersResponse,
} from "../../utils/projectMetadata";
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

        // All five categories, every time. They are rendered from a fixed list
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

      // DSH-28. Status is the one filter whose field the table also shows, so it
      // is checked three ways: the request carries exactly the ticked state ids,
      // every case the backend returns is in one of those states, and every row
      // rendered shows one of them.
      //
      // Two states are ticked where the data allows, because the filter is
      // multi-select and the contract is "any of", not "the last one clicked".
      // They are taken from a sample of the unfiltered table, so the filtered
      // result is known to be non-empty — a filter that matched nothing would
      // pass every "each row is …" check vacuously.
      test("filters the cases table by status and clears it", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const table = await openCasesTableBaseline(page, project.id);
        test.skip(
          table.baseline.cases.length === 0,
          `${projectType} has no outstanding cases, so there is no status ` +
            `to filter to.`,
        );

        const options = (table.metadata.caseStates ?? []).filter(
          (state) => !isClosedLabel(state.label),
        );
        const present = uniqueLabels(
          table.baseline.cases.map((item) => item.status?.label),
        );
        const offered = present.filter((label) =>
          options.some((option) => option.label === label),
        );
        expect(
          offered,
          `the table shows cases in ${JSON.stringify(present)}, but the ` +
            `Status filter offers ${JSON.stringify(options.map((o) => o.label))} ` +
            `— a status on screen should be filterable`,
        ).not.toHaveLength(0);

        // A second state for the multi-select. The first is on screen, so the
        // result cannot be empty. The second is another state on screen only
        // when that still leaves a third out; otherwise it is a state the sample
        // does not hold — so whenever the sample has two or more states, the
        // selection excludes some of it and narrowing can be asserted.
        const second =
          offered.length >= 3
            ? offered[1]
            : (options.find((option) => !present.includes(option.label))
                ?.label ??
              options.find((option) => option.label !== offered[0])?.label);
        const labels = second ? [offered[0], second] : [offered[0]];
        const ids = labels.map((label) =>
          Number(options.find((option) => option.label === label)?.id),
        );

        const { filtered, narrowingProven } = await exerciseCasesTableFilter(
          page,
          table,
          {
            filter: DASHBOARD.casesTable.filters.status,
            labels,
            carriesSelection: (filters) => sameIds(filters.statusIds, ids),
            caseMatches: (item) => labels.includes(item.status?.label ?? ""),
            describeCase: (item) => item.status?.label ?? "no status",
            rowMatches: (row) => labels.some((label) => row.includes(label)),
          },
        );

        console.log(
          `Dashboard (${projectType}): status filter ` +
            `${JSON.stringify(labels)} (ids ${ids.join(",")}) → ` +
            `${filtered.totalRecords} of ${table.baseline.totalRecords}, ` +
            `verified on request, response and rows; narrowing ` +
            `${narrowingProven ? "asserted" : "unprovable (sample all matched)"}; then cleared`,
        );
      });

      // DSH-29. The table has no category column, so the rows cannot show the
      // filter took. It is verified on the wire instead: the request carries the
      // chosen issue-type id in `issueIds`, and every case the backend returns
      // has that issue type. The rows are then checked to come from those cases.
      test("filters the cases table by category and clears it", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const table = await openCasesTableBaseline(page, project.id);
        test.skip(
          table.baseline.cases.length === 0,
          `${projectType} has no outstanding cases, so there is no category ` +
            `to filter to.`,
        );

        const options = table.metadata.issueTypes ?? [];
        const present = uniqueLabels(
          table.baseline.cases.map((item) => item.issueType?.label),
        );
        // The least common category in the sample, so that when the sample
        // holds more than one the filter has something to drop.
        const frequency = (label: string): number =>
          table.baseline.cases.filter((item) => item.issueType?.label === label)
            .length;
        const chosen = options
          .filter((option) => present.includes(option.label))
          .sort((a, b) => frequency(a.label) - frequency(b.label))[0];
        test.skip(
          !chosen,
          `none of ${projectType}'s sampled outstanding cases has ` +
            `an issue type the Category filter offers (cases: ` +
            `${JSON.stringify(present)}, filter: ` +
            `${JSON.stringify(options.map((o) => o.label))}), so no choice ` +
            `is known to return data.`,
        );
        const category = chosen as FilterOption;
        const id = Number(category.id);

        const { filtered, narrowingProven } = await exerciseCasesTableFilter(
          page,
          table,
          {
            filter: DASHBOARD.casesTable.filters.category,
            labels: [category.label],
            carriesSelection: (filters) => sameIds(filters.issueIds, [id]),
            caseMatches: (item) => item.issueType?.label === category.label,
            describeCase: (item) => item.issueType?.label ?? "no issue type",
          },
        );

        console.log(
          `Dashboard (${projectType}): category filter "${category.label}" ` +
            `(issueIds [${id}]) → ${filtered.totalRecords} of ` +
            `${table.baseline.totalRecords}, verified on request and response ` +
            `(no category column); rows matched the response; narrowing ` +
            `${narrowingProven ? "asserted" : "unprovable (sample all matched)"}; then cleared`,
        );
      });

      // DSH-30. Created by is offered per project contact, labelled by name and
      // sent as the contact's email. The Created by column shows the email, so
      // this is checked on the request, the response and the rows.
      //
      // The creator is taken from the unfiltered sample *and* the contact list:
      // a case raised by someone no longer a contact is on screen but cannot be
      // filtered to, which is not a fault in the filter.
      test("filters the cases table by creator and clears it", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const contactsResponse = projectContactsResponse(page, project.id);
        const table = await openCasesTableBaseline(page, project.id);
        const contacts = await contactsResponse;
        test.skip(
          table.baseline.cases.length === 0,
          `${projectType} has no outstanding cases, so there is no creator ` +
            `to filter to.`,
        );

        // Labelled exactly as CasesTable labels the option.
        const optionLabel = (contact: ProjectContact): string =>
          `${contact.firstName ?? ""} ${contact.lastName ?? ""}`.trim() ||
          contact.email;
        const creators = uniqueLabels(
          table.baseline.cases.map((item) => item.createdBy?.toLowerCase()),
        );
        const candidate = contacts.find(
          (contact) =>
            creators.includes(contact.email.toLowerCase()) &&
            // An option name shared by two contacts could not be told apart.
            contacts.filter((c) => optionLabel(c) === optionLabel(contact))
              .length === 1,
        );
        test.skip(
          !candidate,
          `none of the creators of ${projectType}'s sampled outstanding ` +
            `cases (${JSON.stringify(creators)}) is a uniquely named project ` +
            `contact, so the Created By filter offers no choice known to ` +
            `return data.`,
        );
        const contact = candidate as ProjectContact;
        const email = contact.email.toLowerCase();

        const { filtered, narrowingProven } = await exerciseCasesTableFilter(
          page,
          table,
          {
            filter: DASHBOARD.casesTable.filters.createdBy,
            labels: [optionLabel(contact)],
            carriesSelection: (filters) =>
              sameIds(filters.createdBy, [contact.email]),
            caseMatches: (item) => item.createdBy?.toLowerCase() === email,
            describeCase: (item) => item.createdBy ?? "no creator",
            rowMatches: (row) => row.toLowerCase().includes(email),
          },
        );

        console.log(
          `Dashboard (${projectType}): created-by filter ` +
            `"${optionLabel(contact)}" <${contact.email}> → ` +
            `${filtered.totalRecords} of ${table.baseline.totalRecords}, ` +
            `verified on request, response and rows; narrowing ` +
            `${narrowingProven ? "asserted" : "unprovable (sample all matched)"}; then cleared`,
        );
      });

      // DSH-31. The Deployment filter is withheld unless the project has
      // deployments — `includeDeploymentFilter={permissions.hasDeployments}` in
      // DashboardPage.tsx, where hasDeployments is the features response's
      // deployment read *or* write flag (getProjectPermissions).
      //
      // The flag is read from the same features response the page received
      // rather than recorded per project, so the assertion follows whatever the
      // backend says today — and the run's output records which way each
      // project went.
      test("offers the Deployment filter only when the project has deployments", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const featuresResponse = projectFeaturesResponse(page, project.id);
        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);
        const features = await featuresResponse;
        const expected = hasDeployments(features);

        await dashboard.casesTableFiltersButton().click();

        // The Status filter is rendered for every project, so it marks the panel
        // as open and populated — without it an absent Deployment filter could
        // just be a panel still opening.
        const filters = DASHBOARD.casesTable.filters;
        await expect(dashboard.filterSelect(filters.status.selectId)).toBeVisible();

        const deployment = dashboard.filterSelect(filters.deployment.selectId);
        if (expected) {
          await expect(
            deployment,
            `${projectType} has deployments, so the Deployment filter should ` +
              `be offered`,
          ).toBeVisible();
          await expect(
            dashboard.filterLabel(filters.deployment.selectId),
          ).toHaveText(filters.deployment.label);
        } else {
          await expect(
            deployment,
            `${projectType} has no deployments, so the Deployment filter ` +
              `should be withheld`,
          ).toHaveCount(0);
        }

        console.log(
          `Dashboard (${projectType}): hasDeployments=${expected} ` +
            `(read=${!!features.hasDeploymentReadAccess}, ` +
            `write=${!!features.hasDeploymentWriteAccess}) → Deployment ` +
            `filter ${expected ? "offered" : "withheld"}`,
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

      // DSH-02. The card and the page it opens are two independent queries: the
      // card sums `GET /projects/{id}/stats/cases` (actionRequiredCount /
      // outstandingCount / resolvedCases.pastThirtyDays, for the case types the
      // project's features allow) plus the change-request stats when hasCR; the
      // page runs one search per section — cases, SRs, SRA, engagements, CRs,
      // gated on the same feature flags — filtered by state labels and, for
      // Closed, a closedStartDate/closedEndDate window. DashboardPage's own
      // comment says the scoping exists so these "match what is visible on the
      // page", so equality is the contract and a difference is a finding.
      //
      // Compared against the sum of the section chips, not the rows: each
      // section is a single page of 10, so rows undercount anything larger.
      for (const target of DASHBOARD.statCardTargets) {
        test(`the ${target.label} card value matches the total of the ${target.title} list`, async ({
          page,
        }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const dashboard = new DashboardPage(page);
          await dashboard.openViaSideNav(project.id);

          const cardValue = await dashboard.statCardValue(target.label);
          expect(
            cardValue,
            `the ${target.label} card failed to load its stats, so there is ` +
              "no value to compare",
          ).not.toBeNull();

          await dashboard.statCardButton(target.label).click();
          await expect(page).toHaveURL(
            new RegExp(`/projects/${project.id}/${target.pathSegment}$`),
          );

          const totals = await dashboard.itemSectionTotals(target.emptyMessage);

          // A section whose search failed still renders, with its chip at 0 —
          // summing that would compare the card against a broken list.
          await expect(
            dashboard.errorIndicators(),
            `a section of ${target.title} failed to load, so its total is not real`,
          ).toHaveCount(0);

          const listTotal = Object.values(totals).reduce(
            (sum, count) => sum + count,
            0,
          );
          const breakdown =
            Object.entries(totals)
              .map(([section, count]) => `${section} ${count}`)
              .join(", ") || "empty";

          expect(
            listTotal,
            `the ${target.label} card shows ${cardValue}, but ${target.title} ` +
              `lists ${listTotal} (${breakdown})`,
          ).toBe(cardValue);

          console.log(
            `Dashboard (${projectType}): ${target.label} card ${cardValue} = ` +
              `${target.title} total ${listTotal} (${breakdown})`,
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
        if (
          "projectTypes" in entry &&
          !(entry.projectTypes as readonly string[]).includes(projectType)
        ) {
          continue;
        }
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

      // The Outstanding Engagements legend, entry by entry. Unlike the severity
      // legend, the type does not travel in the URL — DashboardPage navigates to
      // the bare engagements path with `engagementTypeId`/`engagementTypeLabel`
      // in router state — so the heading built from that state and the search
      // request's `engagementTypeKeys` are what prove the filter arrived. The
      // returned cases are then checked to be of that type, and their count
      // against the number the legend shows for it.
      for (const entry of DASHBOARD.engagementLegend) {
        test(`the ${entry.label} engagements legend entry opens its list`, async ({
          page,
        }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const heading = DASHBOARD.sections.outstandingEngagements;

          // Both armed before navigating: the features decide whether the card
          // exists, and the engagement stats are what its legend is drawn from
          // — reading them off the page's own requests is what lets the
          // drill-down be compared with what the legend actually showed.
          const featuresResponse = projectFeaturesResponse(page, project.id);
          const statsResponse = engagementStatsResponse(page, project.id);

          const dashboard = new DashboardPage(page);
          await dashboard.openViaSideNav(project.id);

          // Gated on `permissions.hasEngagements` — the same flag the
          // "renders the Outstanding Engagements breakdown" test skips on, read
          // here from the features response rather than inferred from a
          // missing legend, so an empty chart is not mistaken for a withheld one.
          const features = await featuresResponse;
          test.skip(
            !features.hasEngagementsReadAccess,
            `${projectType} does not expose the Outstanding Engagements ` +
              `breakdown (permissions.hasEngagements).`,
          );

          // Summed per category exactly as DashboardPage does: every backend
          // type whose label matches the display name, case-insensitively —
          // one display name can stand for several backend ids.
          const outstanding = (await statsResponse)
            .outstandingEngagementTypeCount ?? [];
          const ofCategory = (label: string) =>
            outstanding.filter(
              (type) => type.label.toLowerCase() === label.toLowerCase(),
            );
          const countOf = (label: string): number =>
            ofCategory(label).reduce((sum, type) => sum + (type.count ?? 0), 0);
          const legendCount = countOf(entry.label);
          const typeIds = ofCategory(entry.label).map((type) => Number(type.id));
          const chartTotal = DASHBOARD.engagementLegend.reduce(
            (sum, category) => sum + countOf(category.label),
            0,
          );

          // Settled on the FIRST legend entry rather than this one, so the wait
          // proves the chart rendered at all; whether this entry is in it is
          // then a separate assertion with its own message. Otherwise a chart
          // that rendered without this entry — a build deployed before it was
          // added to OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA — would time
          // out looking like a chart that never loaded.
          const legend = dashboard.legendEntry(entry.label, heading);
          const firstEntry = dashboard.legendEntry(
            DASHBOARD.engagementLegend[0].label,
            heading,
          );
          await expect(
            firstEntry.or(dashboard.engagementsChartEmpty()),
            "the Outstanding Engagements card should settle into its chart or " +
              "its empty state",
          ).toBeVisible({ timeout: 60_000 });

          // A project with no outstanding engagements at all gets the card's
          // empty state, which has no legend — so there is nothing to click.
          if (chartTotal === 0) {
            await expect(dashboard.engagementsChartEmpty()).toBeVisible();
            await expect(legend).toHaveCount(0);
            test.info().annotations.push({
              type: "empty-chart",
              description:
                `${projectType} has no outstanding engagements; the card ` +
                `shows "${DASHBOARD.engagementsChartEmptyMessage}" and no legend.`,
            });
            test.skip(
              true,
              `${projectType} has no outstanding engagements, so the ` +
                `Outstanding Engagements card renders its empty state and ` +
                `has no ${entry.label} legend entry to click.`,
            );
          }

          // The chart draws every category from a fixed list, zero or not, so on
          // a rendered chart an absent entry is a missing category — never a
          // data gap — and fails rather than skips.
          await expect(
            legend,
            `the "${entry.label}" legend entry is not rendered on the ` +
              "Outstanding Engagements chart, though the chart itself is — is " +
              `the build under test deployed with the "${entry.label}" ` +
              "category (OUTSTANDING_ENGAGEMENTS_CATEGORY_CHART_DATA)?",
          ).toBeVisible();

          // Armed before the click. The ids are those the stats gave this
          // category; none means the legend falls back to the category name as
          // its id, which the page cannot turn into a number — the request then
          // carries no type ids at all.
          const search = engagementChartSearch(page, typeIds);
          await legend.click();

          // Nothing after the path: the type travels in router state, so a
          // query string here would mean the hand-over changed shape.
          // (Not projectPathPattern, which deliberately tolerates a query.)
          await expect(page).toHaveURL(
            new RegExp(`/projects/${project.id}/${ENGAGEMENTS.pathSegment}$`),
          );

          const engagements = new EngagementsPage(page);
          const main = page.getByTestId("app-main");
          await expect(engagements.heading(entry.title)).toBeVisible({
            timeout: 60_000,
          });
          await expect(
            main.getByText(entry.description, { exact: true }),
          ).toBeVisible();

          const response = await search;
          await expectSuccess(
            response,
            `engagements search for ${entry.label} (ids [${typeIds.join(",")}])`,
          );
          const result = (await response.json()) as CaseSearchResult;

          // Every engagement returned is of the clicked type and not Closed —
          // checked on the response, which carries the type the rows only
          // render as a chip.
          const strays = result.cases
            .filter(
              (item) =>
                item.engagementType?.label?.toLowerCase() !==
                  entry.label.toLowerCase() ||
                item.status?.label?.toLowerCase() === "closed",
            )
            .map(
              (item) =>
                `${item.number} (${item.engagementType?.label ?? "no type"}, ` +
                `${item.status?.label ?? "no status"})`,
            );
          expect(
            strays,
            `every engagement listed from the ${entry.label} legend entry ` +
              `should be an outstanding ${entry.label} engagement`,
          ).toEqual([]);

          // The legend's number and the list it opens are two queries — stats
          // and search — so equality is the contract and a difference a
          // finding. Soft, so the row checks below still report.
          expect
            .soft(
              result.totalRecords,
              `the ${entry.label} legend entry shows ${legendCount}, but the ` +
                `list it opens holds ${result.totalRecords}`,
            )
            .toBe(legendCount);

          await engagements.waitForList();

          if (result.cases.length === 0) {
            await expect(
              main.getByText(DASHBOARD.engagementsListEmptyMessage, {
                exact: true,
              }),
            ).toBeVisible();
            await expect(engagements.rows()).toHaveCount(0);
            test.info().annotations.push({
              type: "empty-drilldown",
              description:
                `${projectType}: ${entry.label} has ${legendCount} outstanding ` +
                `engagements; the list shows its empty state, so there were ` +
                `no rows to check the type of.`,
            });
            console.log(
              `Dashboard (${projectType}): ${entry.label} → ${entry.title}, ` +
                `empty (legend ${legendCount}), ids [${typeIds.join(",")}]`,
            );
            return;
          }

          // The rows are the response's engagements, and each shows the type
          // chip — the user-visible half of the check above.
          const numbers = new Set(result.cases.map((item) => item.number));
          const rowCount = await engagements.rows().count();
          expect(rowCount).toBeGreaterThan(0);
          for (let index = 0; index < rowCount; index += 1) {
            const row = engagements.rows().nth(index);
            const number = await engagements.rowNumber(index);
            expect(
              number !== null && numbers.has(number),
              `row ${index + 1} (${number}) should be one of the search's ` +
                `engagements`,
            ).toBe(true);
            await expect(
              // Case-insensitive, as DashboardPage matches backend labels to
              // the display name; the chip shows the backend's own label.
              row.getByText(new RegExp(`^${entry.label}$`, "i")),
              `row ${index + 1} (${number}) should show the ${entry.label} type`,
            ).not.toHaveCount(0);
          }

          console.log(
            `Dashboard (${projectType}): ${entry.label} → ${entry.title}, ` +
              `${result.totalRecords} listed (legend ${legendCount}), ` +
              `${rowCount} rows checked, ids [${typeIds.join(",")}]`,
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

  // ⚠️ WRITES the signed-in user's profile (PATCH /users/me), then restores the
  // original time zone in `finally`. Runs once, not per project type: the
  // profile belongs to the user, not the project.
  test("changes the profile time zone from the account menu and restores it", async ({
    page,
  }) => {
    const project = PROJECTS[ProjectType.SUBSCRIPTION];
    test.skip(!project.id, "Subscription needs a project id.");

    const dashboard = new DashboardPage(page);
    // Armed before navigating, so the user details the modal reads are known
    // to have arrived — saveProfileTimeZone re-arms it before each reload.
    const profile = { userDetails: userDetailsLoad(page) };
    await dashboard.openViaSideNav(project.id);

    const original = await openProfileTimeZone(page, profile.userDetails);
    // The UI cannot clear a time zone, so an account without one could not be
    // put back the way it was.
    test.skip(
      original === ACCOUNT_MENU.timeZonePlaceholder,
      "The test account has no time zone set, so it could not be restored.",
    );
    await closeProfile(page);

    // Set by saveProfileTimeZone as soon as the PATCH succeeds, so a failure
    // after the write (the reload, the re-read) still restores the original.
    const saved = { changed: false };
    try {
      const target = await saveProfileTimeZone(
        page,
        profile,
        (labels) => labels.find((label) => label !== original),
        saved,
      );
      expect(await openProfileTimeZone(page, profile.userDetails)).toBe(target);
      await closeProfile(page);
      console.log(`Profile time zone: ${original} → ${target}`);
    } finally {
      if (saved.changed) {
        const restored = await saveProfileTimeZone(page, profile, (labels) =>
          labels.find((label) => label === original),
        );
        expect(restored).toBe(original);
        expect(await openProfileTimeZone(page, profile.userDetails)).toBe(
          original,
        );
        console.log(`Profile time zone restored to ${original}`);
      }
    }
  });

  // ✅ READ-ONLY. As the Subscription admin (the suite's session): the header's
  // "Customer Portal" link leads to the portal home and its two tables.
  test("opens the portal home from the Customer Portal link", async ({ page }) => {
    const project = PROJECTS[ProjectType.SUBSCRIPTION];
    test.skip(!project.id, "Subscription needs a project id.");

    const dashboard = new DashboardPage(page);
    await dashboard.openViaSideNav(project.id);
    await page
      .locator("header")
      .first()
      .getByText(PORTAL_HOME.headerLink, { exact: true })
      .click();
    await expect(page).toHaveURL(/\/$/, { timeout: 60_000 });

    const projectsHeading = page.getByRole("heading", { name: PORTAL_HOME.projectsHeading });
    const casesHeading = page.getByRole("heading", { name: PORTAL_HOME.casesHeading });
    await expect(projectsHeading).toBeVisible({ timeout: 60_000 });
    await expect(casesHeading).toBeVisible({ timeout: 60_000 });

    const tables = page.getByRole("table");
    await expect(tables).toHaveCount(2);
    const [projectsTable, casesTable] = [tables.nth(0), tables.nth(1)];

    // Projects: its columns, one row per project the heading counts, and the
    // fixture projects among them.
    for (const column of PORTAL_HOME.projectColumns) {
      await expect(
        projectsTable.getByRole("columnheader", { name: column, exact: true }),
      ).toBeVisible();
    }
    const projectTotal = Number(
      PORTAL_HOME.projectsHeading.exec(await projectsHeading.innerText())![1],
    );
    const projectRows = projectsTable.locator("tbody").getByRole("row");
    await expect(projectRows).toHaveCount(projectTotal);
    for (const fixture of Object.values(PROJECTS)) {
      if (fixture.name) {
        await expect(projectsTable.getByText(fixture.name, { exact: true })).toBeVisible();
      }
    }

    // Cases: its columns, and rows; View More offers the rest when the total
    // exceeds what is rendered.
    for (const column of PORTAL_HOME.caseColumns) {
      await expect(
        casesTable.getByRole("columnheader", { name: column, exact: true }),
      ).toBeVisible();
    }
    const caseTotal = Number(PORTAL_HOME.casesHeading.exec(await casesHeading.innerText())![1]);
    const caseRows = casesTable.locator("tbody").getByRole("row");
    await expect(caseRows.first()).toBeVisible();
    const shown = await caseRows.count();
    expect(shown).toBeLessThanOrEqual(caseTotal);
    if (shown < caseTotal) {
      const viewMore = page.getByRole("button", { name: PORTAL_HOME.viewMoreButton });
      await expect(viewMore).toBeVisible();
      const more = Number(PORTAL_HOME.viewMoreButton.exec(await viewMore.innerText())![1]);
      expect(shown + more, "View More should offer the rest of the cases").toBe(caseTotal);
    }

    console.log(
      `Portal home: ${projectTotal} project(s); ${shown} of ${caseTotal} case(s) shown`,
    );
  });

  // ✅ READ-ONLY. Switches project from the header, as the Subscription admin
  // session (the suite's signed-in user), and back again.
  test("switches project from the top bar and loads its dashboard", async ({
    page,
  }) => {
    const from = PROJECTS[ProjectType.SUBSCRIPTION];
    const to = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];
    test.skip(!from.id || !to.id, "Needs both project ids.");

    const dashboard = new DashboardPage(page);
    const switcher = new ProjectSwitcherPage(page);
    const header = page.locator("header").first();

    /** Opens the switcher on the current project and picks another. */
    const switchTo = async (
      current: { name: string },
      target: { id: string; name: string },
    ) => {
      await header.getByText(current.name, { exact: true }).first().click();
      const filter = page.getByPlaceholder(PROJECT_SWITCHER.searchPlaceholder);
      if (await filter.isVisible().catch(() => false)) {
        await filter.fill(target.name);
      }
      const option = switcher.option(target.name);
      await expect(option, `"${target.name}" should be offered`).toBeVisible({
        timeout: 60_000,
      });
      await option.click();

      // The target's dashboard: its URL, the header naming it, and the stat
      // cards loading for it.
      await expect(page).toHaveURL(projectPathPattern(target.id, "dashboard"), {
        timeout: 60_000,
      });
      await expect(header.getByText(target.name, { exact: true }).first()).toBeVisible({
        timeout: 60_000,
      });
      await expect(header.getByText(current.name, { exact: true })).toHaveCount(0);
      await expect(dashboard.statCard(DASHBOARD.statCards[0].label)).toBeVisible({
        timeout: 60_000,
      });
    };

    await dashboard.openViaSideNav(from.id);
    await expect(header.getByText(from.name, { exact: true }).first()).toBeVisible();

    await switchTo(from, to);
    await switchTo(to, from);

    console.log(`Dashboard: switched ${from.name} → ${to.name} → back`);
  });

  // ✅ READ-ONLY. The collapsed state is kept in the browser only
  // (setSidebarCollapsed), never sent to the backend.
  test("expands and collapses the side menu from the header toggle", async ({
    page,
  }) => {
    const project = PROJECTS[ProjectType.SUBSCRIPTION];
    test.skip(!project.id, "Subscription needs a project id.");

    const dashboard = new DashboardPage(page);
    await dashboard.openViaSideNav(project.id);
    const sideNav = new SideNavPage(page);
    // SideNavPage.sidebar() matches both nested <aside>s, so measure Oxygen's
    // inner one; labels are still looked up through the page object.
    const sidebar = page
      .getByTestId(SIDE_NAV.sidebarTestId)
      .getByRole("complementary");
    const expandToggle = page.getByRole("button", {
      name: SIDE_NAV.expandToggle,
    });
    const collapseToggle = page.getByRole("button", {
      name: SIDE_NAV.collapseToggle,
    });

    // The state persists per browser, so the run may start expanded; collapse
    // first so the test always begins from "Expand sidebar".
    if (await collapseToggle.isVisible()) {
      await collapseToggle.click();
    }
    await expect(expandToggle).toBeVisible();
    await expectSidebarWidth(sidebar, SIDE_NAV.collapsedWidthPx);

    await expandToggle.click();
    await expect(collapseToggle).toBeVisible();
    await expect(expandToggle).toHaveCount(0);
    await expectSidebarWidth(sidebar, SIDE_NAV.expandedWidthPx);
    // Expanded, the items show their labels, not just icons.
    await expect(
      sideNav.sidebar().first().getByText(SIDE_NAV.items.dashboard, {
        exact: true,
      }),
    ).toBeVisible();

    await collapseToggle.click();
    await expect(expandToggle).toBeVisible();
    await expect(collapseToggle).toHaveCount(0);
    await expectSidebarWidth(sidebar, SIDE_NAV.collapsedWidthPx);
  });
});

/**
 * Waits for the sidebar to settle at a width (it animates between states).
 *
 * @param sidebar - The sidebar landmark.
 * @param widthPx - The expected width, allowing a few px for borders.
 * @returns Resolves once the width is within tolerance.
 */
async function expectSidebarWidth(
  sidebar: Locator,
  widthPx: number,
): Promise<void> {
  await expect
    .poll(async () => Math.round((await sidebar.boundingBox())?.width ?? 0), {
      message: `the sidebar should settle at about ${widthPx}px wide`,
      timeout: 10_000,
    })
    .toBeGreaterThanOrEqual(widthPx - 4);
  await expect
    .poll(async () => Math.round((await sidebar.boundingBox())?.width ?? 0))
    .toBeLessThanOrEqual(widthPx + 4);
}

/** How long to allow the profile modal and its post-save reload. */
const PROFILE_TIMEOUT_MS = 60_000;

/**
 * Starts waiting for the shell's `GET /users/me`, which the profile modal takes
 * its time zone from.
 *
 * Call this **before** the navigation or reload that issues it.
 *
 * @param page - The page under test.
 * @returns Promise for the user details response.
 */
function userDetailsLoad(page: Page): Promise<Response> {
  return page.waitForResponse(
    (r) =>
      r.request().method() === "GET" &&
      /\/users\/me$/.test(new URL(r.url()).pathname),
    { timeout: PROFILE_TIMEOUT_MS },
  );
}

/**
 * Opens Account → Profile and the Time Zone select's option list.
 *
 * Readiness comes from the `GET /users/me` response, not from the select's
 * text: the select shows its placeholder both while loading and when the user
 * genuinely has no time zone, so the placeholder disappearing cannot be waited
 * for.
 *
 * @param page - The page under test.
 * @param userDetails - The `GET /users/me` response for the current page load.
 * @returns The selected time zone's label, or the placeholder when none is set.
 */
async function openProfileTimeZone(
  page: Page,
  userDetails: Promise<Response>,
): Promise<string> {
  const response = await userDetails;
  expect(response.ok(), `GET /users/me returned ${response.status()}`).toBe(
    true,
  );
  const { timeZone } = (await response.json()) as { timeZone?: string | null };

  await page.getByRole("button", { name: ACCOUNT_MENU.trigger }).click();
  await page
    .getByRole("menuitem", { name: ACCOUNT_MENU.profileItem })
    .click();
  const modal = page.getByRole("dialog");
  const select = modal.getByRole("combobox", {
    name: ACCOUNT_MENU.timeZoneSelect,
  });
  await expect(select).toBeVisible({ timeout: PROFILE_TIMEOUT_MS });
  await select.click();
  // Options come from /metadata, so their presence means the labels are known.
  await expect(page.getByRole("option").first()).toBeVisible({
    timeout: PROFILE_TIMEOUT_MS,
  });
  if (!timeZone) return ACCOUNT_MENU.timeZonePlaceholder;

  // Read the label off the selected option rather than the select: the select
  // falls back to the raw IANA id until the metadata labels it.
  const selected = page.getByRole("option", { selected: true });
  await expect(selected).toHaveCount(1, { timeout: PROFILE_TIMEOUT_MS });
  return (await selected.innerText()).trim();
}

/**
 * Closes the Time Zone option list, then the profile modal, without saving.
 *
 * @param page - The page under test.
 * @returns Resolves once the modal is gone.
 */
async function closeProfile(page: Page): Promise<void> {
  await page.keyboard.press("Escape");
  await expect(page.getByRole("listbox")).toHaveCount(0);
  await page.getByRole("button", { name: ACCOUNT_MENU.closeButton }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
}

/**
 * Opens the profile, picks a time zone and saves it, waiting for the PATCH and
 * the reload a time zone change triggers.
 *
 * @param page - The page under test.
 * @param profile - Holds the `GET /users/me` wait for the current page load;
 *   replaced with the post-reload one, armed before the save is clicked.
 * @param pick - Chooses the label to select from the offered options.
 * @param saved - Optional flag set to true once the PATCH has succeeded.
 * @returns The label that was saved.
 */
async function saveProfileTimeZone(
  page: Page,
  profile: { userDetails: Promise<Response> },
  pick: (labels: string[]) => string | undefined,
  saved?: { changed: boolean },
): Promise<string> {
  await openProfileTimeZone(page, profile.userDetails);
  const labels = (await page.getByRole("option").allInnerTexts()).map((l) =>
    l.trim(),
  );
  const target = pick(labels);
  expect(target, "no suitable time zone option was offered").toBeTruthy();
  await page.getByRole("option", { name: target!, exact: true }).click();

  const modal = page.getByRole("dialog");
  await expect(
    modal.getByRole("combobox", { name: ACCOUNT_MENU.timeZoneSelect }),
  ).toHaveText(target!);

  const patch = page.waitForResponse(
    (r) =>
      r.request().method() === "PATCH" && /\/users\/me$/.test(r.url()),
  );
  const reload = page.waitForEvent("load");
  profile.userDetails = userDetailsLoad(page);
  await modal.getByRole("button", { name: ACCOUNT_MENU.saveButton }).click();
  const response = await patch;
  expect(response.ok(), `PATCH /users/me returned ${response.status()}`).toBe(
    true,
  );
  if (saved) saved.changed = true;
  // The success toast is not asserted: a time zone change reloads the page
  // straight after showing it, so it is usually gone before it can be seen.
  // The re-read after the reload is the proof the value was stored.
  await reload;
  await expect(
    page.getByRole("button", { name: ACCOUNT_MENU.trigger }),
  ).toBeVisible({ timeout: PROFILE_TIMEOUT_MS });
  return target!;
}

/** How long to allow the cases table to re-render after its search returns. */
const TABLE_TIMEOUT_MS = 60_000;

/** The dashboard's cases table at rest, unfiltered, with the metadata its
 * filters are built from. */
interface CasesTableBaseline {
  dashboard: DashboardPage;
  metadata: ProjectFiltersBody;
  /** The non-Closed state ids the table searches with when no status is
   * ticked — what an unfiltered (or cleared) search carries. */
  defaultStatusIds: number[];
  /** The unfiltered search's result, at {@link SAMPLE_ROWS} rows. Its cases
   * are what the specs pick filter values from, so each choice is known to
   * match something — and whether it should exclude something. */
  baseline: CaseSearchResult;
}

/** Rows per page to sample the unfiltered table at — the largest page size it
 * offers. The default 5 rows is too few to find a second status, category or
 * creator on most projects, and without one a filter that the backend ignored
 * would be indistinguishable from one that matched everything. */
const SAMPLE_ROWS = 50;

/** Whether a case-state label is Closed, which the table neither searches by
 * default nor offers as a filter option (`isClosedCaseStatusLabel`). */
function isClosedLabel(label: string): boolean {
  return label.trim().toLowerCase() === "closed";
}

/** Distinct, non-empty values, in first-seen order. */
function uniqueLabels(values: Array<string | null | undefined>): string[] {
  return [...new Set(values.filter((value): value is string => !!value))];
}

/**
 * Opens the dashboard and waits for the cases table's unfiltered search.
 *
 * The filter metadata is captured from the page's own request so the specs
 * can map option labels to the ids the request should carry, and the default
 * status set is derived from it exactly as CasesTable derives it.
 *
 * @param page - Test page.
 * @param projectId - Project to open.
 * @returns The table at rest.
 */
async function openCasesTableBaseline(
  page: Page,
  projectId: string,
): Promise<CasesTableBaseline> {
  const metadataResponse = projectFiltersResponse(page, projectId);
  const searches = recordDashboardCasesSearches(page);

  const dashboard = new DashboardPage(page);
  await dashboard.openViaSideNav(projectId);
  const metadata = await metadataResponse;

  const fromMetadata = (metadata.caseStates ?? [])
    .filter((state) => !isClosedLabel(state.label))
    .map((state) => Number(state.id));
  const defaultStatusIds =
    fromMetadata.length > 0
      ? fromMetadata
      : [...DASHBOARD.casesTable.fallbackStatusIds];

  let unfiltered: Response | undefined;
  await expect
    .poll(
      () => {
        unfiltered = [...searches].reverse().find((response) => {
          const filters = caseSearchFiltersOf(response);
          return (
            !!filters && isUnfilteredDashboardSearch(filters, defaultStatusIds)
          );
        });
        return !!unfiltered;
      },
      {
        message:
          "the cases table should search with the default outstanding states",
        timeout: TABLE_TIMEOUT_MS,
      },
    )
    .toBe(true);
  const response = unfiltered as Response;
  await expectSuccess(response, "unfiltered cases search");
  let baseline = (await response.json()) as CaseSearchResult;

  await dashboard.waitForCasesTableSettled();

  // Widen the sample when there is more than the first page to see. Only the
  // page size changes, so the filters stay the unfiltered set.
  if (baseline.totalRecords > baseline.cases.length) {
    const widened = caseSearchWithPagination(page, {
      limit: SAMPLE_ROWS,
      offset: 0,
    });
    await dashboard.selectRowsPerPage(SAMPLE_ROWS);
    const widenedResponse = await widened;
    await expectSuccess(widenedResponse, "unfiltered cases search, widened");
    baseline = (await widenedResponse.json()) as CaseSearchResult;
    await expectRowsFrom(dashboard, baseline, "widened unfiltered");
  }

  return { dashboard, metadata, defaultStatusIds, baseline };
}

/** One filter, how to choose it, and how to tell it took. */
interface CasesTableFilterExercise {
  filter: { label: string; selectId: string };
  /** Option labels to tick. The first must be known to match a case. */
  labels: string[];
  /** Whether a search request carries exactly this selection. */
  carriesSelection: (filters: CaseSearchFilters) => boolean;
  /** Whether a returned case satisfies the selection. */
  caseMatches: (item: CaseSearchResultItem) => boolean;
  /** How a case's filtered field reads, for the failure message. */
  describeCase: (item: CaseSearchResultItem) => string;
  /** Whether a rendered row shows the selection — only for filters whose
   * field the table displays. */
  rowMatches?: (row: string) => boolean;
}

/**
 * Polls until the table's rows are a non-empty subset of a search's cases.
 *
 * A subset rather than equal: the table drops S0 cases client-side on projects
 * that exclude them (`excludeS0`), and only those, so every row must still come
 * from the response.
 *
 * @param dashboard - The dashboard.
 * @param result - The search the rows should come from.
 * @param what - Which search, for the failure message.
 */
async function expectRowsFrom(
  dashboard: DashboardPage,
  result: CaseSearchResult,
  what: string,
): Promise<void> {
  const numbers = new Set(result.cases.map((item) => item.number));
  await expect
    .poll(
      async () => {
        const rows = await dashboard.casesTableCaseNumbers();
        return (
          rows.length > 0 &&
          rows.length <= numbers.size &&
          rows.every((row) => numbers.has(row))
        );
      },
      {
        message: `the table's rows should be the ${what} search's cases`,
        timeout: TABLE_TIMEOUT_MS,
      },
    )
    .toBe(true);
}

/**
 * Applies a filter to the cases table, checks it narrowed the table, then
 * clears it and checks the table came back unfiltered.
 *
 * @param page - Test page.
 * @param table - The table at rest, from {@link openCasesTableBaseline}.
 * @param exercise - The filter and how to verify it.
 * @returns The filtered search's result, and whether the sample held a case
 *   outside the selection — i.e. whether narrowing was actually asserted.
 */
async function exerciseCasesTableFilter(
  page: Page,
  table: CasesTableBaseline,
  exercise: CasesTableFilterExercise,
): Promise<{ filtered: CaseSearchResult; narrowingProven: boolean }> {
  const { dashboard, defaultStatusIds, baseline } = table;
  const { filter, labels } = exercise;
  const select = dashboard.filterSelect(filter.selectId);

  // The panel is collapsed on load and its contents unmounted.
  await expect(select).toHaveCount(0);
  await dashboard.casesTableFiltersButton().click();
  await expect(select).toBeVisible();
  await expect(dashboard.filterLabel(filter.selectId)).toHaveText(filter.label);

  // Armed before ticking: each tick searches on its own, and only the request
  // carrying the whole selection is the filter applied.
  const applied = dashboardCasesSearch(page, exercise.carriesSelection);
  await dashboard.selectFilterOptions(filter.selectId, labels);
  const appliedResponse = await applied;
  await expectSuccess(appliedResponse, `cases search filtered by ${filter.label}`);
  const filtered = (await appliedResponse.json()) as CaseSearchResult;

  await expect(select).toContainText(labels[0]);

  // The first label was taken from a case on screen, so an empty result is a
  // wrong answer rather than a quiet project.
  expect(
    filtered.cases.length,
    `filtering by ${filter.label} ${JSON.stringify(labels)} should return the ` +
      `case it was chosen from`,
  ).toBeGreaterThan(0);

  const strays = filtered.cases
    .filter((item) => !exercise.caseMatches(item))
    .map((item) => `${item.number} (${exercise.describeCase(item)})`);
  expect(
    strays,
    `every case returned for ${filter.label} ${JSON.stringify(labels)} ` +
      `should match it`,
  ).toEqual([]);

  // Narrowing is only provable when the sample holds a case outside the
  // selection; then the filtered total must be smaller than the unfiltered
  // one. Without such a case — every sampled case already matches — a filter
  // the backend ignored would return the same total, so nothing is claimed.
  const excluded = baseline.cases.filter((item) => !exercise.caseMatches(item));
  if (excluded.length > 0) {
    expect(
      filtered.totalRecords,
      `filtering by ${filter.label} ${JSON.stringify(labels)} should drop ` +
        `cases outside it, e.g. ${excluded[0].number} ` +
        `(${exercise.describeCase(excluded[0])})`,
    ).toBeLessThan(baseline.totalRecords);
  }

  await expectRowsFrom(dashboard, filtered, "filtered");
  if (exercise.rowMatches) {
    const rows = await dashboard.casesTableRows().allInnerTexts();
    for (const [index, row] of rows.entries()) {
      expect(
        exercise.rowMatches(row),
        `row ${index + 1} should show ${filter.label} ${JSON.stringify(labels)}: ` +
          `${row.replace(/\s+/g, " ")}`,
      ).toBe(true);
    }
  }

  // One field is active however many of its options are ticked.
  const clearFilters = dashboard.clearFiltersButton(1);
  await expect(clearFilters).toBeVisible();
  await expect(dashboard.casesTableFiltersButton()).toHaveCount(0);

  // Clearing goes back to the default status set with nothing else — the same
  // search the page loaded with. Armed before the click so the page's own
  // first load cannot satisfy it.
  const cleared = dashboardCasesSearch(page, (filters) =>
    isUnfilteredDashboardSearch(filters, defaultStatusIds),
  );
  await clearFilters.click();
  const clearedResponse = await cleared;
  await expectSuccess(clearedResponse, "cases search after clearing");
  const restored = (await clearedResponse.json()) as CaseSearchResult;

  await expect(dashboard.casesTableFiltersButton()).toBeVisible();
  await expect(select).not.toContainText(labels[0]);
  expect(
    restored.totalRecords,
    "clearing should restore the unfiltered total",
  ).toBe(baseline.totalRecords);
  await expectRowsFrom(dashboard, restored, "cleared");

  return { filtered, narrowingProven: excluded.length > 0 };
}
