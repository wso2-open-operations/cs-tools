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

import { type Locator, type Page, expect } from "@playwright/test";

/**
 * Page object for the "Log time" dialog (`LogTimeCardDialog.tsx`), opened
 * from a case's Time tracking tab. `fillAndSubmit` performs a REAL create —
 * `POST /time-cards` — against the live backend; there is no delete
 * endpoint, so every call leaves a permanent record. Always pass a
 * work-log comment tagged with `E2E_TAG` (see `utils/selectors.ts`).
 */
export class LogTimeDialog {
  constructor(private readonly page: Page) {}

  root(): Locator {
    return this.page.getByRole("dialog");
  }

  titleHeading(): Locator {
    // Scoped by name: the dialog also contains several MUI Typography
    // subtitle2 elements ("Time breakdown (hours)", "Approver (team lead)",
    // the running-total line) that render as <h6> and register as
    // accessible headings too — an unscoped getByRole("heading") matches all
    // of them, not just the DialogTitle.
    return this.root().getByRole("heading", { name: /Log time/ });
  }

  async waitForOpen(): Promise<void> {
    await expect(this.titleHeading()).toBeVisible();
  }

  /** The case number this dialog was opened for, read from "Log time · <caseNumber>". */
  async caseNumber(): Promise<string | null> {
    const title = await this.titleHeading().textContent();
    return title?.match(/Log time · (\S+)/)?.[1] ?? null;
  }

  /**
   * Fill the minimum required fields and submit, waiting for the dialog to
   * close. `approverQuery` should be a string guaranteed to match a real,
   * email-having, *other* account — see `approverSearchQuery()` in
   * `fixtures/test.ts` (a generic single-letter query can match only an
   * empty-email service account in a small staging tenant, confirmed live,
   * and the signed-in user's own address never matches since the picker
   * excludes them).
   */
  async fillAndSubmit(opts: {
    hours: number;
    workLogComment: string;
    approverQuery: string;
  }): Promise<void> {
    const dialog = this.root();

    // One activity bucket is enough to satisfy "log time against at least
    // one activity" — use the first row ("Analysis and debugging").
    await dialog.getByLabel("Analysis and debugging").fill(String(opts.hours));

    await dialog.getByLabel("Work log comment").fill(opts.workLogComment);

    await dialog
      .getByPlaceholder("Search engineers by name or email…")
      .fill(opts.approverQuery);
    const firstCandidate = dialog.getByTestId("approver-candidate").first();
    await expect(firstCandidate).toBeVisible({ timeout: 10_000 });
    // dispatchEvent, not click(): this button sits in a nested scrollable
    // container (DialogContent > results Box, both overflow:auto) where
    // Playwright's pixel-coordinate click intermittently lands on the
    // sibling DialogActions bar instead (confirmed via elementFromPoint) —
    // a headless-rendering/stacking quirk, not a real click target issue
    // (a native btn.click() reaches the handler fine). dispatchEvent fires
    // the DOM event directly, without positional hit-testing.
    await firstCandidate.dispatchEvent("click");

    await dialog.getByRole("button", { name: "Submit for review" }).click();
    await expect(dialog).toBeHidden({ timeout: 15_000 });
  }
}
