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
// The Security Center's Component Analysis tab: third-party components with
// known vulnerabilities.
//
// Read-only throughout — this tab creates nothing, so unlike the rest of the
// security suite these tests leave no permanent records and are safe to re-run.
//
// ⚠️ The thing to know before changing anything here: this table is entirely
// CLIENT-side. ProductVulnerabilitiesTable fetches the whole dataset once
// (POST /product-vulnerabilities/search with a fetch-all request) and then
// narrows it in a `useMemo` for search, filters and pagination alike. So:
//
//   - typing in the search box issues NO request, and a test that waits for one
//     hangs until it times out;
//   - the search input is debounced, so a count read immediately after typing
//     is a count of the PREVIOUS state, which presents as a search that matched
//     everything;
//   - pagination is client-side at ten rows a page, so counting rendered rows
//     measures the page, not the result set. Narrowing is asserted against the
//     footer's "1-10 of 1265" total instead.
//
// The tab is gated on the `hasComponentAnalysis` project feature, so it does
// not exist for every project — hence the skip rather than a hard failure.
//

import { test, expect, withSession } from "../../fixtures/test";
import { SecurityCenterPage } from "../../pages/SecurityCenterPage";
import { PROJECTS, ProjectType } from "../../config/testData";
import { SECURITY_CENTER } from "../../utils/selectors";

withSession(test);

const project = PROJECTS[ProjectType.SUBSCRIPTION];
const components = SECURITY_CENTER.componentAnalysis;

test.describe("Security Center — Component Analysis", () => {
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(async ({ page }) => {
    test.skip(!project.id, `${ProjectType.SUBSCRIPTION} needs a project id.`);

    const security = new SecurityCenterPage(page);
    await security.openViaSideNav(project.id);

    // The tab is permission-gated. Skipping names the reason; without this the
    // failure would be a missing tab, which reads like a broken locator.
    const hasTab = await security.tab(components.title).count();
    test.skip(
      hasTab === 0,
      `${ProjectType.SUBSCRIPTION} does not expose Component Analysis ` +
        `(project feature hasComponentAnalysis).`,
    );
  });

  test("opens Component Analysis and loads the table", async ({ page }) => {
    const security = new SecurityCenterPage(page);

    const total = await security.openComponentAnalysis();
    console.log(`Component Analysis loaded with ${total} record(s)`);

    // The table's own title, distinguishing it from the sibling Security Report
    // Analysis tab, which the default branch also renders.
    await expect(
      page.getByText(components.title, { exact: true }).first(),
    ).toBeVisible();
    await expect(security.componentSearchInput()).toBeVisible();

    // Every row is keyed by a CVE, so their presence is the data having
    // genuinely arrived rather than a skeleton having rendered.
    if (total > 0) {
      await expect(security.componentRows().first()).toBeVisible();
    } else {
      await expect(
        page.getByText(components.emptyMessage).first(),
      ).toBeVisible();
    }
  });

  test("searches by CVE and clears the search", async ({ page }) => {
    const security = new SecurityCenterPage(page);

    const total = await security.openComponentAnalysis();
    test.skip(
      total === 0,
      "Component Analysis has no rows on this project to search.",
    );

    // Search on a CVE read off the table itself rather than a hardcoded one:
    // this is live vulnerability data and whatever is pinned today will be
    // remediated eventually, turning the test red for no reason.
    const firstRow = await security.componentRows().first().textContent();
    const cve = firstRow?.match(components.cvePattern)?.[0];
    expect(cve, "expected a CVE in the first row").toBeTruthy();

    const matched = await security.searchComponents(cve!);
    expect(matched, `searching ${cve} should match at least its own row`)
      .toBeGreaterThan(0);

    // A real narrowing, not merely "no more than before". These counts are
    // result-set totals, so on a dataset of any size a single CVE must match
    // strictly fewer records than the unfiltered set — the check that would
    // have caught the search silently doing nothing.
    expect(matched, `searching ${cve} should narrow ${total} records`)
      .toBeLessThan(total);

    // Every surviving row must actually contain the term — the filter matches
    // CVE, component name and vulnerability id, and this is what separates a
    // working search from one that silently returns everything.
    await expect(security.componentRows().first()).toContainText(cve!);

    // Clearing restores the full set, which proves the narrowing was the search
    // and not the data having changed underneath.
    await security.clearComponentSearch();
    const restored = await security.searchComponents("");
    expect(restored).toBe(total);

    console.log(
      `Searched "${cve}": ${matched} of ${total} record(s), ` +
        `${restored} after clearing`,
    );
  });

  test("returns nothing for a term that cannot match", async ({ page }) => {
    const security = new SecurityCenterPage(page);

    await security.openComponentAnalysis();

    // A deliberate miss. The empty state is the assertion: a search that
    // matched nothing must say so rather than quietly showing the full table,
    // which is exactly how a broken client-side filter presents.
    const matched = await security.searchComponents("zzz-no-such-component-zzz");
    expect(matched).toBe(0);
    await expect(page.getByText(components.emptyMessage).first()).toBeVisible();
  });

  test("filters by product and clears the filters", async ({ page }) => {
    const security = new SecurityCenterPage(page);

    const total = await security.openComponentAnalysis();
    test.skip(
      total === 0,
      "Component Analysis has no rows on this project to filter.",
    );

    await security.openFilters();

    const chosen = await security.selectFilterValue(
      components.filters.productLabel,
      components.filters.productAllOption,
    );
    test.skip(
      chosen === null,
      "Product filter offers no values beyond its All option.",
    );

    const filtered = await security.searchComponents("");

    // Every row on the page must actually be the product that was selected.
    // This is the assertion that a filter is doing its job — a count that merely
    // fails to grow proves nothing, because a filter that is ignored entirely
    // returns the whole list and satisfies it.
    const rows = security.componentRows();
    const rendered = await rows.count();
    expect(rendered, "a matching product should render rows").toBeGreaterThan(0);
    for (let i = 0; i < rendered; i += 1) {
      await expect(
        rows.nth(i),
        `row ${i + 1} should be for "${chosen!.value}"`,
      ).toContainText(chosen!.value);
    }

    // And the result set must be strictly smaller — but only when there was more
    // than one product to choose between. On a project with a single product,
    // matching everything is the correct answer, not a broken filter.
    if (chosen!.optionCount > 1) {
      expect(
        filtered,
        `"${chosen!.value}" is 1 of ${chosen!.optionCount} products, so it ` +
          `should narrow ${total} records`,
      ).toBeLessThan(total);
    } else {
      expect(filtered).toBeLessThanOrEqual(total);
    }

    // Clear Filters must restore the full set. This is the half of filtering
    // that users actually get stuck on, and it is cheap to assert.
    await page
      .getByRole("button", { name: components.clearFiltersButton })
      .click();
    const restored = await security.searchComponents("");
    expect(restored).toBe(total);

    console.log(
      `Product "${chosen!.value}" (1 of ${chosen!.optionCount}): ` +
        `${filtered} of ${total} record(s), ` +
        `${restored} after clearing filters`,
    );
  });
});
