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

import { type Locator, type Page, expect } from "../fixtures/test";
import { PROJECT_SWITCHER } from "../utils/selectors";
import { projectPathPattern } from "../utils/ids";

/** The switcher is skeletonised until the projects query resolves, and the
 * selection then triggers a dashboard load. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the header project switcher (`ProjectSwitcher.tsx`).
 *
 * Worth having separately from the pages it sits above: the switcher decides
 * which project the header search bar queries, so a search test depends on it
 * even though it belongs to no one page.
 */
export class ProjectSwitcherPage {
  constructor(private readonly page: Page) {}

  /**
   * The header region.
   *
   * `.first()` because the stacked (narrow) layout nests a second header inside
   * the first, and an unqualified `header` locator is then strict-mode
   * ambiguous.
   */
  private header(): Locator {
    return this.page.locator("header").first();
  }

  /**
   * The switcher trigger.
   *
   * Identified by the text it shows, since it carries no role or test id. Both
   * states are accepted: the trigger reads "Select Project" until the projects
   * query resolves the current one, so matching only the project name would
   * race the load.
   *
   * @param projectName - Name of the currently selected project, if any.
   * @returns Locator for the trigger.
   */
  trigger(projectName: string): Locator {
    const header = this.header();
    return header
      .getByText(projectName, { exact: true })
      .or(header.getByText(PROJECT_SWITCHER.unselectedLabel, { exact: true }))
      .first();
  }

  /**
   * A project's row in the open dropdown.
   *
   * Matched on an exact-text child rather than the row's own text, because each
   * row also renders the project key as a caption — and one project's name can
   * be a substring of another's.
   *
   * @param projectName - Project to find.
   * @returns Locator for that row.
   */
  option(projectName: string): Locator {
    return this.page
      .getByRole("menuitem")
      .filter({ has: this.page.getByText(projectName, { exact: true }) });
  }

  /**
   * Selects a project through the switcher and waits for its dashboard.
   *
   * This is what gives the header search bar a project to query — the bar is
   * project-scoped, and searching before the selection resolves returns nothing
   * for a reason that looks like a search defect.
   *
   * Selecting the project that is ALREADY current is deliberately not
   * short-circuited: the component navigates either way, and going through the
   * same path every time means the test does not depend on where it started.
   *
   * @param projectId - Project id, used to confirm the resulting URL.
   * @param projectName - Project's display name, as shown in the dropdown.
   */
  async selectProject(projectId: string, projectName: string): Promise<void> {
    const trigger = this.trigger(projectName);
    await expect(trigger).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await trigger.click();

    // The dropdown renders skeletons while the projects page loads; those carry
    // no role, so waiting for a row waits out the load.
    const option = this.option(projectName);

    // The filter field only exists above a threshold of projects. Typing into
    // it when present narrows a paginated list that might otherwise not have
    // reached this project yet; when absent the full list is already short.
    const filter = this.page.getByPlaceholder(
      PROJECT_SWITCHER.searchPlaceholder,
    );
    if (await filter.isVisible().catch(() => false)) {
      await filter.fill(projectName);
    }

    await expect(
      option,
      `"${projectName}" should be offered by the project switcher`,
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await option.click();

    // Selection navigates to the project's dashboard — the point at which the
    // header is scoped to it.
    await expect(this.page).toHaveURL(
      projectPathPattern(projectId, "dashboard"),
      { timeout: LOAD_TIMEOUT_MS },
    );

    // And the trigger reflects it, which is the part that proves the header —
    // not just the URL — is on this project.
    await expect(
      this.header().getByText(projectName, { exact: true }),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }
}
