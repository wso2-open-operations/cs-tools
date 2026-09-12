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
// Searching update levels from the Updates side-menu item.
//
// ✅ READ-ONLY. The search is a POST — `/updates/levels/search` — but it reads
// rather than writes, so this is safe to run repeatedly.
//
// The four filters cascade: version needs a product, the start level needs a
// version, and the end level needs a start. Search stays disabled until all four
// are set, which is what the first test walks.
//
// Scoped to Subscription. Updates access is a per-project feature flag and Cloud
// Support does not have it (see SIDE_NAV_VISIBILITY), so the item is not even in
// its side menu.
//

import fs from "node:fs";
import { test, expect, withSession } from "../../fixtures/test";
import { UpdatesPage } from "../../pages/UpdatesPage";
import { PROJECTS, UPDATES_SEARCH_INPUT } from "../../config/testData";
import { UPDATES } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";

withSession(test);

test.describe("Update Levels", () => {
  // A shell load, a nav navigation and the filter-options query behind the
  // product list; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[UPDATES_SEARCH_INPUT.projectType];

  test(`${UPDATES_SEARCH_INPUT.projectType} — searches update levels for a product version`, async ({
    page,
  }) => {
    test.skip(
      !project.id,
      `${UPDATES_SEARCH_INPUT.projectType} needs a project id. ` +
        `Fill it in tests/e2e/config/testData.ts.`,
    );

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);

    await expect(page).toHaveURL(
      new RegExp(`/projects/${project.id}/${UPDATES.pathSegment}`),
    );
    await expect(updates.sectionTitle()).toBeVisible();

    // Nothing has been searched yet, and Search is withheld until the filters
    // are complete — the cascade below is what completes them.
    await expect(updates.idleHint()).toBeVisible();
    await expect(updates.searchButton()).toBeDisabled();

    // Version, start and end each stay disabled until the one before has a
    // value, so filling them in order is a requirement rather than a style.
    await expect(updates.versionSelect()).toBeDisabled();
    await updates.chooseOption(
      updates.productSelect(),
      UPDATES_SEARCH_INPUT.productName,
    );

    await expect(updates.startLevelSelect()).toBeDisabled();
    await updates.chooseOption(
      updates.versionSelect(),
      UPDATES_SEARCH_INPUT.productVersion,
    );

    await expect(updates.endLevelSelect()).toBeDisabled();
    await updates.chooseOption(
      updates.startLevelSelect(),
      UPDATES_SEARCH_INPUT.startLevel,
    );

    // Still incomplete without the end level.
    await expect(updates.searchButton()).toBeDisabled();
    await updates.chooseOption(
      updates.endLevelSelect(),
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expect(updates.searchButton()).toBeEnabled();

    const response = await updates.search();
    await expectSuccess(response, "search update levels");

    // The chosen range on the wire. The selects show labels; the request carries
    // the product name, version and the two levels, so this is where the choice
    // is actually observable.
    const payload = JSON.parse(response.request().postData() ?? "{}") as {
      productName?: string;
      productVersion?: string;
      startingUpdateLevel?: number | string;
      endingUpdateLevel?: number | string;
    };
    expect(payload.productName).toBe(UPDATES_SEARCH_INPUT.productName);
    expect(payload.productVersion).toBe(UPDATES_SEARCH_INPUT.productVersion);
    expect(String(payload.startingUpdateLevel)).toBe(
      UPDATES_SEARCH_INPUT.startLevel,
    );
    expect(String(payload.endingUpdateLevel)).toBe(
      UPDATES_SEARCH_INPUT.endLevel,
    );

    // And onto the URL, which the page writes so a search can be shared or
    // reloaded. Written with `replace`, so it does not add a history entry.
    const url = new URL(page.url());
    expect(url.searchParams.get(UPDATES.urlParams.product)).toBe(
      UPDATES_SEARCH_INPUT.productName,
    );
    expect(url.searchParams.get(UPDATES.urlParams.version)).toBe(
      UPDATES_SEARCH_INPUT.productVersion,
    );
    expect(url.searchParams.get(UPDATES.urlParams.startLevel)).toBe(
      UPDATES_SEARCH_INPUT.startLevel,
    );
    expect(url.searchParams.get(UPDATES.urlParams.endLevel)).toBe(
      UPDATES_SEARCH_INPUT.endLevel,
    );

    // The idle hint gives way once a search has run — whether the range holds
    // updates is environment data, so this asserts the page left its initial
    // state rather than demanding results.
    await expect(updates.idleHint()).toHaveCount(0);

    // Wait for the table to replace its skeleton: the response landing is not
    // the same as the results being rendered.
    await updates.waitForResults();

    // The range asked for holds updates, so the table is the expected outcome
    // here — but the wait above tolerates an empty one, and this skips rather
    // than fails if the environment ever returns none.
    const hasResults =
      (await updates.resultsColumnHeader(UPDATES.results.headers[0]).count()) >
      0;
    test.skip(
      !hasResults,
      `No update levels between ${UPDATES_SEARCH_INPUT.startLevel} and ` +
        `${UPDATES_SEARCH_INPUT.endLevel} for ${UPDATES_SEARCH_INPUT.productName} ` +
        `${UPDATES_SEARCH_INPUT.productVersion}, so there is no table to check.`,
    );

    // Soft, so one missing column does not hide the state of the others.
    for (const header of UPDATES.results.headers) {
      await expect.soft(updates.resultsColumnHeader(header)).toBeVisible();
    }

    console.log(
      `Update levels (${UPDATES_SEARCH_INPUT.projectType}): searched ` +
        `${UPDATES_SEARCH_INPUT.productName} ${UPDATES_SEARCH_INPUT.productVersion} ` +
        `levels ${UPDATES_SEARCH_INPUT.startLevel}–${UPDATES_SEARCH_INPUT.endLevel}`,
    );
  });

  test(`${UPDATES_SEARCH_INPUT.projectType} — clears the filters before searching`, async ({
    page,
  }) => {
    test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);

    // Nothing selected yet, so there is nothing to clear.
    await expect(updates.clearFiltersButton()).toBeDisabled();

    await updates.fillSearch(
      UPDATES_SEARCH_INPUT.productName,
      UPDATES_SEARCH_INPUT.productVersion,
      UPDATES_SEARCH_INPUT.startLevel,
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expect(updates.clearFiltersButton()).toBeEnabled();
    await expect(updates.searchButton()).toBeEnabled();

    await updates.clearFiltersButton().click();

    // The chosen values are gone. Asserted as their absence rather than against
    // the placeholder: a closed MUI select with no value renders a zero-width
    // space, not the placeholder text, so "shows Select Product" would never
    // hold.
    await expect(updates.productSelect()).not.toContainText(
      UPDATES_SEARCH_INPUT.productName,
    );
    await expect(updates.versionSelect()).not.toContainText(
      UPDATES_SEARCH_INPUT.productVersion,
    );

    // And the cascade is back to its starting state, which is the observable
    // consequence of the levels being cleared too: with no product chosen the
    // three dependent selects are disabled again, and neither action is
    // available.
    await expect(updates.versionSelect()).toBeDisabled();
    await expect(updates.startLevelSelect()).toBeDisabled();
    await expect(updates.endLevelSelect()).toBeDisabled();
    await expect(updates.searchButton()).toBeDisabled();
    await expect(updates.clearFiltersButton()).toBeDisabled();

    console.log(
      `Update levels (${UPDATES_SEARCH_INPUT.projectType}): filters cleared before searching`,
    );
  });

  test(`${UPDATES_SEARCH_INPUT.projectType} — clears a completed search`, async ({
    page,
  }) => {
    test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);

    await updates.fillSearch(
      UPDATES_SEARCH_INPUT.productName,
      UPDATES_SEARCH_INPUT.productVersion,
      UPDATES_SEARCH_INPUT.startLevel,
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expectSuccess(await updates.search(), "search update levels");

    // The search left its parameters on the URL; clearing has to take them off
    // again, which a filter-only reset would not.
    expect(
      new URL(page.url()).searchParams.get(UPDATES.urlParams.product),
    ).toBe(UPDATES_SEARCH_INPUT.productName);

    await updates.clearFiltersButton().click();

    await expect
      .poll(
        () => new URL(page.url()).searchParams.get(UPDATES.urlParams.product),
        {
          message: "clearing should drop the search from the URL",
          timeout: 15_000,
        },
      )
      .toBeNull();
    for (const param of Object.values(UPDATES.urlParams)) {
      expect(new URL(page.url()).searchParams.get(param)).toBeNull();
    }

    // The results give way to the idle hint, which is the page's own statement
    // that no search is in effect.
    await expect(updates.idleHint()).toBeVisible();
    await expect(updates.productSelect()).not.toContainText(
      UPDATES_SEARCH_INPUT.productName,
    );
    await expect(updates.clearFiltersButton()).toBeDisabled();

    console.log(
      `Update levels (${UPDATES_SEARCH_INPUT.projectType}): completed search cleared`,
    );
  });

  test(`${UPDATES_SEARCH_INPUT.projectType} — views an update level's details`, async ({
    page,
  }) => {
    test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);
    await updates.fillSearch(
      UPDATES_SEARCH_INPUT.productName,
      UPDATES_SEARCH_INPUT.productVersion,
      UPDATES_SEARCH_INPUT.startLevel,
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expectSuccess(await updates.search(), "search update levels");
    await updates.waitForResults();

    const level = UPDATES_SEARCH_INPUT.viewLevel;
    const view = updates.viewLevelButton(level);
    test.skip(
      (await view.count()) === 0,
      `Level ${level} is not among the results for ` +
        `${UPDATES_SEARCH_INPUT.productName} ${UPDATES_SEARCH_INPUT.productVersion}, ` +
        `so there is no row to open.`,
    );

    await view.first().click();

    // The level's own page, which carries the searched product forward as query
    // parameters — `productBaseVersion` there, where the search wrote `pv`.
    await expect(page).toHaveURL(
      new RegExp(
        `/projects/${project.id}/${UPDATES.levelDetails.pathSegment}/${level}\\?`,
      ),
    );

    const url = new URL(page.url());
    expect(url.searchParams.get("productName")).toBe(
      UPDATES_SEARCH_INPUT.productName,
    );
    expect(url.searchParams.get("productBaseVersion")).toBe(
      UPDATES_SEARCH_INPUT.productVersion,
    );

    // The summary names the product it is describing. Soft, so one missing field
    // does not hide the state of the others.
    for (const label of UPDATES.levelDetails.summaryLabels) {
      await expect
        .soft(updates.levelSummaryLabel(label).first())
        .toBeVisible({ timeout: 30_000 });
    }

    // Each filter is offered, and choosing it keeps the page on this level —
    // these narrow the listed updates rather than navigating.
    for (const filter of UPDATES.levelDetails.filterButtons) {
      const button = updates.levelFilterButton(filter);
      await expect.soft(button).toBeVisible();
      await button.click();

      await expect(page).toHaveURL(
        new RegExp(`/${UPDATES.levelDetails.pathSegment}/${level}\\?`),
      );

      // Under All there are updates to list; Security and Regular are subsets and
      // either may legitimately be empty, so only the count is reported for them.
      const listed = await updates.levelUpdateNumbers().count();
      if (filter === UPDATES.levelDetails.filterButtons[0]) {
        expect(
          listed,
          `the All view should list the level's updates`,
        ).toBeGreaterThan(0);

        // Every listed update shows its number, and the section that describes it
        // is on the page.
        await expect(updates.levelUpdateNumbers().first()).toBeVisible();
        await expect
          .soft(
            updates
              .levelSummaryLabel(UPDATES.levelDetails.descriptionHeading)
              .first(),
          )
          .toBeVisible();
      }

      console.log(
        `Update level ${level} (${filter}): ${listed} update(s) listed`,
      );
    }
  });

  test(`${UPDATES_SEARCH_INPUT.projectType} — goes back from an update level to the search`, async ({
    page,
  }) => {
    test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);
    await updates.fillSearch(
      UPDATES_SEARCH_INPUT.productName,
      UPDATES_SEARCH_INPUT.productVersion,
      UPDATES_SEARCH_INPUT.startLevel,
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expectSuccess(await updates.search(), "search update levels");
    await updates.waitForResults();

    const level = UPDATES_SEARCH_INPUT.viewLevel;
    const view = updates.viewLevelButton(level);
    test.skip(
      (await view.count()) === 0,
      `Level ${level} is not among the results, so there is no row to open.`,
    );

    await view.first().click();
    await expect(page).toHaveURL(
      new RegExp(`/${UPDATES.levelDetails.pathSegment}/${level}\\?`),
    );

    await updates.levelBackButton().click();

    // Back on the search, which is what the filter section's heading identifies.
    await expect(updates.sectionTitle()).toBeVisible({ timeout: 30_000 });
    await expect(page).toHaveURL(
      new RegExp(`/projects/${project.id}/${UPDATES.pathSegment}`),
    );
    await expect(page).not.toHaveURL(
      new RegExp(UPDATES.levelDetails.pathSegment),
    );

    // The control walks history back rather than routing to a fixed path, so the
    // search that was in effect comes back with it — the parameters were written
    // onto the URL, and this is where that pays off. Soft, because it is a
    // consequence of the history behaviour rather than the thing under test.
    await expect
      .soft(updates.searchButton(), "the search should still be complete")
      .toBeEnabled();

    // Asserted through `toHaveURL`, which retries: the parameters are restored as
    // the page remounts, so reading `page.url()` once can catch it before they
    // are back.
    await expect
      .soft(page, "the search parameters should be restored")
      .toHaveURL(
        new RegExp(
          `[?&]${UPDATES.urlParams.product}=${UPDATES_SEARCH_INPUT.productName}(&|$)`,
        ),
      );

    const restored = new URL(page.url()).searchParams.toString();
    console.log(
      `Update level ${level}: went back to the search ` +
        `(params ${restored ? "restored" : "dropped"})`,
    );
  });

  test(`${UPDATES_SEARCH_INPUT.projectType} — downloads the update levels report`, async ({
    page,
  }) => {
    test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);
    await updates.fillSearch(
      UPDATES_SEARCH_INPUT.productName,
      UPDATES_SEARCH_INPUT.productVersion,
      UPDATES_SEARCH_INPUT.startLevel,
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expectSuccess(await updates.search(), "search update levels");
    await updates.waitForResults();

    // The button becomes available once the search has results to report on.
    await expect(updates.downloadReportButton()).toBeEnabled();

    const download = await updates.downloadReport();

    // The file a user ends up with. The name is built from the searched product
    // and range, so it doubles as a check that the report describes the search
    // that produced it.
    expect(download.suggestedFilename()).toBe(
      UPDATES.report.fileName(
        UPDATES_SEARCH_INPUT.productName,
        UPDATES_SEARCH_INPUT.productVersion,
        UPDATES_SEARCH_INPUT.startLevel,
        UPDATES_SEARCH_INPUT.endLevel,
      ),
    );

    // And it has content: the PDF is generated in the browser, so an empty file
    // would mean jsPDF produced nothing while still triggering the download.
    const path = await download.path();
    expect(path, "the download produced no file").toBeTruthy();
    expect(
      fs.statSync(path as string).size,
      "the report should not be empty",
    ).toBeGreaterThan(0);

    console.log(
      `Update levels report: downloaded ${download.suggestedFilename()} ` +
        `(${fs.statSync(path as string).size} bytes)`,
    );
  });

  test(`${UPDATES_SEARCH_INPUT.projectType} — withholds the report until there is something to report`, async ({
    page,
  }) => {
    test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

    // Counted from the start: neither of the states below may produce a file, and
    // a disabled button alone would not show that nothing was generated.
    let downloads = 0;
    page.on("download", () => {
      downloads += 1;
    });

    const updates = new UpdatesPage(page);
    await updates.openViaSideNav(project.id);

    // Nothing searched yet, so there is no report data to build from.
    await expect(updates.downloadReportButton()).toBeDisabled();

    // Filling the filters is not enough either — the data comes from the search,
    // not the selection.
    await updates.fillSearch(
      UPDATES_SEARCH_INPUT.productName,
      UPDATES_SEARCH_INPUT.productVersion,
      UPDATES_SEARCH_INPUT.startLevel,
      UPDATES_SEARCH_INPUT.endLevel,
    );
    await expect(updates.downloadReportButton()).toBeDisabled();

    await expectSuccess(await updates.search(), "search update levels");
    await updates.waitForResults();
    await expect(updates.downloadReportButton()).toBeEnabled();

    // And clearing takes it away again, since the results go with it.
    await updates.clearFiltersButton().click();
    await expect(updates.downloadReportButton()).toBeDisabled();
    await expect(updates.idleHint()).toBeVisible();

    expect(
      downloads,
      "no report should be produced without downloading one",
    ).toBe(0);

    console.log(
      `Update levels report: withheld before searching and after clearing`,
    );
  });

  //
  // Validation. Nothing here searches successfully, and nothing is written —
  // the rules are all about which choices the form allows.
  //
  test.describe("validation", () => {
    test(`${UPDATES_SEARCH_INPUT.projectType} — offers only levels above the start as the end`, async ({
      page,
    }) => {
      test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

      const updates = new UpdatesPage(page);
      await updates.openViaSideNav(project.id);

      await updates.chooseOption(
        updates.productSelect(),
        UPDATES_SEARCH_INPUT.productName,
      );
      await updates.chooseOption(
        updates.versionSelect(),
        UPDATES_SEARCH_INPUT.productVersion,
      );

      const startOptions = await updates.optionLabels(updates.startLevelSelect());
      expect(
        startOptions.length,
        "at least two start levels are needed for one to sit above another",
      ).toBeGreaterThan(1);

      // Guarded like every other runtime discovery here: without this, a level
      // that is not on offer makes the option click time out on a missing element
      // rather than saying which fixture value is wrong.
      const start = UPDATES_SEARCH_INPUT.viewLevel;
      test.skip(
        !startOptions.includes(start),
        `Level ${start} is not offered as a start level for ` +
          `${UPDATES_SEARCH_INPUT.productName} ${UPDATES_SEARCH_INPUT.productVersion}. ` +
          `Update UPDATES_SEARCH_INPUT.viewLevel in tests/e2e/config/testData.ts.`,
      );

      await updates.chooseOption(updates.startLevelSelect(), start);

      // The end select lists only levels above the start, which is what makes an
      // inverted range unreachable: `validateAllUpdatesFilter` also rejects
      // start > end, but the UI never lets it be submitted in the first place.
      const endOptions = await updates.optionLabels(updates.endLevelSelect());
      expect(endOptions.length, "no end levels offered").toBeGreaterThan(0);

      for (const option of endOptions) {
        expect(
          Number(option),
          `end option ${option} should be above the start ${start}`,
        ).toBeGreaterThan(Number(start));
      }
      expect(endOptions).not.toContain(start);

      console.log(
        `Update levels: end offers ${endOptions.length} levels above ${start}`,
      );
    });

    test(`${UPDATES_SEARCH_INPUT.projectType} — resets the end level when the start changes`, async ({
      page,
    }) => {
      test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

      const updates = new UpdatesPage(page);
      await updates.openViaSideNav(project.id);
      await updates.fillSearch(
        UPDATES_SEARCH_INPUT.productName,
        UPDATES_SEARCH_INPUT.productVersion,
        UPDATES_SEARCH_INPUT.startLevel,
        UPDATES_SEARCH_INPUT.endLevel,
      );
      await expect(updates.searchButton()).toBeEnabled();

      // A different start invalidates the end that was chosen against the old one,
      // so the form clears it rather than leaving an inconsistent pair.
      const startOptions = await updates.optionLabels(updates.startLevelSelect());
      const nextStart = startOptions.find(
        (option) => option !== UPDATES_SEARCH_INPUT.startLevel,
      );
      test.skip(!nextStart, `only one start level is offered.`);

      await updates.chooseOption(updates.startLevelSelect(), nextStart as string);

      await expect(updates.endLevelSelect()).not.toContainText(
        UPDATES_SEARCH_INPUT.endLevel,
      );
      await expect(updates.searchButton()).toBeDisabled();

      console.log(
        `Update levels: start ${UPDATES_SEARCH_INPUT.startLevel} → ${nextStart} ` +
          `cleared the end level`,
      );
    });

    test(`${UPDATES_SEARCH_INPUT.projectType} — resets the version and levels when the product changes`, async ({
      page,
    }) => {
      test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

      const updates = new UpdatesPage(page);
      await updates.openViaSideNav(project.id);

      const products = await updates.optionLabels(updates.productSelect());
      const otherProduct = products.find(
        (option) => option !== UPDATES_SEARCH_INPUT.productName,
      );
      test.skip(
        !otherProduct,
        `only "${UPDATES_SEARCH_INPUT.productName}" is offered, so the product ` +
          `cannot be changed.`,
      );

      await updates.fillSearch(
        UPDATES_SEARCH_INPUT.productName,
        UPDATES_SEARCH_INPUT.productVersion,
        UPDATES_SEARCH_INPUT.startLevel,
        UPDATES_SEARCH_INPUT.endLevel,
      );
      await expect(updates.searchButton()).toBeEnabled();

      // Changing the product invalidates everything chosen under it — the version
      // list belongs to the product, and the levels to the version.
      await updates.chooseOption(
        updates.productSelect(),
        otherProduct as string,
      );

      await expect(updates.versionSelect()).not.toContainText(
        UPDATES_SEARCH_INPUT.productVersion,
      );

      // With no version, the two level selects are disabled again — which is the
      // observable form of their values having been cleared.
      await expect(updates.startLevelSelect()).toBeDisabled();
      await expect(updates.endLevelSelect()).toBeDisabled();
      await expect(updates.searchButton()).toBeDisabled();

      console.log(
        `Update levels: product ${UPDATES_SEARCH_INPUT.productName} → ` +
          `${otherProduct} cleared the version and levels`,
      );
    });

    test(`${UPDATES_SEARCH_INPUT.projectType} — resets the levels when the version changes`, async ({
      page,
    }) => {
      test.skip(!project.id, `${UPDATES_SEARCH_INPUT.projectType} needs a project id.`);

      const updates = new UpdatesPage(page);
      await updates.openViaSideNav(project.id);
      await updates.chooseOption(
        updates.productSelect(),
        UPDATES_SEARCH_INPUT.productName,
      );

      const versions = await updates.optionLabels(updates.versionSelect());
      const otherVersion = versions.find(
        (option) => option !== UPDATES_SEARCH_INPUT.productVersion,
      );
      test.skip(
        !otherVersion,
        `${UPDATES_SEARCH_INPUT.productName} offers only one version, so it ` +
          `cannot be changed.`,
      );

      await updates.chooseOption(
        updates.versionSelect(),
        UPDATES_SEARCH_INPUT.productVersion,
      );
      await updates.chooseOption(
        updates.startLevelSelect(),
        UPDATES_SEARCH_INPUT.startLevel,
      );
      await updates.chooseOption(
        updates.endLevelSelect(),
        UPDATES_SEARCH_INPUT.endLevel,
      );
      await expect(updates.searchButton()).toBeEnabled();

      // The levels belong to the version, so changing it clears both.
      await updates.chooseOption(
        updates.versionSelect(),
        otherVersion as string,
      );

      await expect(updates.endLevelSelect()).toBeDisabled();
      await expect(updates.searchButton()).toBeDisabled();

      console.log(
        `Update levels: version ${UPDATES_SEARCH_INPUT.productVersion} → ` +
          `${otherVersion} cleared the levels`,
      );
    });
  });
});