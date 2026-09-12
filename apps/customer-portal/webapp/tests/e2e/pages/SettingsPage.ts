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

import {
  type Locator,
  type Page,
  type Response,
  expect,
} from "../fixtures/test";
import { CASE_DETAIL, SETTINGS } from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";

/** How long to allow for the page and its queries to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the settings page and its four tabs.
 */
export class SettingsPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens settings through the side nav's footer item, as a user would.
   *
   * @param projectId - Project whose settings to open.
   */
  async openViaSideNav(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);
    await sideNav.clickItem(
      SETTINGS.navItem,
      new RegExp(`/projects/${projectId}/${SETTINGS.pathSegment}`),
    );
    await expect(this.tab(SETTINGS.tabs.userManagement)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * A settings tab.
   *
   * @param label - The tab's label.
   */
  tab(label: string): Locator {
    return this.main().getByRole("tab", { name: label, exact: true });
  }

  /**
   * Switches tab and waits for it to be selected.
   *
   * @param label - The tab to open.
   */
  async openTab(label: string): Promise<void> {
    await this.tab(label).click();
    await expect(this.tab(label)).toHaveAttribute("aria-selected", "true", {
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  //
  // User Management tab.
  //

  /** A column header of the user list. */
  columnHeader(label: string): Locator {
    return this.main().getByRole("columnheader", { name: label, exact: true });
  }

  searchInput(): Locator {
    return this.main().getByPlaceholder(
      SETTINGS.userManagement.searchPlaceholder,
    );
  }

  /** The Add User button, which only renders for an admin. */
  addUserButton(): Locator {
    return this.main().getByRole("button", {
      name: SETTINGS.userManagement.addUserButton,
      exact: true,
    });
  }

  /**
   * The row for a user, found by the email it lists.
   *
   * @param email - The user's email address.
   */
  userRow(email: string): Locator {
    return this.main().getByRole("row").filter({ hasText: email });
  }

  /** Every user row currently listed, identified by their edit control. */
  userRows(): Locator {
    return this.main().getByRole("row").filter({
      has: this.page.getByRole("button", {
        name: SETTINGS.userManagement.editUserButton,
        exact: true,
      }),
    });
  }

  /**
   * The edit control on a user's row.
   *
   * The control's accessible name is the same on every row, so it has to be
   * reached through the row it belongs to.
   *
   * @param email - The user's email address.
   */
  editUserButton(email: string): Locator {
    return this.userRow(email).getByRole("button", {
      name: SETTINGS.userManagement.editUserButton,
      exact: true,
    });
  }

  /**
   * The remove control on a user's row.
   *
   * @param email - The user's email address.
   */
  removeUserButton(email: string): Locator {
    return this.userRow(email).getByRole("button", {
      name: SETTINGS.userManagement.removeUserButton,
      exact: true,
    });
  }

  modal(): Locator {
    return this.page.getByRole("dialog");
  }

  /**
   * Opens the Edit User Roles modal for a user.
   *
   * @param email - The user's email address.
   */
  async openEditUserModal(email: string): Promise<void> {
    await this.editUserButton(email).click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(
      modal.getByText(SETTINGS.userManagement.editModal.title),
    ).toBeVisible();
  }

  /**
   * A role checkbox in the Edit User Roles modal.
   *
   * Matched loosely on the role name: the checkbox carries no `aria-label`, so
   * its accessible name is the whole label — the role plus its description
   * ("Lead A portal user who can escalate an issue beyond level 3") — and an
   * exact match finds nothing. No role's description contains another role's
   * name, so a substring match still resolves to one.
   *
   * @param label - The role's label.
   */
  roleCheckbox(label: string): Locator {
    return this.modal().getByRole("checkbox", { name: label });
  }

  /** The modal's save control. Disabled until something actually changes. */
  saveRolesButton(): Locator {
    return this.modal().getByRole("button", {
      name: SETTINGS.userManagement.editModal.saveButton,
      exact: true,
    });
  }

  /**
   * Saves the role change and waits for the PATCH to land.
   *
   * @param projectId - Project the contact belongs to.
   * @returns The update response.
   */
  async saveRoles(projectId: string): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          new RegExp(`/projects/${projectId}/contacts/`).test(
            new URL(r.url()).pathname,
          ) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.saveRolesButton().click(),
    ]);
    return response;
  }

  //
  // Display tab.
  //

  /**
   * A font-size option.
   *
   * These are radios, not buttons — the group is a radiogroup of `div`s, each
   * carrying an "Font size: <label>" aria-label.
   *
   * @param label - The option's label, e.g. "Large".
   */
  fontSizeOption(label: string): Locator {
    return this.main().getByRole("radio", {
      name: SETTINGS.display.fontSizeOption(label),
      exact: true,
    });
  }

  /**
   * The root font size the document is currently rendered at.
   *
   * Choosing an option writes this onto `<html>`, so it is the observable effect
   * of the setting rather than a class name or a highlighted control.
   *
   * @returns The computed `font-size` of the root element.
   */
  async rootFontSize(): Promise<string> {
    return this.page.evaluate(
      () => document.documentElement.style.fontSize,
    );
  }
}
