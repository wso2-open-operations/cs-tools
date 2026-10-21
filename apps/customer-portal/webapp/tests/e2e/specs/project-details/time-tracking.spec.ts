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
// Time cards: Project Details → Time Tracking.
//
// The tab lists Approved time cards for the project, filtered by a date range
// that DEFAULTS to the project's start date through today.
//
// Read-only. Nothing here writes; the exports are built in the browser.
//
// Three live findings shape these tests — all verified on staging 2026-10-06:
//
//   • THE DEFAULT RANGE RETURNS NOTHING on the automation projects, while both
//     the cleared filter and 1 Jan 2025 → today return the same 3 cards. The
//     default runs from the project's `startDate`, so the cards simply predate
//     it. This is why the listing test does not demand a non-empty default:
//     entries are asserted where they actually appear, by the range test.
//   • THERE IS NO SEARCH OR APPLY BUTTON. Each picker's onChange resets to page
//     1 and refetches, so "searching" IS the refetch.
//   • The previous summary stays on screen during that refetch, so counts read
//     straight after a filter change are STALE. Every filter change here waits
//     for the search response carrying the expected filters and then for the
//     rendered total to equal that response's `totalRecords` — see
//     `applyFilterChange`.
//
// ⚠️ The tab is permission-gated on `hasTimeLogs` and is ABSENT rather than
// disabled when withheld — Cloud Support has no Time Tracking tab at all, which
// is why this runs against one project rather than all three.
//

import {
  test,
  expect,
  withSession,
  type Download,
  type TestInfo,
} from "../../fixtures/test";
import fs from "node:fs/promises";
import path from "node:path";
import { ProjectTimeTrackingPage } from "../../pages/ProjectTimeTrackingPage";
import { TIME_CARDS_INPUT, PROJECTS } from "../../config/testData";
import { TIME_TRACKING } from "../../utils/selectors";

withSession(test);

const project = PROJECTS[TIME_CARDS_INPUT.projectType];

/** Where exported files are kept after a run — beside the other artefacts, so
 * `test-results/` stays the one place to look. */
const EXPORT_DIR = path.join("test-results", "exports");

/**
 * Saves a download and attaches it to the HTML report.
 *
 * Playwright deletes downloads when the context closes, so a file only read
 * during the test leaves nothing behind. Saving makes it inspectable, and
 * attaching puts it in the report CI uploads.
 *
 * @param testInfo - The running test, for the attachment.
 * @param download - The download to keep.
 * @returns Absolute path to the saved file.
 */
async function keepExport(
  testInfo: TestInfo,
  download: Download,
): Promise<string> {
  const target = path.resolve(EXPORT_DIR, download.suggestedFilename());
  await fs.mkdir(path.dirname(target), { recursive: true });
  await download.saveAs(target);
  await testInfo.attach(download.suggestedFilename(), { path: target });
  return target;
}

