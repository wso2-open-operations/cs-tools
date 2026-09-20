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
// The Engagements side-menu item: its three stat cards, the list and its
// controls, and an engagement's own tabs.
//
// An engagement is a case underneath — the detail route renders CaseDetailsPage —
// so its rows carry a "CS" number and its Activity and Attachments tabs are the
// case ones. The Details tab is where they differ: the sections read "Engagement
// Overview" and "Customer Information", with an "Engagement ID" where a case
// shows "Case ID".
//
// ⚠️ Two tests WRITE, and neither record can be removed: a comment cannot be
// deleted, and the attachment is deliberately kept so the listing assertion has
// something to read. Both are guarded on their own content, so repeated runs add
// nothing.
//
// Everything else is read-only. Choosing a stat card filters the list in place —
// the URL does not change — so the list heading is the only evidence the filter
// applied.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { CaseAttachmentsPage } from "../../pages/CaseAttachmentsPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { EngagementsPage } from "../../pages/EngagementsPage";
import {
  ATTACHMENT_FILES,
  ENGAGEMENT_INPUT,
  PROJECTS,
} from "../../config/testData";
import {
  CASE_ATTACHMENTS,
  CASE_CALLS,
  ENGAGEMENTS,
} from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import { RECORD_ID_PATTERN, projectPathPattern } from "../../utils/ids";

withSession(test);

/** How long to allow for a route swap behind a URL change. */
const LOAD_TIMEOUT_MS = 30_000;

