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
// The four stat cards across the top of Support Center, and the list each one
// opens. Run against every project type.
//
// Read-only — nothing here creates or changes a record.
//
// Every card navigates whatever its count, zero included (several of these read
// 0 on the test tenant), so the assertions are about the destination page rather
// than what it lists. Each card's list is a filtered view of a route these specs
// already cover unfiltered, so the heading *and* the query string are both
// checked: the heading alone would not distinguish "Outstanding Cases" from any
// other view of the same cases list.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SupportCenterPage } from "../../pages/SupportCenterPage";
import { CasesListPage } from "../../pages/CasesListPage";
import { CASE_LIST_PROJECTS, PROJECTS } from "../../config/testData";
import { CASES_LIST, SUPPORT_CENTER } from "../../utils/selectors";
import {
  type CaseSearchResult,
  caseClosedOn,
  caseSearchBodyOf,
  parseBackendUtc,
  resolvedCasesSearch,
} from "../../utils/listSearch";
import { projectFiltersResponse } from "../../utils/projectMetadata";

withSession(test);

test.describe("Support Center", () => {
  // A dashboard load, a nav navigation, the stats query and then a second page
  // with its own list query — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  for (const projectType of CASE_LIST_PROJECTS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      test("shows the four overview cards", async ({ page }) => {
        test.skip(
          !project.id,
          `${projectType} needs a project id. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );

        const support = new SupportCenterPage(page);
        await support.openViaSideNav(project.id);

        for (const card of SUPPORT_CENTER.statCards) {
          await expect.soft(support.statCard(card.label)).toBeVisible();
        }

        // Exactly four — a fifth card appearing is as much a change as one going
        // missing, and the loop above alone would not notice it. Counted off the
        // page rather than off the list that drove the loop, which would only
        // restate the config.
        await expect(support.statCards()).toHaveCount(4);
      });

      for (const card of SUPPORT_CENTER.statCards) {
        test(`the ${card.label} card opens its list`, async ({ page }) => {
          test.skip(!project.id, `${projectType} needs a project id.`);

          const support = new SupportCenterPage(page);
          await support.openViaSideNav(project.id);

          await support.statCard(card.label).click();

          // The query string is what makes this list the card's list; the
          // heading below would read the same for more than one of these views.
          await expect(page).toHaveURL(
            new RegExp(
              `/projects/${project.id}/${card.pathSegment}\\?${card.query}$`,
            ),
          );

          // Wait for the destination to render before reading its heading. The
          // URL changes first, and until the route swaps the Support Center is
          // still mounted — where this very card's label and the overview card's
          // title are both on screen, so the heading assertion below would
          // resolve to two elements and fail on strict mode instead of retrying.
          await support.waitForList();

          const main = page.getByTestId("app-main");
          await expect(
            main.getByRole("heading", { name: card.title, exact: true }),
          ).toBeVisible();
          await expect(
            main.getByText(card.description, { exact: true }),
          ).toBeVisible();

          // And back, which only offers this label because the card passed a
          // `returnTo` — so the round trip is part of what the card has to get
          // right.
          await support.returnFromList(project.id);
          await expect(support.statCard(card.label)).toBeVisible();
        });
      }

      // UI-SUP-001. The generic card test above already covers the heading and
      // description; what is specific to Outstanding Cases is that the list it
      // opens drops the search panel. AllCasesPage swaps ListSearchPanel (search
      // input + Filters toggle) for a bare Divider whenever `statusFilter` is set,
      // so the panel is unmounted, not hidden — hence `toHaveCount(0)`.
      test("the Outstanding Cases card opens its list without the search panel", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const card = SUPPORT_CENTER.statCards[0];
        const support = new SupportCenterPage(page);
        const cases = new CasesListPage(page);
        const main = page.getByTestId("app-main");

        await support.openViaSideNav(project.id);

        // Control first: the same route, unfiltered, DOES render the panel. An
        // absence check is only worth something once these locators are proven
        // to match the real controls — otherwise a renamed placeholder would
        // turn the assertion below into a silent pass.
        await support
          .outstandingCasesFooterButton(
            SUPPORT_CENTER.outstandingCases.allCasesButton,
          )
          .click();
        await support.waitForList();
        await expect(page).toHaveURL(
          new RegExp(`/projects/${project.id}/${CASES_LIST.pathSegment}$`),
        );
        await expect(
          main.getByRole("heading", {
            name: CASES_LIST.allCases.title,
            exact: true,
          }),
        ).toBeVisible();
        await expect(cases.searchPanelInput()).toBeVisible();
        await expect(cases.searchPanelFiltersButton()).toBeVisible();

        await support.returnFromList(project.id);
        await support.statCard(card.label).click();

        // Exact path and exact query: parsed rather than pattern-matched so a
        // stray extra parameter, or a different statusFilter value, fails here
        // with the offending URL in the message.
        await expect(page).toHaveURL(
          new RegExp(`/projects/${project.id}/${card.pathSegment}\\?`),
        );
        const url = new URL(page.url());
        expect(url.pathname).toBe(`/projects/${project.id}/${card.pathSegment}`);
        expect(url.search).toBe(`?${card.query}`);
        expect(url.searchParams.get("statusFilter")).toBe("active");

        // The destination must have rendered before an absence assertion means
        // anything — the URL flips before the route swaps.
        await support.waitForList();
        await expect(
          main.getByRole("heading", { name: card.title, exact: true }),
        ).toBeVisible();
        await expect(
          main.getByText(card.description, { exact: true }),
        ).toBeVisible();

        await expect(cases.searchPanelInput()).toHaveCount(0);
        await expect(cases.searchPanelFiltersButton()).toHaveCount(0);
      });

      // UI-SUP-003. The generic card test proves the card lands on the right
      // URL and heading; this proves the list it lands on is what the heading
      // claims — Closed cases, closed in the last 30 days (UTC).
      //
      // How AllCasesPage builds it (`?statusFilter=resolved`):
      // - `filters.statusIds` = [id of the case state labelled "Closed" in
      //   GET /projects/{id}/filters], written by a useEffect a render after
      //   mount;
      // - `filters.closedStartDate` / `closedEndDate` = getLast30DaysUtcRange(),
      //   i.e. now − 30 days … now, UTC, truncated to the second;
      // - `caseTypes: ["default_case"]`, and nothing user-chosen — the search
      //   panel is unmounted, and created/updated ranges are dropped while it is.
      //
      // The search response carries no closing date, so each listed case's
      // `closedOn` is read from GET /cases/{id}. Read-only throughout.
      test("the Resolved Cases card lists only cases closed in the last 30 days", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);

        const card = SUPPORT_CENTER.statCards[2];
        const { closedStatusLabel, windowDays } = SUPPORT_CENTER.resolvedCases;
        const windowMs = windowDays * 24 * 60 * 60 * 1000;
        // Local clock vs the browser's (same machine, but the request is built
        // a render or two after the click) — whole seconds, since the window
        // is truncated to the second.
        const requestClockToleranceMs = 5_000;
        // Backend timestamps vs the window the browser computed: different
        // clocks, so allow a little skew either side.
        const serverSkewToleranceMs = 2 * 60_000;

        const support = new SupportCenterPage(page);
        const cases = new CasesListPage(page);
        const main = page.getByTestId("app-main");

        // Armed before the first navigation: the filter metadata is fetched
        // once and then served from cache (5 min staleTime), so the list page
        // will not request it again.
        const filtersBody = projectFiltersResponse(page, project.id);
        await support.openViaSideNav(project.id);

        const closedState = (await filtersBody).caseStates?.find(
          (s) => s.label === closedStatusLabel,
        );
        expect(
          closedState,
          `GET /projects/${project.id}/filters should offer a "${closedStatusLabel}" case state`,
        ).toBeDefined();
        const closedId = Number(closedState!.id);

        const cardValue = await support.statCardValue(card.label);

        const searchResponse = resolvedCasesSearch(page);
        const clickedAt = Date.now();
        await support.statCard(card.label).click();
        const response = await searchResponse;
        const answeredAt = Date.now();
        expect(response.ok(), "the resolved-cases search should succeed").toBe(
          true,
        );

        // Exact path and query, parsed — as for Outstanding Cases.
        await expect(page).toHaveURL(
          new RegExp(`/projects/${project.id}/${card.pathSegment}\\?`),
        );
        const url = new URL(page.url());
        expect(url.pathname).toBe(`/projects/${project.id}/${card.pathSegment}`);
        expect(url.search).toBe(`?${card.query}`);
        expect(url.searchParams.get("statusFilter")).toBe("resolved");

        await support.waitForList();
        await expect(
          main.getByRole("heading", { name: card.title, exact: true }),
        ).toBeVisible();
        await expect(
          main.getByText(card.description, { exact: true }),
        ).toBeVisible();

        // --- The request: Closed only, last 30 days UTC, nothing else. ---
        const filters = caseSearchBodyOf(response)?.filters ?? {};
        expect(filters.statusIds, "only the Closed state").toEqual([closedId]);
        expect(filters.caseTypes).toEqual(["default_case"]);
        for (const absent of [
          "severityIds",
          "issueIds",
          "deploymentIds",
          "createdBy",
          "createdByMe",
          "searchQuery",
          "startCreatedDate",
          "endCreatedDate",
          "startUpdatedDate",
          "endUpdatedDate",
        ] as const) {
          expect(filters[absent], `request should not carry ${absent}`).toBe(
            undefined,
          );
        }

        const utcSecond = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/;
        expect(filters.closedStartDate).toMatch(utcSecond);
        expect(filters.closedEndDate).toMatch(utcSecond);
        const windowStart = Date.parse(filters.closedStartDate!);
        const windowEnd = Date.parse(filters.closedEndDate!);
        // Both ends come from one `now`, so the span is exactly 30 days.
        expect(windowEnd - windowStart).toBe(windowMs);
        // And `now` is the time of the click, not a stale or shifted value.
        expect(windowEnd).toBeGreaterThanOrEqual(
          clickedAt - requestClockToleranceMs,
        );
        expect(windowEnd).toBeLessThanOrEqual(
          answeredAt + requestClockToleranceMs,
        );

        // --- The response: every case Closed, closed inside the window. ---
        const result = (await response.json()) as CaseSearchResult;
        for (const c of result.cases) {
          expect
            .soft(c.status?.label, `case ${c.number} status label`)
            .toBe(closedStatusLabel);
          expect
            .soft(Number(c.status?.id), `case ${c.number} status id`)
            .toBe(closedId);

          const closedOn = await caseClosedOn(page, response, c.id);
          expect
            .soft(closedOn, `case ${c.number} should have a closedOn`)
            .not.toBeNull();
          if (closedOn === null) continue;
          const closedAt = parseBackendUtc(closedOn);
          expect
            .soft(closedAt, `case ${c.number} closedOn "${closedOn}" parses`)
            .not.toBeNaN();
          expect
            .soft(
              closedAt,
              `case ${c.number} closedOn ${closedOn} should be on/after ${filters.closedStartDate}`,
            )
            .toBeGreaterThanOrEqual(windowStart - serverSkewToleranceMs);
          expect
            .soft(
              closedAt,
              `case ${c.number} closedOn ${closedOn} should be on/before ${filters.closedEndDate}`,
            )
            .toBeLessThanOrEqual(windowEnd + serverSkewToleranceMs);
        }

        // --- The rendered list: exactly the response's page, each Closed. ---
        if (result.totalRecords === 0) {
          // Nothing closed in the last 30 days on this project. The request
          // shape above is still verified; the per-row checks have nothing to
          // run on, which is recorded rather than passed silently.
          test.info().annotations.push({
            type: "empty",
            description:
              `${projectType}: no cases closed in the last ${windowDays} days — ` +
              "row checks not exercised; empty state asserted instead.",
          });
          await expect(cases.emptyState()).toBeVisible();
          await expect(cases.caseCards()).toHaveCount(0);
        } else {
          await expect(cases.caseCards()).toHaveCount(result.cases.length);
          for (const c of result.cases) {
            const row = cases.caseCard(c.number);
            await expect(row, `row for ${c.number}`).toHaveCount(1);
            await expect(
              row.getByText(closedStatusLabel, { exact: true }),
              `row for ${c.number} shows ${closedStatusLabel}`,
            ).toBeVisible();
          }
        }

        const listTotal = await cases.totalCount();
        expect(listTotal, "results bar total = response totalRecords").toBe(
          result.totalRecords,
        );
        test.info().annotations.push({
          type: "counts",
          description:
            `${projectType}: card=${cardValue} listTotal=${listTotal} ` +
            `rowsChecked=${result.cases.length} window=${filters.closedStartDate}..${filters.closedEndDate}`,
        });
        // The card and the list are meant to be the same "resolved in the last
        // 30 days" figure — but they come from different backend queries (the
        // stats endpoint's resolvedCount.pastThirtyDays vs a case search), so
        // a mismatch is reported without hiding the rest of the checks.
        expect
          .soft(listTotal, "card count should equal the list total")
          .toBe(cardValue);

        await expect(cases.searchPanelInput()).toHaveCount(0);
        await expect(cases.searchPanelFiltersButton()).toHaveCount(0);
      });
    });
  }
});