test.describe("Time Tracking", () => {
  // A shell load, the project-details page, the time cards query, and for the
  // export tests a file build over the full result set.
  test.describe.configure({ timeout: 180_000 });

  test.beforeEach(async ({ page }) => {
    test.skip(
      !project.id,
      `${TIME_CARDS_INPUT.projectType} needs a project id. ` +
        `Fill it in tests/e2e/config/testData.ts.`,
    );

    const timeTracking = new ProjectTimeTrackingPage(page);
    await timeTracking.openTimeTrackingTab(project.id);
  });

  test("lists time cards for the default range", async ({ page }) => {
    const timeTracking = new ProjectTimeTrackingPage(page);

    const { shown, total } = await timeTracking.counts();

    // The summary and the DOM must agree. They come from different places — the
    // line is rendered from the response's `totalRecords`, the cards from its
    // array — so a mismatch means the list dropped or duplicated entries.
    //
    // No assertion that the default range is non-empty: it is 0 on the
    // automation projects, because the default starts at the project's start
    // date and the cards predate it. Entries are asserted by the date-range
    // test below, which is where they actually appear.
    await expect(timeTracking.entries()).toHaveCount(shown);
    expect(shown, "the page cannot show more cards than exist").toBeLessThanOrEqual(
      total,
    );

    console.log(
      `Time Tracking: default range shows ${shown} of ${total} time cards`,
    );
  });

  test("clears the date filter", async ({ page }) => {
    const timeTracking = new ProjectTimeTrackingPage(page);

    // The range arrives pre-populated (project start date → today), so there is
    // always a Clear button to act on.
    const before = await timeTracking.counts();
    await expect(
      timeTracking.clearButton(),
      "the default date range should leave a Clear button to act on",
    ).toBeVisible({ timeout: 60_000 });

    // Resolves only once the no-dates request has come back AND the DOM shows
    // its total, so the counts below are the cleared list's, not the previous
    // one's still on screen.
    const after = await timeTracking.clearDateRange();

    // Gone — the component's own signal that no range is set, since the button
    // renders only while one is.
    await expect(
      timeTracking.clearButton(),
      "clearing should remove the Clear button",
    ).toHaveCount(0);

    // Whatever came back is rendered.
    await expect(timeTracking.entries()).toHaveCount(after.shown);

    // Dropping a filter cannot narrow the result: the unfiltered set contains
    // every range's. Live this is a real widening — the default range returns
    // nothing while the cleared list returns the project's cards — so the
    // bound is doing work rather than comparing two equal numbers.
    expect(
      after.total,
      "clearing the date filter must not return fewer cards than the " +
        "default range did",
    ).toBeGreaterThanOrEqual(before.total);

    console.log(
      `Time Tracking: cleared filter — ${before.total} → ${after.total} ` +
        `total time cards`,
    );
  });

  test("filters from the range start to today", async ({ page }) => {
    const timeTracking = new ProjectTimeTrackingPage(page);

    const start = new Date(TIME_CARDS_INPUT.rangeStart);
    const end = new Date();
    const filtered = await timeTracking.setDateRange(start, end);

    // The range stuck: both pickers accepted their values rather than one being
    // rejected by the other's min/max constraint.
    await expect(
      timeTracking.clearButton(),
      "a range should be applied",
    ).toBeVisible({ timeout: 60_000 });

    // This range is where the project's time cards actually fall, so it is the
    // one place entries can be asserted rather than merely counted.
    expect(
      filtered.total,
      `${TIME_CARDS_INPUT.rangeStart} → today should return time cards on ` +
        `${TIME_CARDS_INPUT.projectType}`,
    ).toBeGreaterThan(0);

    // And they are rendered, not just counted — the part a regression between
    // the refetch and the list would break.
    await expect(timeTracking.entries()).toHaveCount(filtered.shown);

    console.log(
      `Time Tracking: ${TIME_CARDS_INPUT.rangeStart} → today returned ` +
        `${filtered.shown} of ${filtered.total} time cards`,
    );
  });

  test("exports time cards to CSV", async ({ page }, testInfo) => {
    const timeTracking = new ProjectTimeTrackingPage(page);

    // Filter to the range that has cards first: Export is disabled while the
    // result set is empty, and the default range is empty here.
    const { total } = await timeTracking.setDateRange(
      new Date(TIME_CARDS_INPUT.rangeStart),
      new Date(),
    );
    expect(total, "there should be time cards to export").toBeGreaterThan(0);

    await timeTracking.openExportMenu();
    const download = await timeTracking.exportAs(TIME_TRACKING.export.csvItem);

    // The filename is built by the app (buildFilename), so it proves the CSV
    // path ran rather than the PDF one.
    expect(download.suggestedFilename()).toMatch(
      TIME_TRACKING.export.filenamePattern("csv"),
    );

    const saved = await keepExport(testInfo, download);
    const csv = await fs.readFile(saved, "utf8");
    expect(csv.length, "the CSV should not be empty").toBeGreaterThan(0);

    // Header plus at least one data row. A file holding only its header is a
    // failure the size check alone would pass.
    const lines = csv.split(/\r?\n/).filter((line) => line.trim().length > 0);
    expect(
      lines.length,
      "the CSV should carry data rows, not just a header",
    ).toBeGreaterThan(1);

    console.log(
      `Time Tracking CSV: ${download.suggestedFilename()} ` +
        `(${csv.length} bytes, ${lines.length - 1} rows) ` +
        `→ ${path.relative(process.cwd(), saved)}`,
    );
  });

  test("exports time cards to PDF", async ({ page }, testInfo) => {
    const timeTracking = new ProjectTimeTrackingPage(page);

    // As above — the export control is disabled on an empty result set.
    const { total } = await timeTracking.setDateRange(
      new Date(TIME_CARDS_INPUT.rangeStart),
      new Date(),
    );
    expect(total, "there should be time cards to export").toBeGreaterThan(0);

    await timeTracking.openExportMenu();
    const download = await timeTracking.exportAs(TIME_TRACKING.export.pdfItem);

    expect(download.suggestedFilename()).toMatch(
      TIME_TRACKING.export.filenamePattern("pdf"),
    );

    const saved = await keepExport(testInfo, download);

    // A PDF's text is compressed, so its contents are not asserted the way the
    // CSV's are. The magic number distinguishes a real PDF from an error page
    // or a zero-byte file, which is the failure this can meaningfully catch.
    const bytes = await fs.readFile(saved);
    expect(bytes.length, "the PDF should not be empty").toBeGreaterThan(0);
    expect(
      bytes.subarray(0, 5).toString("latin1"),
      "the file should be a PDF",
    ).toBe("%PDF-");

    console.log(
      `Time Tracking PDF: ${download.suggestedFilename()} ` +
        `(${bytes.length} bytes) → ${path.relative(process.cwd(), saved)}`,
    );
  });
});
