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
import {
  CASE_DETAIL,
  CREATE_CASE,
  CREATE_SERVICE_REQUEST,
  GET_HELP_BUTTON,
  GET_HELP_MENU,
} from "../utils/selectors";

/** How long to allow for the project dashboard's header to finish loading.
 * Well above the 5s default expect timeout, because the header is skeletonised
 * until the projects list resolves. */
const DASHBOARD_LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the create-service-request form at
 * `/projects/:projectId/support/service-requests/create`
 * (`CreateServiceRequestPage.tsx`).
 *
 * Deployment and Product reuse the same placeholder-matched MUI Selects as the
 * case form. The Request Details fields are different in kind: they are built
 * from the chosen catalog item's ServiceNow variables, so their ids are
 * React-generated (`_r_16_`) and change between renders, and their labels are
 * sibling `<span>`s rather than `<label for>`. They are therefore addressed
 * structurally — see `requestDetailsInput()`.
 */
export class ServiceRequestCreatePage {
  constructor(private readonly page: Page) {}

  /** The app's <main> region — scoping everything to it keeps the surrounding
   * chrome (global search, promo banner) out of the locators. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens the project dashboard and starts a service request from the "Get
   * Help" split button's dropdown, then waits for the form.
   *
   * The Service Request item only appears when the project has service-request
   * read access (`isServiceRequestVisible` in GetHelpDropdown.tsx), so a
   * project without it fails here rather than somewhere more obscure.
   *
   * @param projectId - Project to raise the request under.
   */
  async openViaGetHelpMenu(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);
    // The header renders skeletons until the projects list resolves, which on a
    // cold dashboard load can exceed the 5s default expect timeout — observed
    // failing here when this spec ran last in a full-suite run. The wait is for
    // the real split button to replace the skeleton, so it needs the longer
    // budget rather than the default.
    await expect(
      this.page.getByRole("button", { name: GET_HELP_BUTTON, exact: true }),
    ).toBeVisible({ timeout: DASHBOARD_LOAD_TIMEOUT_MS });

    await this.page
      .getByRole("button", { name: GET_HELP_MENU.trigger })
      .click();
    const menu = this.page.getByRole("menu");
    await expect(menu).toBeVisible();
    await menu
      .getByRole("menuitem")
      .filter({ hasText: GET_HELP_MENU.items.serviceRequest })
      .click();

    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${projectId}/support/service-requests/create`),
    );
    await expect(this.deploymentSelect()).toBeVisible();
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

  /** Accordion header for a request-type catalog, e.g. "Generic Requests". */
  catalogAccordion(name: string): Locator {
    return this.main().getByRole("button").filter({ hasText: name });
  }

  /** A catalog item's radio. Rendered as role="radio" by CatalogSelector. */
  catalogItemRadio(name: string): Locator {
    return this.main().getByRole("radio", { name });
  }

  /**
   * The single-line Request Details input.
   *
   * The catalog item's variables carry no stable id, test id, or associated
   * label, so this selects on the one structural fact that holds: within
   * <main> it is the only control that is both exposed as a textbox and
   * enabled. Verified live — of the elements in <main>:
   *   - the Project field is a textbox but disabled;
   *   - the description editor is a contenteditable div, not an input;
   *   - the Deployment/Product Selects render enabled `MuiSelect-nativeInput`
   *     inputs, but they are aria-hidden so carry no textbox role.
   * Matching on `input:enabled` alone picks up those hidden Select inputs (5
   * matches), which is why the role is part of the locator rather than a
   * bare CSS selector.
   *
   * `fillRequestDetails` asserts the count is exactly one, so a catalog item
   * with extra variables fails loudly instead of typing into the wrong box.
   */
  requestDetailsInput(): Locator {
    return this.main().getByRole("textbox").and(this.page.locator(":enabled"));
  }

  /** Lexical rich-text description editor — a contenteditable, not an input. */
  descriptionEditor(): Locator {
    return this.main().getByTestId(CREATE_SERVICE_REQUEST.testIds.description);
  }

  submitButton(): Locator {
    return this.page.getByRole("button", {
      name: CREATE_SERVICE_REQUEST.submitButton,
    });
  }

  /**
   * Selects a deployment by its exact name.
   *
   * The select loads deployments 10 at a time and fetches the next page when
   * its list is scrolled to the bottom (CreateServiceRequestPage). Runs of the
   * deployment specs keep adding "Automation Test Deployment …" records, which
   * push older ones such as "Production" off the first page — so this scrolls
   * until the option appears or the list stops growing.
   *
   * @param name - Exact deployment option label.
   */
  async selectDeployment(name: string): Promise<void> {
    await this.deploymentSelect().click();
    const option = this.page.getByRole("option", { name, exact: true });
    const listbox = this.page.getByRole("listbox");
    await expect(listbox.getByRole("option").nth(1)).toBeVisible({
      timeout: 60_000,
    });

    let previous = -1;
    for (let attempt = 0; attempt < 30 && !(await option.count()); attempt++) {
      const count = await listbox.getByRole("option").count();
      if (count === previous) {
        // Nothing new arrived after the last scroll: give the next page a
        // moment, then stop if it still has not grown.
        await this.page.waitForTimeout(1_500);
        if ((await listbox.getByRole("option").count()) === count) break;
      }
      previous = count;
      await listbox.evaluate((el) => {
        el.scrollTop = el.scrollHeight;
      });
      await this.page.waitForTimeout(500);
    }

    await option.scrollIntoViewIfNeeded();
    await option.click();
  }

  /**
   * Selects a product. Waits for the control to become enabled first — options
   * are fetched per-deployment, so it is disabled immediately after one is
   * chosen.
   *
   * @param name - Exact product option label.
   */
  async selectProductVersion(name: string): Promise<void> {
    const select = this.productVersionSelect();
    await expect(select).toBeEnabled();
    await select.click();
    await this.page.getByRole("option", { name, exact: true }).click();
  }

  /**
   * Expands a catalog and picks one of its items.
   *
   * @param catalog - Catalog accordion name.
   * @param item - Radio item name within it.
   */
  async selectRequestType(catalog: string, item: string): Promise<void> {
    await expect(
      this.main().getByText(CREATE_SERVICE_REQUEST.requestTypeHeading).first(),
    ).toBeVisible();
    await this.catalogAccordion(catalog).click();

    const radio = this.catalogItemRadio(item);
    await radio.click();
    await expect(radio).toHaveAttribute("aria-checked", "true");

    // Selecting an item loads its variables, which is what renders the section
    // the next steps type into.
    await expect(
      this.main()
        .getByRole("heading", { name: CREATE_SERVICE_REQUEST.detailsHeading })
        .first(),
    ).toBeVisible();
  }

  async fillRequestDetails(text: string): Promise<void> {
    const input = this.requestDetailsInput();
    await expect(
      input,
      "expected exactly one enabled text input in the Request Details section",
    ).toHaveCount(1);
    await input.fill(text);
  }

  /**
   * Types into the Lexical description editor. It is a contenteditable, so
   * `fill()` does not apply — the text is typed so Lexical's key handling
   * builds the editor state the form reads.
   *
   * @param text - Description body.
   */
  async fillDescription(text: string): Promise<void> {
    const editor = this.descriptionEditor();
    await editor.click();
    await editor.pressSequentially(text);
  }

  async submit(): Promise<void> {
    await this.submitButton().click();
  }
}
