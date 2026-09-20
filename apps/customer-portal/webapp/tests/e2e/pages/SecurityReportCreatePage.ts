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

import path from "node:path";
import { type Locator, type Page, expect } from "../fixtures/test";
import {
  CASE_DETAIL,
  CREATE_CASE,
  CREATE_SECURITY_REPORT,
  GET_HELP_BUTTON,
  GET_HELP_MENU,
} from "../utils/selectors";
import { projectPathPattern } from "../utils/ids";

/** How long to allow for the dashboard header and the form to finish loading —
 * both are skeletonised while their queries resolve, well beyond the 5s default
 * expect timeout. */
const LOAD_TIMEOUT_MS = 60_000;

/** Escapes regex metacharacters so a literal label can be embedded in a pattern.
 * Product and deployment names contain dots (version numbers) and could contain
 * brackets, which would otherwise change what the pattern matches. */
function escapeForRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Page object for the create-security-report form at
 * `/projects/:projectId/support/security-report/create`.
 *
 * The same `CreateCasePage` component backs this route, with `isSecurityReport`
 * derived from the path — so Deployment, Product, Title and Description behave
 * exactly as on the case form, while Issue Type and Severity are hidden and an
 * attachment becomes mandatory.
 */
export class SecurityReportCreatePage {
  constructor(private readonly page: Page) {}

