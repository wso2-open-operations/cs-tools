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
import { idPattern, projectPathPattern } from "../utils/ids";

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
      projectPathPattern(projectId, SETTINGS.pathSegment),
    );
    await expect(this.tab(SETTINGS.tabs.userManagement)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * Opens settings directly by URL.
   *
   * For getting *back* to the page mid-test — a restore step should not depend on
   * a dashboard load and a side-nav click, which is a long way round and one more
   * thing that can fail while state is left changed.
   *
   * @param projectId - Project whose settings to open.
   */
  async open(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/${SETTINGS.pathSegment}`);
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

  /** Every tab on the Settings page, for asserting how many there are. */
  tabs(): Locator {
    return this.main().getByRole("tab");
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

  /**
   * Every user row currently listed, regardless of who is looking.
   *
   * Matched on the email each row carries, NOT on the row's edit control.
   * {@link userRows} uses the latter, which is admin-only — the Actions column
   * and its buttons render inside `canAddOrRemoveUsers`, so for a Portal, Lead
   * or Security user it matches nothing. Using it as a readiness signal is
   * therefore circular: a non-admin waits forever for a control they are not
   * permitted to see.
   */
  anyUserRows(): Locator {
    return this.main().getByRole("row").filter({ hasText: /@/ });
  }

  /**
   * The user list's column headers, present for every role.
   *
   * The Actions column is admin-only, so only the first three are guaranteed —
   * which is why this takes the header to wait for rather than assuming one.
   *
   * @param label - Header text, e.g. "User".
   */
  userTableHeader(label: string): Locator {
    return this.main().getByRole("columnheader", { name: label, exact: true });
  }

  /**
   * The name, email and role labels a user row displays.
   *
   * Read off the ROW rather than taken from config, so the values are
   * guaranteed to exist — a search built from a fixture that has since changed
   * finds nothing and reads as a broken filter rather than stale data.
   *
   * The row renders the name and email stacked in the first cell and the roles
   * as chips, so the text comes back line-separated; the email is picked out by
   * shape and the roles by matching the known badge labels.
   *
   * @param row - The row to read.
   * @returns Its displayed name, email and role labels.
   */
  async userRowDetails(
    row: Locator,
  ): Promise<{ name: string; email: string; roles: string[] }> {
    const lines = (await row.innerText())
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    const emailIndex = lines.findIndex((line) => /^\S+@\S+$/.test(line));
    const email = emailIndex >= 0 ? lines[emailIndex] : "";

    // The name sits directly above the email. Falls back to empty rather than
    // guessing, so a contact with no name does not silently yield the initials
    // badge or a placeholder as its "name".
    const nameLine = emailIndex > 0 ? lines[emailIndex - 1] : "";
    const name = nameLine === SETTINGS.userManagement.nullPlaceholder ? "" : nameLine;

    const roles = SETTINGS.userManagement.roleLabels.filter((label) =>
      lines.includes(label),
    );

    return { name, email, roles };
  }

  //
  // Add User. A two-step modal: an email that is validated server-side, then
  // name and roles.
  //

  /** The Add User modal. */
  addUserModal(): Locator {
    return this.page
      .getByRole("dialog")
      .filter({ hasText: SETTINGS.userManagement.addUserModal.title });
  }

  /** Opens the modal and waits for its first step. */
  async openAddUserModal(): Promise<void> {
    await expect(this.addUserButton()).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await this.addUserButton().click();
    await expect(this.addUserModal()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Fills step 1 and advances, failing with the backend's own reason if the
   * address is rejected.
   *
   * The email is validated server-side before step 2 opens, so a rejected
   * address leaves the modal on step 1. Surfacing that message matters: the
   * project enforces an allowed-domain list, and "the name field never
   * appeared" would say nothing about why.
   *
   * @param email - Address to invite.
   */
  async submitAddUserEmail(email: string): Promise<void> {
    const modal = this.addUserModal();
    const addUser = SETTINGS.userManagement.addUserModal;

    await modal.getByPlaceholder(addUser.emailPlaceholder).fill(email);

    // Tied to the VALIDATE response rather than to the step changing.
    //
    // Both outcomes leave step 1 on screen for a moment — the button reads
    // "Validating..." while the request is in flight — so a check made straight
    // after the click reads a still-pending modal and reports a rejection that
    // never happened. Waiting for the response first removes the race.
    const [validation] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          new URL(r.url()).pathname.endsWith("/contacts/validate"),
        { timeout: LOAD_TIMEOUT_MS },
      ),
      modal
        .getByRole("button", { name: addUser.nextButton, exact: true })
        .click(),
    ]);

    const firstName = modal.getByPlaceholder(addUser.firstNamePlaceholder);
    const emailField = modal.getByPlaceholder(addUser.emailPlaceholder);

    // Then for the modal to settle on one of its two states.
    await expect(firstName.or(emailField).first()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
    await expect(
      modal.getByRole("button", { name: addUser.validatingButton }),
    ).toHaveCount(0, { timeout: LOAD_TIMEOUT_MS });

    if ((await firstName.count()) === 0) {
      const body = await validation.text().catch(() => "");
      const reason = (await modal.innerText()).replace(/\s+/g, " ").trim();
      throw new Error(
        `"${email}" was rejected at the email step, so the user could not be ` +
          `added. Validate returned ${validation.status()} ${body}. ` +
          `The modal said: ${reason}`,
      );
    }
  }

  /**
   * Completes step 2 and captures the create request.
   *
   * @param firstName - Given name.
   * @param lastName - Family name.
   * @returns The POST /contacts response.
   */
  async submitAddUserDetails(
    firstName: string,
    lastName: string,
  ): Promise<Response> {
    const modal = this.addUserModal();
    const addUser = SETTINGS.userManagement.addUserModal;

    // Overwritten rather than appended to: step 1 pre-fills these when the
    // address already belongs to a known contact.
    await modal.getByPlaceholder(addUser.firstNamePlaceholder).fill(firstName);
    await modal.getByPlaceholder(addUser.lastNamePlaceholder).fill(lastName);

    const submit = modal.getByRole("button", {
      name: addUser.submitButton,
      exact: true,
    });
    await expect(submit).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          new URL(r.url()).pathname.endsWith("/contacts"),
        { timeout: LOAD_TIMEOUT_MS },
      ),
      submit.click(),
    ]);
    return response;
  }

  /**
   * Removes a user through the per-row control and its confirmation.
   *
   * @param email - The user to remove.
   * @returns The DELETE /contacts/{email} response.
   */
  async removeUser(email: string): Promise<Response> {
    await expect(this.removeUserButton(email)).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
    await this.removeUserButton(email).click();

    const confirm = this.page
      .getByRole("dialog")
      .filter({ hasText: SETTINGS.userManagement.removeUserModal.title });
    await expect(confirm).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          r.request().method() === "DELETE" &&
          new URL(r.url()).pathname.includes("/contacts/"),
        { timeout: LOAD_TIMEOUT_MS },
      ),
      confirm
        .getByRole("button", {
          name: SETTINGS.userManagement.removeUserModal.confirmButton,
          exact: true,
        })
        .click(),
    ]);
    return response;
  }

  /** Every user row currently listed, identified by their edit control.
   *
   * ⚠️ Admin-only by construction — see {@link anyUserRows}. */
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
          new RegExp(`/projects/${idPattern(projectId)}/contacts/`).test(
            new URL(r.url()).pathname,
          ) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.saveRolesButton().click(),
    ]);
    return response;
  }

  //
  // AI Assistant tab.
  //

  /**
   * Text within the settings page, for asserting a notice or label is shown.
   *
   * Returns every match rather than narrowing: some strings legitimately appear
   * more than once, and callers assert on visibility or count rather than
   * pretending there is exactly one.
   *
   * @param text - Exact text to find.
   */
  settingsText(text: string): Locator {
    return this.main().getByText(text, { exact: true });
  }

  /** The Support Capabilities section heading. */
  capabilitiesSection(): Locator {
    return this.main().getByText(SETTINGS.aiAssistant.capabilitiesSection, {
      exact: true,
    });
  }

  /** The AI Chat Assistant (Novera) row's label. */
  noveraLabel(): Locator {
    return this.main().getByText(SETTINGS.aiAssistant.novera.label, {
      exact: true,
    });
  }

  /**
   * The Novera toggle.
   *
   * A `switch`, not a `checkbox` — MUI's Switch carries `role="switch"`, and the
   * tab has no checkbox at all, so a checkbox locator finds nothing and hangs.
   * Verified live.
   *
   * Located without a name: it is labelled through `aria-labelledby`, and the
   * tab renders exactly one switch, so narrowing further only adds a way to
   * mismatch.
   */
  noveraToggle(): Locator {
    return this.main().getByRole("switch");
  }

  /**
   * The state chip beside the Novera label.
   *
   * @param label - "Active" or "Inactive".
   */
  noveraChip(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /**
   * Flips the Novera toggle and waits for the project patch to land.
   *
   * The toggle writes `hasAgent` on the project, so the request body is what
   * distinguishes enabling from disabling.
   *
   * @param projectId - Project being updated.
   * @param enable - True to switch on, false to switch off.
   * @returns The update response.
   */
  async setNovera(projectId: string, enable: boolean): Promise<Response> {
    // A request that never gets a response — the backend rejecting it without
    // CORS headers, so the browser aborts it — would otherwise sit in
    // waitForResponse for the full timeout and report nothing but "timed out".
    // Racing the failure surfaces the actual reason at the point it happens.
    const matchesPatch = (url: string): boolean =>
      new RegExp(`/projects/${idPattern(projectId)}$`).test(
        new URL(url).pathname,
      );

    const failure = new Promise<never>((_, reject) => {
      this.page.on("requestfailed", (request) => {
        if (request.method() === "PATCH" && matchesPatch(request.url())) {
          reject(
            new Error(
              `PATCH ${request.url()} failed at the network layer ` +
                `(${request.failure()?.errorText}). The Novera toggle could ` +
                `not be saved, so the switch reverts and the assistant stays ` +
                `inactive. This is a backend/deployment fault, not a test one.`,
            ),
          );
        }
      });
    });

    const [response] = await Promise.all([
      Promise.race([
        this.page.waitForResponse(
        (r) =>
          // The portal calls the backend with whichever spelling of the id
          // is in the address bar, so this must accept both — matching only
          // the plain form means the wait never resolves and the toggle looks
          // like it failed to take.
          new RegExp(`/projects/${idPattern(projectId)}$`).test(
            new URL(r.url()).pathname,
          ) &&
          r.request().method() === "PATCH",
          { timeout: LOAD_TIMEOUT_MS },
        ),
        failure,
      ]),
      enable
        ? this.noveraToggle().check()
        : this.noveraToggle().uncheck(),
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