test.describe("Engagements", () => {
  // A shell load, a nav navigation and the engagements query behind the list.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ENGAGEMENT_INPUT.projectType];

  /**
   * Opens the first engagement in the list and returns its case number.
   *
   * Shared by the detail tests so each can run alone: they act on whichever
   * engagement the project lists first rather than on a sysid from config, which
   * would go stale.
   *
   * @param page - Test page.
   * @returns The engagement's number, or null when the project lists none.
   */
  async function openFirstEngagement(page: Page): Promise<string | null> {
    const engagements = new EngagementsPage(page);
    await engagements.openViaSideNav(project.id);
    await engagements.waitForList();

    if ((await engagements.rows().count()) === 0) return null;

    const number = await engagements.rowNumber(0);
    await engagements.rows().first().click();

    await expect(page).toHaveURL(
      projectPathPattern(
        project.id,
        `${ENGAGEMENTS.pathSegment}/${RECORD_ID_PATTERN}$`,
      ),
    );

    // The URL changes before the route swaps, and until it does the listing is
    // still mounted — its rows carry a CS number each, so the detail header's
    // number resolves to several elements and the assertion dies on strict mode
    // instead of retrying. Waiting for a detail-only control is what makes the
    // caller's first read land on the engagement rather than the list.
    await expect(
      page.getByRole("tab", {
        name: CASE_CALLS.alwaysPresentTab,
        exact: true,
      }),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(engagements.rows()).toHaveCount(0);

    return number;
  }

  test("shows the three stat cards", async ({ page }) => {
    test.skip(
      !project.id,
      `${ENGAGEMENT_INPUT.projectType} needs a project id. ` +
        `Fill it in tests/e2e/config/testData.ts.`,
    );

    const engagements = new EngagementsPage(page);
    await engagements.openViaSideNav(project.id);

    // Soft, so one missing card does not hide the state of the others.
    for (const card of ENGAGEMENTS.statCards) {
      await expect.soft(engagements.statCard(card.label)).toBeVisible();
    }

    // And no list heading yet: the page renders either the cards or a filtered
    // title, never both, and nothing is filtered on load. Asserting the titles
    // are absent is what pins that — an "All Engagements" heading does not exist
    // here, which is what this test originally got wrong.
    for (const card of ENGAGEMENTS.statCards) {
      await expect.soft(engagements.heading(card.title)).toHaveCount(0);
    }

    // And no Back control: it renders only once a card has been chosen or the
    // page was opened with a `returnTo`, so its absence here is part of the
    // contract rather than an accident of load order.
    await expect(engagements.backButton()).toHaveCount(0);
  });

  for (const card of ENGAGEMENTS.statCards) {
    test(`filters the list from the ${card.label} card`, async ({ page }) => {
      test.skip(!project.id, `${ENGAGEMENT_INPUT.projectType} needs a project id.`);

      const engagements = new EngagementsPage(page);
      await engagements.openViaSideNav(project.id);
      await engagements.waitForList();

      await engagements.statCardButton(card.label).first().click();

      // The heading and subtitle are the only evidence the filter applied — the
      // card narrows the list in place and leaves the URL alone.
      await expect(engagements.heading(card.title)).toBeVisible({
        timeout: 30_000,
      });
      await expect(
        page.getByText(card.subtitle, { exact: true }),
      ).toBeVisible();

      await engagements.waitForList();
      const rows = await engagements.rows().count();

      // The cards are gone while filtered — the page shows one or the other —
      // and Back is what brings them back. It does not navigate: it clears the
      // stat filter, so the URL is the same before and after.
      await expect(engagements.statCard(card.label)).toHaveCount(0);

      const filteredUrl = page.url();
      await engagements.backButton().click();

      await expect(engagements.statCard(card.label)).toBeVisible({
        timeout: 30_000,
      });
      await expect(engagements.heading(card.title)).toHaveCount(0);
      await expect(engagements.backButton()).toHaveCount(0);
      expect(page.url(), "clearing the filter should not navigate").toBe(
        filteredUrl,
      );

      // The full list is back, so the filter was cleared rather than merely
      // hidden.
      await engagements.waitForList();
      const unfiltered = await engagements.rows().count();
      expect(unfiltered).toBeGreaterThanOrEqual(rows);

      console.log(
        `Engagements (${card.label}): ${rows} listed, ${unfiltered} after Back`,
      );
    });
  }

  test("sorts the list by field and order", async ({ page }) => {
    test.skip(!project.id, `${ENGAGEMENT_INPUT.projectType} needs a project id.`);

    const engagements = new EngagementsPage(page);
    await engagements.openViaSideNav(project.id);
    await engagements.waitForList();

    const { fields, orders } = ENGAGEMENTS.sort;

    // Each field is on offer and can be chosen; the list stays on the page, so
    // the control reporting the choice back is what confirms it took.
    for (const field of Object.values(fields)) {
      await engagements.chooseSortOption(
        engagements.sortFieldSelect(),
        field.label,
      );
      await expect(engagements.sortFieldSelect()).toContainText(field.label);
      await engagements.waitForList();
    }

    // Back to a chronological field before touching the order: the order labels
    // are worded for the field, and Status is ordinal.
    await engagements.chooseSortOption(
      engagements.sortFieldSelect(),
      fields.updatedOn.label,
    );
    await engagements.chooseSortOption(
      engagements.sortOrderSelect(),
      orders.oldestFirst.label,
    );
    await expect(engagements.sortOrderSelect()).toContainText(
      orders.oldestFirst.label,
    );
    await engagements.waitForList();

    console.log(`Engagements: sorted by each field and reversed the order`);
  });

  test("searches the list, then filters and clears", async ({ page }) => {
    test.skip(!project.id, `${ENGAGEMENT_INPUT.projectType} needs a project id.`);

    const engagements = new EngagementsPage(page);
    await engagements.openViaSideNav(project.id);
    await engagements.waitForList();

    const before = await engagements.rows().count();
    test.skip(before === 0, `${ENGAGEMENT_INPUT.projectType} lists no engagements.`);

    // Search for an engagement that is listed, so the term is known to match.
    const number = await engagements.rowNumber(0);
    await engagements.searchInput().fill(number as string);

    await expect
      .poll(() => engagements.rows().count(), { timeout: 30_000 })
      .toBeLessThanOrEqual(before);
    await expect(engagements.rows().first()).toBeVisible();

    // A term nobody matches empties the list, which is what shows the search is
    // applied rather than the row surviving by coincidence.
    await engagements.searchInput().fill("no-such-engagement-zzz");
    await expect(engagements.rows()).toHaveCount(0);

    // Searching counts as a refinement, so the control offers to clear it.
    const clear = engagements.clearFiltersButton(1);
    await expect(clear).toBeVisible();
    await clear.click();

    await expect(engagements.filtersButton()).toBeVisible();
    await expect
      .poll(() => engagements.rows().count(), { timeout: 30_000 })
      .toBe(before);

    console.log(
      `Engagements: search narrowed ${before} rows and clearing restored them`,
    );
  });

  test("opens an engagement and shows its details", async ({ page }) => {
    test.skip(!project.id, `${ENGAGEMENT_INPUT.projectType} needs a project id.`);

    const number = await openFirstEngagement(page);
    test.skip(
      number === null,
      `${ENGAGEMENT_INPUT.projectType} lists no engagements to open.`,
    );

    const engagements = new EngagementsPage(page);

    // The engagement that was clicked, not merely some engagement.
    const detail = new CaseDetailPage(page);
    await expect(detail.caseNumber()).toHaveText(number as string, {
      timeout: 30_000,
    });

    // The engagement wording, not the case wording — the shared opener waits for
    // the first section by name, and "Case Overview" never appears here.
    await detail.openDetailsTab(ENGAGEMENTS.details.overviewSection);

    // Both sections, with every field the engagement view is meant to carry.
    // Soft throughout, so one missing label reports alongside the rest.
    for (const section of [
      ENGAGEMENTS.details.overviewSection,
      ENGAGEMENTS.details.customerSection,
    ]) {
      await expect
        .soft(page.getByText(section, { exact: true }).first())
        .toBeVisible();
    }

    for (const label of [
      ...ENGAGEMENTS.details.overviewLabels,
      ...ENGAGEMENTS.details.customerLabels,
    ]) {
      await expect
        .soft(page.getByText(label, { exact: true }).first(), label)
        .toBeVisible();
    }

    // Back returns to the list, where the engagement that was open is listed
    // again — the detail page routes to `/engagements` when it has no `returnTo`.
    await engagements.backButton().click();

    await expect(page).toHaveURL(
      new RegExp(`/projects/${project.id}/${ENGAGEMENTS.pathSegment}$`),
    );
    await engagements.waitForList();
    await expect(engagements.rows().first()).toBeVisible();

    console.log(`Engagement ${number}: details shown, then back to the list`);
  });

  test("comments on an engagement", async ({ page }) => {
    test.skip(!project.id, `${ENGAGEMENT_INPUT.projectType} needs a project id.`);

    const number = await openFirstEngagement(page);
    test.skip(
      number === null,
      `${ENGAGEMENT_INPUT.projectType} lists no engagements to comment on.`,
    );

    const detail = new CaseDetailPage(page);
    const text = ENGAGEMENT_INPUT.comment;

    // Post only when absent: comments cannot be deleted, so an unguarded post
    // would add one on every run.
    const alreadyPosted = (await detail.comment(text).count()) > 0;

    if (!alreadyPosted) {
      await expectSuccess(
        await detail.addComment(text),
        "add comment to engagement",
      );
    }

    await expect(detail.comment(text)).toBeVisible({ timeout: 30_000 });

    console.log(
      `Engagement ${number}: comment ${alreadyPosted ? "present" : "added"}`,
    );
  });

  test("attaches a file to an engagement", async ({ page }) => {
    test.skip(!project.id, `${ENGAGEMENT_INPUT.projectType} needs a project id.`);

    const number = await openFirstEngagement(page);
    test.skip(
      number === null,
      `${ENGAGEMENT_INPUT.projectType} lists no engagements to attach to.`,
    );

    const attachments = new CaseAttachmentsPage(page);
    await attachments.openTab();

    const kept = ATTACHMENT_FILES.kept;

    // Upload only when absent, and leave it: the listing assertions need a file
    // there, and nothing removes it between runs.
    const alreadyAttached = (await attachments.attachment(kept.name).count()) > 0;

    if (!alreadyAttached) {
      await expectSuccess(
        await attachments.upload(kept.path),
        "upload attachment to engagement",
      );
    }

    await expect(attachments.attachment(kept.name)).not.toHaveCount(0);

    const row = attachments.attachmentRow(kept.name);
    await expect(row).toContainText(kept.name);
    await expect(row).toContainText(kept.size);
    await expect
      .soft(row, "uploader")
      .toContainText(CASE_ATTACHMENTS.row.uploadedByPrefix);

    console.log(
      `Engagement ${number}: ${kept.name} ` +
        `${alreadyAttached ? "present" : "uploaded"}`,
    );
  });
});