  /** The app's <main> region, keeping the surrounding chrome out of locators. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the project dashboard and starts a security report from the "Get
   * Help" split button's dropdown.
   *
   * The Security Report item only appears when the project has SRA write access
   * (`isSecurityReportVisible` in GetHelpDropdown.tsx), so a project without it
   * fails here rather than somewhere more obscure.
   *
   * @param projectId - Project to raise the report under.
   */
  async openViaGetHelpMenu(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);
    await expect(
      this.page.getByRole("button", { name: GET_HELP_BUTTON, exact: true }),
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    await this.openGetHelpMenu();
    await this.securityReportMenuItem().click();

    await expect(this.page).toHaveURL(
      projectPathPattern(projectId, "support/security-report/create"),
    );
    // The Product select is the readiness signal: verified present on all three
    // project types, whereas Deployment is absent on Cloud Support, whose
    // deployment is locked to primary production.
    await expect(this.productVersionSelect()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * Opens the Get Help dropdown from the split button's arrow half.
   *
   * The menu gets the long timeout rather than the 5s default: the header
   * renders skeletons until the projects list resolves, so on a cold dashboard
   * the click can land before the real menu is mounted.
   */
  async openGetHelpMenu(): Promise<void> {
    await this.page.getByRole("button", { name: GET_HELP_MENU.trigger }).click();
    await expect(this.getHelpMenu()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  getHelpMenu(): Locator {
    return this.page.getByRole("menu");
  }

  /** The "Security Report" item. Present only when the project has SRA write
   * access, so its absence is itself an assertable fact. */
  securityReportMenuItem(): Locator {
    return this.getHelpMenu()
      .getByRole("menuitem")
      .filter({ hasText: GET_HELP_MENU.items.securityReport });
  }

  deploymentSelect(): Locator {
    return this.page
      .getByRole("combobox")
      .filter({ hasText: CREATE_CASE.placeholders.deployment });
  }

  productVersionSelect(): Locator {
    return this.page
      .getByRole("combobox")
      .filter({ hasText: CREATE_CASE.placeholders.productVersion });
  }

  /** The Title field. Read-only in practice: for security reports its value is
   * generated from deployment + product + today's date, and the effect that does
   * so overwrites anything typed. */
  titleInput(): Locator {
    return this.page.locator(CREATE_CASE.ids.title);
  }

  descriptionEditor(): Locator {
    return this.main().getByTestId(CREATE_CASE.testIds.description);
  }

  /** The Issue Type / Severity controls, which this form must not render. */
  issueTypeSelect(): Locator {
    return this.page.locator(CREATE_CASE.ids.issueType);
  }

  severitySelect(): Locator {
    return this.page.locator(CREATE_CASE.ids.severity);
  }

  /** The dropzone that opens the upload modal. Rendered as a Paper with
   * role="button", not a file input — the input lives inside the modal. */
  attachDropzone(): Locator {
    return this.main().getByRole("button", {
      name: new RegExp(CREATE_SECURITY_REPORT.uploadDropzone),
    });
  }

  submitButton(): Locator {
    return this.page.getByRole("button", {
      name: CREATE_SECURITY_REPORT.submitButton,
    });
  }

  errorAlert(): Locator {
    return this.page.getByRole("alert");
  }

  /** The shared upload modal opened by the dropzone. */
  uploadModal(): Locator {
    return this.page.getByRole("dialog");
  }

  /** The modal's confirm control. Reads "Add" rather than "Upload" because
   * CreateCasePage passes `onSelect`, holding the file locally until submit. */
  uploadModalConfirm(): Locator {
    return this.uploadModal().getByRole("button", {
      name: CREATE_SECURITY_REPORT.uploadModal.confirmButton,
      exact: true,
    });
  }

  /** Opens the upload modal without choosing a file. */
  async openUploadModal(): Promise<void> {
    await this.attachDropzone().click();
    await expect(this.uploadModal()).toBeVisible();
  }

  async cancelUploadModal(): Promise<void> {
    await this.uploadModal()
      .getByRole("button", { name: "Cancel", exact: true })
      .click();
    await expect(this.uploadModal()).toBeHidden();
  }

  /** Clicks submit without waiting for a create response — for submissions
   * expected to be rejected before any request is made. */
  async attemptSubmit(): Promise<void> {
    await this.submitButton().click();
  }

  private async chooseOption(select: Locator, option: string): Promise<void> {
    // These Selects are replaced by Skeletons while their options load, so the
    // control genuinely does not exist yet on a cold navigation.
    await expect(select).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(select).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await select.click();
    await this.page.getByRole("option", { name: option, exact: true }).click();
  }

  async selectDeployment(name: string): Promise<void> {
    await this.chooseOption(this.deploymentSelect(), name);
  }

  async selectProductVersion(name: string): Promise<void> {
    await this.chooseOption(this.productVersionSelect(), name);
  }

  /**
   * Pattern the auto-generated title must match:
   * `<deployment> - <product name> - YYYY-MM-DD`.
   *
   * All three segments are matched exactly. Only the date is computed — built
   * rather than hardcoded so it is always today's, matching how CreateCasePage
   * generates it.
   *
   * @param deployment - Deployment label the report is filed against, or null
   * when the project auto-selects it (Cloud Support): the generator uses whichever
   * primary production deployment it picked, whose label the fixtures do not
   * record, so that segment is matched loosely.
   * @param productName - Product name *without* its version, as the generator
   * uses (`ProjectFixture.productName`, not `productVersion`).
   * @returns A regex the Title field's value should satisfy.
   */
  static expectedTitlePattern(
    deployment: string | null,
    productName: string,
  ): RegExp {
    const today = new Date();
    const date = [
      today.getFullYear(),
      String(today.getMonth() + 1).padStart(2, "0"),
      String(today.getDate()).padStart(2, "0"),
    ].join("-");
    const deploymentSegment = deployment ? escapeForRegExp(deployment) : ".+";
    return new RegExp(
      `^${deploymentSegment} - ${escapeForRegExp(productName)} - ${date}$`,
    );
  }

  /**
   * Types into the Lexical description editor. It is a contenteditable, so
   * `fill()` does not apply.
   *
   * @param text - Description body.
   */
  async fillDescription(text: string): Promise<void> {
    const editor = this.descriptionEditor();
    await editor.click();
    await editor.pressSequentially(text);
  }

  /**
   * Attaches the security report file through the upload modal.
   *
   * The dropzone opens `UploadAttachmentModal`, whose file input is
   * `display: none` — `setInputFiles` drives it directly, which is the
   * supported way to bypass the OS file picker. The confirm button reads "Add"
   * rather than "Upload" because CreateCasePage passes an `onSelect` handler, so
   * the file is held in page state and uploaded only when the report is
   * submitted.
   *
   * @param relativePath - Path to the file, relative to the tests/e2e directory.
   */
  async attachSecurityReport(relativePath: string): Promise<void> {
    await this.attachDropzone().click();

    const modal = this.page.getByRole("dialog");
    await expect(modal).toBeVisible();
    await expect(
      this.page.getByText(CREATE_SECURITY_REPORT.uploadModal.title),
    ).toBeVisible();

    // relativePath is resolved against tests/e2e; setInputFiles needs a full
    // filesystem path, not one relative to the spec.
    const attachmentPath = path.join(process.cwd(), "tests", "e2e", relativePath);
    await modal.locator('input[type="file"]').setInputFiles(attachmentPath);

    const confirm = modal.getByRole("button", {
      name: CREATE_SECURITY_REPORT.uploadModal.confirmButton,
      exact: true,
    });
    await expect(confirm).toBeEnabled();
    await confirm.click();
    await expect(modal).toBeHidden();
  }

  async submit(): Promise<void> {
    await this.submitButton().click();
  }
}
