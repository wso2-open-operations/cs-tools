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
import { CASE_DETAIL, CREATE_CASE, GET_HELP_BUTTON } from "../utils/selectors";
import { projectPathPattern } from "../utils/ids";

/** How long to allow for the create-case form to render. Well above the 5s
 * default expect timeout: the page waits on project details, features and
 * filters before the form appears. */
const FORM_LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the case-creation form at
 * `/projects/:projectId/support/chat/create-case` (`CreateCasePage.tsx`).
 *
 * Locator strategy: the form's field labels are plain sibling `<Typography>`
 * nodes, not `<label for>`, so `getByLabel` does not work here. Title, Issue
 * Type and Severity have stable element ids; the description is a Lexical
 * contenteditable carrying a `data-testid`; Deployment and Product Version have
 * neither, so they are located by the placeholder text their `renderValue`
 * emits.
 */
export class CaseCreatePage {
  constructor(private readonly page: Page) {}

  /**
   * Opens the project dashboard and starts case creation via the header's
   * "Get Help" button, then waits for the form to render.
   *
   * @param projectId - Project to create the case under (see PROJECTS in
   * ../config/testData.ts).
   */
  async openViaGetHelp(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);
    await this.page
      .getByRole("button", { name: GET_HELP_BUTTON, exact: true })
      .click();
    // "Get Help" branches on the project's Novera AI flag (see handleIssue in
    // GetHelpDropdown.tsx): with the agent enabled it opens the describe-issue
    // chat page instead of this form. Asserting the URL makes that divergence a
    // clear failure rather than a confusing missing-field error.
    await expect(this.page).toHaveURL(
      projectPathPattern(projectId, "support/chat/create-case"),
    );
    await expect(
      this.page.getByRole("heading", { name: CREATE_CASE.heading }),
    ).toBeVisible({ timeout: FORM_LOAD_TIMEOUT_MS });

    // The heading renders before the Basic Information selects do — those are
    // Skeletons until deployments/products resolve. The Product select is the
    // reliable readiness signal because it is present for every project type
    // (Cloud Support hides Deployment but still shows Product), so waiting on
    // it lets callers assert on field state without racing the load.
    await expect(this.productVersionSelect()).toBeVisible({
      timeout: FORM_LOAD_TIMEOUT_MS,
    });
  }

  /** The app's <main> region, for scoping text assertions away from the
   * surrounding chrome. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /** Error banner shown by `showError` for failed submit validation. */
  errorAlert(): Locator {
    return this.page.getByRole("alert");
  }

  /** The `<n>/160` character counter under the Title field. */
  titleCounter(): Locator {
    return this.main().getByText(CREATE_CASE.titleCounter);
  }

  /** Field-level error shown when the title exceeds 160 characters. */
  titleLengthError(): Locator {
    return this.main().getByText(CREATE_CASE.titleTooLongError);
  }

  /** Clicks submit without waiting for a create response — for cases that are
   * expected to be rejected before any request is made. */
  async attemptSubmit(): Promise<void> {
    await this.submitButton().click();
  }

  /**
   * Fills only the Basic Information fields, leaving the case details empty.
   * Validation tests build on this and then supply just the field under test.
   *
   * @param deployment - Deployment label, or empty when the project
   * auto-selects it (Cloud Support).
   * @param productVersion - Product option label.
   */
  async fillBasicInformation(
    deployment: string,
    productVersion: string,
  ): Promise<void> {
    if (deployment) {
      await this.selectDeployment(deployment);
    }
    await this.selectProductVersion(productVersion);
  }

  /** The MUI Select for Deployment, matched on its placeholder text. */
  deploymentSelect(): Locator {
    return this.page
      .getByRole("combobox")
      .filter({ hasText: CREATE_CASE.placeholders.deployment });
  }

  /** The MUI Select for Product Version. Stays disabled, reading "Select
   * deployment first", until a deployment is chosen. */
  productVersionSelect(): Locator {
    return this.page
      .getByRole("combobox")
      .filter({ hasText: CREATE_CASE.placeholders.productVersion });
  }

  titleInput(): Locator {
    return this.page.locator(CREATE_CASE.ids.title);
  }

  descriptionEditor(): Locator {
    return this.page.getByTestId(CREATE_CASE.testIds.description);
  }

  issueTypeSelect(): Locator {
    return this.page.locator(CREATE_CASE.ids.issueType);
  }

  severitySelect(): Locator {
    return this.page.locator(CREATE_CASE.ids.severity);
  }

  submitButton(): Locator {
    return this.page.getByRole("button", { name: CREATE_CASE.submitButton });
  }

  /**
   * Opens a MUI Select and picks an option by its exact visible text.
   *
   * @param select - The Select control to open.
   * @param option - Exact option label to choose.
   */
  private async chooseOption(select: Locator, option: string): Promise<void> {
    // Each of these Selects is replaced by a Skeleton while its options are
    // being fetched (see BasicInformationSection / CaseDetailsSection), so the
    // control genuinely does not exist yet on a cold load — waiting for it to
    // be present and interactive is required, not belt-and-braces.
    await expect(select).toBeVisible({ timeout: FORM_LOAD_TIMEOUT_MS });
    await expect(select).toBeEnabled({ timeout: FORM_LOAD_TIMEOUT_MS });
    await select.click();
    await this.page.getByRole("option", { name: option, exact: true }).click();
  }

  async selectDeployment(name: string): Promise<void> {
    await this.chooseOption(this.deploymentSelect(), name);
  }

  /**
   * Selects a product version. Waits for the control to become enabled first —
   * the options are fetched per-deployment, so it is disabled immediately after
   * a deployment is chosen.
   *
   * @param name - Exact product version label.
   */
  async selectProductVersion(name: string): Promise<void> {
    // chooseOption already waits for the control to be present and enabled, on
    // the long form-load budget — which is what this needs, since the options
    // are refetched after a deployment is picked.
    await this.chooseOption(this.productVersionSelect(), name);
  }

  async fillTitle(title: string): Promise<void> {
    await this.titleInput().fill(title);
  }

  /**
   * Types into the Lexical description editor. It is a contenteditable, so
   * `fill()` does not apply — the text is typed so Lexical's own key handling
   * builds the editor state the form reads.
   *
   * @param text - Description body.
   */
  async fillDescription(text: string): Promise<void> {
    const editor = this.descriptionEditor();
    await editor.click();
    await editor.pressSequentially(text);
  }

  async selectIssueType(name: string): Promise<void> {
    await this.chooseOption(this.issueTypeSelect(), name);
  }

  async selectSeverity(name: string): Promise<void> {
    await this.chooseOption(this.severitySelect(), name);
  }

  async submit(): Promise<void> {
    await this.submitButton().click();
  }
}
