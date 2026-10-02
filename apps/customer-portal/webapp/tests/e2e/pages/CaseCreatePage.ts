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

  /**
   * Opens the create-case form by URL, bypassing Get Help.
   *
   * Get Help branches on the project's `hasAgent`: with the assistant enabled it
   * opens the chat instead of this form, so a spec that only needs a case
   * cannot use it while Novera is on — and a non-admin cannot turn Novera off.
   * Navigating straight to the route sidesteps that entirely, which keeps case
   * creation independent of a global flag other specs may have changed.
   *
   * The form arrives pre-populated here (it shares the route with the
   * chat-originated variant), so callers review and overwrite rather than fill
   * from empty.
   *
   * @param projectId - Project to raise the case under.
   */
  async openDirect(projectId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CREATE_CASE.pathSegment}`,
    );
    await expect(
      this.page.getByRole("heading", { name: CREATE_CASE.heading }),
    ).toBeVisible({ timeout: FORM_LOAD_TIMEOUT_MS });

    // Product is the reliable readiness signal for every project type — see
    // openViaGetHelp.
    await expect(this.productVersionSelect()).toBeVisible({
      timeout: FORM_LOAD_TIMEOUT_MS,
    });
  }


  /**
   * The product field while it is still gated on choosing a deployment.
   *
   * Asserting this is GONE is how the gate lifting is verified. The obvious
   * alternative — waiting for the product placeholder to become enabled —
   * fails whenever the chosen deployment has exactly one product, because the
   * form selects it automatically and the placeholder never appears.
   */
  productGateMessage(): Locator {
    return this.main()
      .getByRole("combobox")
      .filter({ hasText: CREATE_CASE.placeholders.productGatedOnDeployment });
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
  /**
   * A dropdown option, matched on its exact accessible name.
   *
   * Deliberately not `filter({ hasText: new RegExp(...) })`: the labels here are
   * project data and routinely contain regex metacharacters, so interpolating
   * one into a pattern makes the match broader than it looks.
   *
   * @param label - The option's exact visible text.
   */
  private optionByName(label: string): Locator {
    return this.page.getByRole("option", { name: label, exact: true });
  }

  /**
   * The Deployment select, WHILE IT STILL SHOWS ITS PLACEHOLDER.
   *
   * ⚠️ These two selects are matched on placeholder text rather than on a
   * stable handle because the app gives them none: BasicInformationSection
   * renders bare MUI `<Select>` elements with no id, no labelId and no
   * InputLabel, so the combobox has no accessible name and the visible
   * "Deployment *" beside it is an unassociated text node. `getByLabel` and an
   * id selector both fail against it.
   *
   * The consequence to know: the locator stops matching once a value is
   * chosen, so it answers "is this still awaiting a choice?", not "where is the
   * deployment select". That is exactly what the `toBeDisabled` / `toBeHidden`
   * assertions at the call sites want, and {@link selectDeployment} treats a
   * vanished placeholder as "already selected" rather than an error.
   *
   * Giving those selects an id in the app would allow a value-independent
   * locator and is the real fix.
   */
  deploymentSelect(): Locator {
    return this.main()
      .getByRole("combobox")
      .filter({ hasText: CREATE_CASE.placeholders.deployment });
  }

  /** The MUI Select for Product Version. Stays disabled, reading "Select
   * deployment first", until a deployment is chosen. */
  productVersionSelect(): Locator {
    return this.main()
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

  /**
   * Confirms a Select that no longer shows its placeholder genuinely holds a
   * chosen value, so "(already selected)" is only ever returned on evidence.
   *
   * The placeholder locators stop matching both when a value was chosen AND when
   * the control never rendered (a failed options fetch leaves the Skeleton in
   * place). Treating the second as the first would hide a real failure, so the
   * fallback requires a visible combobox whose text is not a placeholder.
   *
   * The two value Selects are the form's only comboboxes besides issue type and
   * severity (which have ids). Deployment is the first of them; Product Version
   * is the last, which also holds on Cloud Support where Deployment is hidden.
   *
   * @param position - Which of the value Selects to inspect.
   * @param field - Field name, for the failure message.
   * @throws When the control is absent or still shows a placeholder.
   */
  private async expectAlreadyChosen(
    position: "first" | "last",
    field: string,
  ): Promise<void> {
    const candidates = this.main().locator(
      `[role="combobox"]:not(${CREATE_CASE.ids.issueType}):not(${CREATE_CASE.ids.severity})`,
    );
    const control = position === "first" ? candidates.first() : candidates.last();

    await expect(
      control,
      `${field}: its placeholder never appeared and no selected value is ` +
        "shown either — the control did not render",
    ).toBeVisible({ timeout: FORM_LOAD_TIMEOUT_MS });

    const shown = (await control.innerText()).trim();
    if (!shown || /^select\b/i.test(shown)) {
      throw new Error(
        `${field} was expected to be pre-selected but shows "${shown}".`,
      );
    }
  }

  async selectDeployment(name: string): Promise<string> {
    const select = this.deploymentSelect();

    // Located by PLACEHOLDER text, so a pre-populated form — which the
    // chat-shared route produces — matches nothing, and the match is transient
    // while loading ("Select deployment first"). Treat a vanished placeholder
    // as "already chosen" rather than an error.
    const needsChoosing = await select
      .waitFor({ state: "attached", timeout: FORM_LOAD_TIMEOUT_MS })
      .then(() => true)
      .catch(() => false);
    if (!needsChoosing) {
      await this.expectAlreadyChosen("first", "Deployment");
      return "(already selected)";
    }

    await expect(select).toBeEnabled({ timeout: FORM_LOAD_TIMEOUT_MS });
    await select.click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: FORM_LOAD_TIMEOUT_MS });

    // Exact accessible-name matching, not an interpolated RegExp: labels carry
    // metacharacters — product versions have dots ("WSO2 API Manager 4.5.0"),
    // severities have parentheses ("S4(Query)") — and unescaped those match
    // more than intended, so a lookup could select the wrong option.
    const exact = this.optionByName(name);
    if ((await exact.count()) > 0) {
      await exact.first().click();
      return name;
    }

    // Deployment names are project data, not fixtures, and they do change —
    // projects have at times carried only the timestamped records the
    // add-deployment spec leaves behind. Falling back keeps a fixture drift
    // from stalling for the full timeout on an option that cannot appear.
    //
    // "Add Deployment" is an ACTION in this list, not a deployment; selecting
    // it opens a creation dialog, so it is excluded.
    const labels = (await options.allTextContents())
      .map((text) => text.trim())
      .filter(
        (text) =>
          text && !/^select /i.test(text) && !/^add deployment$/i.test(text),
      );

    if (labels.length === 0) {
      throw new Error(
        `No deployments are available to choose from (wanted "${name}").`,
      );
    }

    const chosen = labels[0];
    console.log(
      `Deployment "${name}" is not offered on this project; using "${chosen}" ` +
        `instead (${labels.length} available). Update the fixture in ` +
        `config/testData.ts if this project should have "${name}".`,
    );
    await this.optionByName(chosen).first().click();
    return chosen;
  }

  /**
   * Selects a product version. Waits for the control to become enabled first —
   * the options are fetched per-deployment, so it is disabled immediately after
   * a deployment is chosen.
   *
   * @param name - Preferred product version label. When the selected deployment
   *   does not offer it, the first available product is used and logged.
   * @returns The product version actually selected.
   */
  async selectProductVersion(name: string): Promise<string> {
    const select = this.productVersionSelect();

    // Same placeholder caveat as selectDeployment: a pre-populated form has no
    // placeholder to match, and the "Select deployment first" variant appears
    // only transiently while the options load.
    const needsChoosing = await select
      .waitFor({ state: "attached", timeout: FORM_LOAD_TIMEOUT_MS })
      .then(() => true)
      .catch(() => false);
    if (!needsChoosing) {
      await this.expectAlreadyChosen("last", "Product Version");
      return "(already selected)";
    }

    // Waits for enabled on the long form-load budget: the options are refetched
    // after a deployment is picked, and the control is disabled until they land.
    await expect(select).toBeEnabled({ timeout: FORM_LOAD_TIMEOUT_MS });
    await select.click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: FORM_LOAD_TIMEOUT_MS });

    // Exact accessible-name matching, not an interpolated RegExp: labels carry
    // metacharacters — product versions have dots ("WSO2 API Manager 4.5.0"),
    // severities have parentheses ("S4(Query)") — and unescaped those match
    // more than intended, so a lookup could select the wrong option.
    const exact = this.optionByName(name);
    if ((await exact.count()) > 0) {
      await exact.first().click();
      return name;
    }

    // Products are scoped to the CHOSEN DEPLOYMENT, so this follows from the
    // deployment fallback above: a different deployment offers a different
    // product list, and insisting on the fixture's product would stall for the
    // full timeout on an option that cannot appear.
    const labels = (await options.allTextContents())
      .map((text) => text.trim())
      .filter((text) => text && !/^select /i.test(text));

    if (labels.length === 0) {
      throw new Error(
        `No product versions are available for the selected deployment ` +
          `(wanted "${name}").`,
      );
    }

    const chosen = labels[0];
    console.log(
      `Product "${name}" is not offered for the selected deployment; using ` +
        `"${chosen}" instead (${labels.length} available).`,
    );
    await this.optionByName(chosen).first().click();
    return chosen;
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
