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
//
// Security Center — the Security Report Analysis tab: its section, search,
// status filter and sorting.
//
// ✅ READ-ONLY. Only searches, filters and sorts; nothing is submitted.
//
// Assertions are made on the list's own search request (POST
// /projects/{id}/cases/search) as well as the rows, so a control that changed
// the UI but not the query — or the reverse — fails.
//
import {
  test,
  expect,
  withSession,
  type Locator,
  type Page,
  type Request,
} from "../../fixtures/test";
import { CasesListPage } from "../../pages/CasesListPage";
import { SecurityCenterPage } from "../../pages/SecurityCenterPage";
import { PROJECTS, SRA_VIEWS } from "../../config/testData";
import { SECURITY_CENTER } from "../../utils/selectors";

withSession(test);

const LOAD_TIMEOUT_MS = 60_000;
const SRA = SECURITY_CENTER.reportAnalysis;

interface SraSearchBody {
  filters?: {
    caseTypes?: string[];
    searchQuery?: string;
    statusIds?: number[];
  };
  sortBy?: { field?: string; order?: string };
}

/**
 * Waits for the report list's next search request that satisfies a check.
 *
 * @param page - The page under test.
 * @param check - Predicate on the parsed request body.
 * @returns The matching request body.
 */
async function nextSraSearch(
  page: Page,
  check: (body: SraSearchBody) => boolean,
): Promise<SraSearchBody> {
  const request: Request = await page.waitForRequest(
    (r) => {
      if (r.method() !== "POST" || !/\/cases\/search$/.test(r.url())) {
        return false;
      }
      const body = r.postDataJSON() as SraSearchBody | null;
      return (
        !!body?.filters?.caseTypes?.includes(SRA.caseType) && check(body)
      );
    },
    { timeout: LOAD_TIMEOUT_MS },
  );
  return request.postDataJSON() as SraSearchBody;
}

/**
 * The CS numbers of the rendered report rows, in order.
 *
 * @param rows - The report cards.
 * @returns Each row's CS number.
 */
async function caseNumbers(rows: Locator): Promise<string[]> {
  return (await rows.allInnerTexts()).map((t) => /CS\d+/.exec(t)?.[0] ?? "");
}

