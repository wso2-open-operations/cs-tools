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
// The header search bar, driven from the dashboard: search, then open a result.
//
// Three query shapes, which is the whole point — the bar sends one `searchQuery`
// string to the cases API, and these cover the fields a user actually types at
// it:
//
//   case number        CS0448707                 — one case
//   WSO2 case id       AUTOMATIONTESTCUSSUB-210  — the SAME case, found another way
//   project key        AUTOMATIONTESTCUSSUB      — project-wide
//
// The first two name one record, so they assert that exact record is found AND
// that both land on it — different queries reaching the same case is the part
// worth pinning. The third matches many records whose composition depends on
// live project data, so it asserts the shape instead: results render, and the
// one opened leads to a real record page.
//
// Read-only. Searching and viewing change nothing.
//
// Run against all three project types, each with its OWN case and key — the bar
// is project-scoped, so one project's case number returns nothing while another
// is selected, and sharing fixtures across types would test nothing. The ids
// above are the Subscription set; see GLOBAL_SEARCH_INPUT for the rest.
//
// ⚠️ The bar is project-SCOPED despite sitting in the global header: it searches
// the selected project's cases and change requests. From the project hub it
// offers "Select a project to search." instead — hence the dashboard, which
// guarantees a project context. Each test therefore SELECTS the project through
// the header switcher first; a search issued before that resolves returns
// nothing, which reads as a search defect rather than a too-early question.
//
// Searching by PROJECT NAME is deliberately not covered. Verified live on
// 2026-10-06: the full project name returns "No results found" — a clean 200
// with zero matches, not an error, with the query reaching the backend intact.
// The downstream search matches CASE fields only, which is also why the project
// key works: it is the leading segment of every WSO2 case id in the project
// (AUTOMATIONTESTCUSSUB-215), not a searchable field in its own right. Recorded
// so the gap reads as a known product behaviour rather than an oversight.
//

import { test, expect, withSession } from "../../fixtures/test";
import { DashboardPage } from "../../pages/DashboardPage";
import { GlobalSearchPage } from "../../pages/GlobalSearchPage";
import { ProjectSwitcherPage } from "../../pages/ProjectSwitcherPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  GLOBAL_SEARCH_INPUT,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { GLOBAL_SEARCH } from "../../utils/selectors";
import { RECORD_ID_PATTERN, projectPathPattern } from "../../utils/ids";

withSession(test);

/**
 * Where a result is allowed to lead, for one project.
 *
 * A union rather than the cases path, because the destination follows the
 * RECORD's type label and not the query: a search can surface announcements,
 * engagements, service requests, security report analyses and change requests
 * alongside plain cases, each routed to its own section. Pinning this to
 * `support/cases` would fail whenever a project-wide query happened to return,
 * say, a service request first.
 *
 * Still anchored on a record id, so "landed on some list page" cannot pass for
 * "opened the record". Built per project, so a result belonging to a DIFFERENT
 * project cannot satisfy it either.
 *
 * @param projectId - The project the search was scoped to.
 * @returns A RegExp matching any record page under that project.
 */
function resultDestination(projectId: string): RegExp {
  return projectPathPattern(
    projectId,
    `(?:support/cases|announcements|engagements|operations/service-requests|` +
      `security-center/security-report-analysis|(?:support|operations)/change-requests)/` +
      RECORD_ID_PATTERN,
  );
}

test.describe("Dashboard — header search", () => {
  // A shell load, the dashboard's own queries, then a debounced search across
  // two endpoints and a navigation into the record.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of Object.values(ProjectType)) {
    const project = PROJECTS[projectType];
    const searchFor = GLOBAL_SEARCH_INPUT[projectType];
    const RESULT_DESTINATION = resultDestination(project.id);

    test.describe(projectType, () => {
      test.beforeEach(async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );
        test.skip(
          !project.projectKey,
          `${projectType} needs a project key. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );

        const dashboard = new DashboardPage(page);
        await dashboard.openViaSideNav(project.id);

        // Select the project through the header switcher before searching.
        //
        // Not redundant with the navigation above, which is the point: the search
        // bar queries the SELECTED project, and landing on a project URL is not by
        // itself the same thing — a search issued while the header is still
        // resolving its project comes back empty, which reads as "search is
        // broken" rather than "asked too early". Going through the switcher makes
        // the selection an awaited step instead of an assumption.
        const switcher = new ProjectSwitcherPage(page);
        await switcher.selectProject(project.id, project.name);
      });

      test("finds a case by its case number and opens it", async ({ page }) => {
        const search = new GlobalSearchPage(page);
        await search.search(searchFor.number);

        // A case number is unique, so one result — not "at least one". A query that
        // started matching loosely would still satisfy a `>= 1` assertion while
        // making the search useless.
        const card = search.resultCard(searchFor.number);
        await expect(
          card,
          `searching the case number should find exactly ${searchFor.number}`,
        ).toHaveCount(1);

        // The card carries BOTH ids, which is what makes the next test's claim —
        // that the two queries reach one record — checkable from either direction.
        await expect(card).toContainText(searchFor.wso2CaseId);

        await search.openResult(card);

        await expect(page).toHaveURL(RESULT_DESTINATION);

        // The destination really is this case. The URL alone only proves some
        // record opened.
        const caseDetail = new CaseDetailPage(page);
        await expect(caseDetail.caseNumber()).toHaveText(
          searchFor.number,
          { timeout: 60_000 },
        );
      });

      test("finds the same case by its WSO2 case id and opens it", async ({
        page,
      }) => {
        const search = new GlobalSearchPage(page);
        await search.search(searchFor.wso2CaseId);

        // Also unique: the id carries the project key plus a per-project sequence.
        const card = search.resultCard(searchFor.wso2CaseId);
        await expect(
          card,
          `searching the WSO2 case id should find exactly ${searchFor.wso2CaseId}`,
        ).toHaveCount(1);

        await search.openResult(card);

        await expect(page).toHaveURL(RESULT_DESTINATION);

        // The point of this test: a different query, the same case. Asserted on the
        // case number — the field the OTHER query searched — so the two tests are
        // provably about one record rather than two that merely both exist.
        const caseDetail = new CaseDetailPage(page);
        await expect(caseDetail.caseNumber()).toHaveText(
          searchFor.number,
          { timeout: 60_000 },
        );
      });

      test("finds records by project key and opens a result", async ({ page }) => {
        const query = project.projectKey;
        const search = new GlobalSearchPage(page);
        await search.search(query);

        // No count assertion: how many records a project-wide query returns is
        // live data, and the dropdown pages them in as it scrolls. That results
        // exist at all is the claim.
        const cards = search.resultCards();
        await expect(
          cards.first(),
          `searching the project key ("${query}") should return results`,
        ).toBeVisible();

        // Said explicitly, because "no results" is a dropdown state that renders
        // perfectly happily and would otherwise just look like an empty list.
        await expect(
          page.getByText(GLOBAL_SEARCH.emptyStates.noResults),
        ).toHaveCount(0);
        await expect(page.getByText(GLOBAL_SEARCH.emptyStates.error)).toHaveCount(
          0,
        );

        const count = await cards.count();
        const first = cards.first();
        const summary = (await first.innerText()).split("\n")[0];

        await search.openResult(first);

        // Opening the first result lands on a record page. Which record is not
        // asserted — the ordering is `createdOn` DESC over live data, so naming
        // one would make this a test of the project's contents.
        await expect(page).toHaveURL(RESULT_DESTINATION);

        console.log(
          `Header search [project key] "${query}": ${count} result(s); opened "${summary}"`,
        );
      });
    });
  }
});