test.describe("Security Report Analysis", () => {
  test.describe.configure({ timeout: 180_000 });

  for (const { projectType } of SRA_VIEWS) {
    const project = PROJECTS[projectType];

    test.describe(projectType, () => {
      /** Opens Security Center on the Security Report Analysis tab with its
       * list loaded. */
      async function openSra(page: Page) {
        const security = new SecurityCenterPage(page);
        await security.openViaSideNav(project.id);
        const tab = security.tab(SECURITY_CENTER.tabs.securityReportAnalysis);
        await tab.click();
        await expect(tab).toHaveAttribute("aria-selected", "true");
        const main = page.getByTestId("app-main");
        const rows = new CasesListPage(page).caseCards();
        const empty = main.getByText(SECURITY_CENTER.emptyMessage, {
          exact: true,
        });
        await expect(rows.first().or(empty)).toBeVisible({
          timeout: LOAD_TIMEOUT_MS,
        });
        return { security, main, rows, empty };
      }

      test("shows the Security Report Analysis tab and section", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);
        const { main, rows } = await openSra(page);

        await expect(main.getByText(SRA.title, { exact: true }).first()).toBeVisible();
        await expect(main.getByText(SRA.subtitle, { exact: true })).toBeVisible();
        for (const label of SRA.viewTabs) {
          await expect(main.getByRole("tab", { name: label, exact: true })).toBeVisible();
        }
        await expect(main.getByPlaceholder(SECURITY_CENTER.searchPlaceholder)).toBeVisible();
        await expect(main.getByRole("button", { name: SRA.filtersButton })).toBeVisible();
        await expect(main.locator(`#${SRA.sort.fieldSelectId}`)).toBeVisible();
        await expect(main.locator(`#${SRA.sort.orderSelectId}`)).toBeVisible();
        console.log(`SRA (${projectType}): ${await rows.count()} row(s) on page 1`);
      });

      test("searches reports by case number and clears the search", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);
        const { main, rows, empty } = await openSra(page);
        test.skip(await empty.isVisible(), `${projectType} has no reports to search.`);

        const before = await rows.count();
        const caseNumber = /CS\d+/.exec(await rows.first().innerText())?.[0];
        expect(caseNumber, "the first row should show a CS number").toBeTruthy();

        const search = main.getByPlaceholder(SECURITY_CENTER.searchPlaceholder);
        const searched = nextSraSearch(
          page,
          (b) => b.filters?.searchQuery === caseNumber,
        );
        await search.fill(caseNumber!);
        await searched;
        await expect(rows.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
        await expect
          .poll(async () => {
            const texts = await rows.allInnerTexts();
            return texts.length > 0 && texts.every((t) => t.includes(caseNumber!));
          }, { timeout: LOAD_TIMEOUT_MS })
          .toBe(true);

        const cleared = nextSraSearch(page, (b) => !b.filters?.searchQuery);
        await search.fill("");
        await cleared;
        await expect.poll(() => rows.count(), { timeout: LOAD_TIMEOUT_MS }).toBe(before);
        console.log(`SRA (${projectType}): searched ${caseNumber}, then cleared`);
      });

      test("filters reports by status and clears the filter", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);
        const { main, rows } = await openSra(page);

        await main.getByRole("button", { name: SRA.filtersButton }).click();
        const statusSelect = main.locator(`#${SRA.statusFilterId}`);
        await expect(statusSelect).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
        await statusSelect.click();
        const options = page.getByRole("option");
        await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
        // Prefer a status a listed report actually has, so the filtered list
        // is not empty by construction.
        const labels = (await options.allInnerTexts()).map((l) => l.trim()).filter(Boolean);
        const rowTexts = await rows.allInnerTexts();
        const status =
          labels.find((l) => rowTexts.some((t) => new RegExp(`\\b${l}\\b`).test(t))) ??
          labels[0];

        const filtered = nextSraSearch(page, (b) => b.filters?.statusIds?.length === 1);
        await page.getByRole("option", { name: status, exact: true }).click();
        await page.keyboard.press("Escape");
        const body = await filtered;
        expect(body.filters?.statusIds).toHaveLength(1);

        const empty = main.getByText(SECURITY_CENTER.emptyMessage, { exact: true });
        await expect(rows.first().or(empty)).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
        await expect
          .poll(async () => {
            const texts = await rows.allInnerTexts();
            return texts.every((t) => t.includes(status));
          }, { timeout: LOAD_TIMEOUT_MS, message: `every row should be ${status}` })
          .toBe(true);
        const shown = await rows.count();

        const cleared = nextSraSearch(page, (b) => !b.filters?.statusIds?.length);
        await main.getByRole("button", { name: SRA.clearFiltersButton }).click();
        await cleared;
        console.log(`SRA (${projectType}): status "${status}" → ${shown} row(s), then cleared`);
      });

      test("sorts reports by each field and both orders", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);
        const { main } = await openSra(page);
        const fieldSelect = main.locator(`#${SRA.sort.fieldSelectId}`);
        const orderSelect = main.locator(`#${SRA.sort.orderSelectId}`);

        // Defaults: Updated date, newest first.
        await expect(fieldSelect).toContainText(SRA.sort.fields[0].label);
        await expect(orderSelect).toContainText(SRA.sort.orders.chronological.desc);

        for (const field of SRA.sort.fields) {
          const orders = field.ordinal ? SRA.sort.orders.ordinal : SRA.sort.orders.chronological;
          if (!(await fieldSelect.innerText()).includes(field.label)) {
            const sorted = nextSraSearch(page, (b) => b.sortBy?.field === field.value);
            await fieldSelect.click();
            await page.getByRole("option", { name: field.label, exact: true }).click();
            await sorted;
          }
          await expect(fieldSelect).toContainText(field.label);

          for (const [order, label] of [["asc", orders.asc], ["desc", orders.desc]] as const) {
            const ordered = nextSraSearch(
              page,
              (b) => b.sortBy?.field === field.value && b.sortBy?.order?.toLowerCase() === order,
            );
            await orderSelect.click();
            await page.getByRole("option", { name: label, exact: true }).click();
            await ordered;
            await expect(orderSelect).toContainText(label);
          }
        }
        console.log(`SRA (${projectType}): sorted by every field in both orders`);
      });

      test("pages through the report list", async ({ page }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);
        const { main, rows } = await openSra(page);
        const range = main.getByText(SRA.displayedRowsPattern);
        await expect(range).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
        const total = Number(SRA.displayedRowsPattern.exec(await range.innerText())![3]);
        test.skip(
          total <= SRA.pageSize,
          `${projectType} has ${total} report(s), one page only.`,
        );
        const pages = Math.ceil(total / SRA.pageSize);
        const next = main.getByRole("button", { name: SRA.nextPageButton });
        const previous = main.getByRole("button", { name: SRA.previousPageButton });

        // Page 1.
        await expect(previous).toBeDisabled();
        await expect(next).toBeEnabled();
        await expect(range).toHaveText(new RegExp(`^1[–-]${SRA.pageSize} of ${total}$`));
        await expect(rows).toHaveCount(SRA.pageSize);
        const firstPage = await caseNumbers(rows);

        // Page 2: the next slice, disjoint from page 1.
        await next.click();
        const end2 = Math.min(2 * SRA.pageSize, total);
        await expect(range).toHaveText(
          new RegExp(`^${SRA.pageSize + 1}[–-]${end2} of ${total}$`),
        );
        await expect(rows).toHaveCount(end2 - SRA.pageSize);
        const secondPage = await caseNumbers(rows);
        expect(secondPage.filter((n) => firstPage.includes(n))).toEqual([]);
        await expect(previous).toBeEnabled();

        // Last page: Next disables.
        for (let p = 2; p < pages; p++) await next.click();
        await expect(range).toHaveText(
          new RegExp(`^${(pages - 1) * SRA.pageSize + 1}[–-]${total} of ${total}$`),
        );
        await expect(next).toBeDisabled();
        await expect(rows).toHaveCount(total - (pages - 1) * SRA.pageSize);

        // Back to page 1: the same reports as before.
        for (let p = pages; p > 1; p--) await previous.click();
        await expect(previous).toBeDisabled();
        expect(await caseNumbers(rows)).toEqual(firstPage);
        console.log(`SRA (${projectType}): ${total} reports over ${pages} pages`);
      });

      test("orders reports by created date, newest and oldest first", async ({
        page,
      }) => {
        test.skip(!project.id, `${projectType} needs a project id.`);
        const { main, rows, empty } = await openSra(page);
        test.skip(await empty.isVisible(), `${projectType} has no reports to order.`);
        const fieldSelect = main.locator(`#${SRA.sort.fieldSelectId}`);
        const orderSelect = main.locator(`#${SRA.sort.orderSelectId}`);
        const created = SRA.sort.fields.find((f) => f.value === "createdOn")!;
        const { desc, asc } = SRA.sort.orders.chronological;

        const byCreated = nextSraSearch(page, (b) => b.sortBy?.field === created.value);
        await fieldSelect.click();
        await page.getByRole("option", { name: created.label, exact: true }).click();
        await byCreated;

        // CS numbers are issued in creation order, so the rows' numbers are a
        // proxy for created date that the list itself displays.
        const numbersAfter = async (label: string, order: string) => {
          if (!(await orderSelect.innerText()).includes(label)) {
            // Armed only when a search will follow: left un-awaited, it would
            // time out and reject unhandled.
            const sorted = nextSraSearch(
              page,
              (b) =>
                b.sortBy?.field === created.value &&
                b.sortBy?.order?.toLowerCase() === order,
            );
            await orderSelect.click();
            await page.getByRole("option", { name: label, exact: true }).click();
            await sorted;
          }
          await expect(orderSelect).toContainText(label);
          let numbers: number[] = [];
          await expect
            .poll(async () => {
              numbers = (await caseNumbers(rows)).map((n) => Number(n.slice(2)));
              const sortedCopy = [...numbers].sort((a, b) =>
                order === "desc" ? b - a : a - b,
              );
              return numbers.length > 0 && numbers.join() === sortedCopy.join();
            }, { timeout: LOAD_TIMEOUT_MS, message: `rows should be ${label}` })
            .toBe(true);
          return numbers;
        };

        const newest = await numbersAfter(desc, "desc");
        const oldest = await numbersAfter(asc, "asc");
        expect(newest[0]).toBeGreaterThanOrEqual(oldest[0]);
        expect(newest[0]).toBeGreaterThanOrEqual(oldest[oldest.length - 1]);
        console.log(
          `SRA (${projectType}): newest first CS${newest[0]}, oldest first CS${oldest[0]}`,
        );
      });
    });
  }
});
