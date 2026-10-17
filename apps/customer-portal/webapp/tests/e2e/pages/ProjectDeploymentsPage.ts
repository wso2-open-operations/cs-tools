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
import {
  ADD_DEPLOYMENT,
  ADD_PRODUCT,
  CASE_DETAIL,
  DEPLOYMENT_DOCUMENTS,
  DEPLOYMENTS_LIST,
  MUI_PAGINATION,
  DELETE_DEPLOYMENT,
  DELETE_PRODUCT,
  EDIT_DEPLOYMENT,
  MANAGE_PRODUCT,
  PROJECT_DETAILS,
} from "../utils/selectors";

/** How long to allow for the dashboard, the project-details page and the
 * deployments list to load — each is skeletonised while its queries resolve,
 * well beyond the 5s default expect timeout. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Deployments tab of the project details page
 * (`/projects/:projectId/project-details`).
 */
export class ProjectDeploymentsPage {
  constructor(private readonly page: Page) {}

  /** The app's <main> region. Scoping matters here: the modal's confirm button
   * shares its accessible name with the button that opens it. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Navigates from the project dashboard to the Deployments tab via the side
   * nav, then waits for the deployments list.
   *
   * Readiness is signalled by the Add Deployment button, which is permission
   * gated — `ProjectDeployments` withholds it entirely for a Restricted project.
   * That is fine for every fixture this suite has (none is Restricted) but means
   * this method cannot be pointed at a Restricted project as-is: it would time
   * out waiting for a button that is deliberately absent. A Restricted fixture
   * would need a readiness signal independent of the button — the deployments
   * list or its empty state.
   *
   * @param projectId - Project whose deployments to open.
   */
  async openDeploymentsTab(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);

    const navItem = this.page.getByRole("button", {
      name: PROJECT_DETAILS.navItem,
    });
    await expect(navItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await navItem.click();
    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${projectId}/${PROJECT_DETAILS.pathSegment}`),
    );

    await this.deploymentsTab().click();
    // The Add Deployment button only renders once the deployments query resolves
    // (and is withheld entirely for a Restricted project), so waiting on it
    // confirms the tab is ready to act on.
    await expect(this.addDeploymentButton()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * Opens the project details page without touching the Deployments tab.
   *
   * Separate from `openDeploymentsTab` so a spec can assert on the tab strip
   * itself — including a tab's absence — rather than assuming the tab is there.
   *
   * @param projectId - Project whose details to open.
   */
  async openProjectDetails(projectId: string): Promise<void> {
    await this.page.goto(`/projects/${projectId}/dashboard`);

    const navItem = this.page.getByRole("button", {
      name: PROJECT_DETAILS.navItem,
    });
    await expect(navItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await navItem.click();
    await expect(this.page).toHaveURL(
      new RegExp(`/projects/${projectId}/${PROJECT_DETAILS.pathSegment}`),
    );
  }

  /** The deployments list's empty copy, shown when a project has none. */
  emptyDeploymentsMessage(): Locator {
    return this.main().getByText(DEPLOYMENTS_LIST.emptyMessage);
  }

  deploymentsTab(): Locator {
    return this.page.getByRole("tab", {
      name: new RegExp(PROJECT_DETAILS.tabs.deployments),
    });
  }

  /** The button that opens the modal. Scoped to <main> and excluded from the
   * dialog, since the modal's confirm control has the same name. */
  addDeploymentButton(): Locator {
    return this.main().getByRole("button", {
      name: ADD_DEPLOYMENT.openButton,
      exact: true,
    });
  }

  modal(): Locator {
    return this.page.getByRole("dialog");
  }

  nameInput(): Locator {
    return this.modal().locator(ADD_DEPLOYMENT.ids.name);
  }

  typeSelect(): Locator {
    return this.modal().locator(ADD_DEPLOYMENT.ids.type);
  }

  descriptionInput(): Locator {
    return this.modal().locator(ADD_DEPLOYMENT.ids.description);
  }

  /** The modal's confirm control, scoped to the dialog to keep it distinct from
   * the button that opened it. */
  modalSubmitButton(): Locator {
    return this.modal().getByRole("button", {
      name: ADD_DEPLOYMENT.submitButton,
      exact: true,
    });
  }

  modalCancelButton(): Locator {
    return this.modal().getByRole("button", { name: "Cancel", exact: true });
  }

  /** Picks a deployment type without touching the other fields. */
  async selectType(type: string): Promise<void> {
    await this.typeSelect().click();
    await this.page.getByRole("option", { name: type, exact: true }).click();
  }

  async cancel(): Promise<void> {
    await this.modalCancelButton().click();
    await expect(this.modal()).toBeHidden();
  }

  /** Opens the Add Deployment modal and waits for it. */
  async openAddDeploymentModal(): Promise<void> {
    await this.addDeploymentButton().click();
    await expect(this.modal()).toBeVisible();
    // Scoped to the dialog so nothing elsewhere on the page can satisfy it.
    // Substring, not exact: the title is a bare text node inside DialogTitle
    // alongside the description Typography, so the element's full text is
    // "Add New Deployment" + "Create a new deployment environment…" and no
    // element's text equals the title on its own.
    await expect(
      this.modal().getByText(ADD_DEPLOYMENT.dialogTitle),
    ).toBeVisible();
  }

  /**
   * Fills the modal.
   *
   * @param name - Deployment name.
   * @param type - Deployment type option label.
   * @param description - Description text.
   */
  async fillDeployment(
    name: string,
    type: string,
    description: string,
  ): Promise<void> {
    await this.nameInput().fill(name);

    await this.typeSelect().click();
    await this.page.getByRole("option", { name: type, exact: true }).click();

    await this.descriptionInput().fill(description);
  }

  async submit(): Promise<void> {
    await this.modalSubmitButton().click();
  }

  /**
   * The list entry for a deployment, matched on its exact name.
   *
   * Exact rather than substring: every deployment this suite creates shares the
   * `namePrefix`, so a substring match would also hit the entries left by
   * earlier runs. Deliberately not narrowed with `.first()` either — callers
   * assert the count, so an ambiguous match fails loudly instead of silently
   * asserting against whichever entry happens to come first.
   *
   * @param name - Full deployment name.
   * @returns Locator for the matching entry (expected to be unique).
   */
  deploymentEntry(name: string): Locator {
    return this.main().getByText(name, { exact: true });
  }

  /**
   * Every deployment card on the page, for counting them.
   *
   * Counted by the per-card edit control: one per card, and unlike the card's
   * text it cannot be confused with anything inside an expanded card.
   */
  deploymentCards(): Locator {
    return this.main().getByRole("button", {
      name: EDIT_DEPLOYMENT.openButton,
      exact: true,
    });
  }

  /**
   * Collapses an expanded deployment card.
   *
   * @param name - Full deployment name.
   */
  async collapseDeployment(name: string): Promise<void> {
    const summary = this.deploymentSummary(name);
    await summary.click();
    await expect(summary).toHaveAttribute("aria-expanded", "false", {
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  //
  // Pagination, at the foot of the deployments list.
  //

  /** The Rows per page control. Accepts either role, as MUI changed the trigger
   * from `button` to `combobox` across versions. */
  rowsPerPageSelect(): Locator {
    const name = new RegExp(MUI_PAGINATION.rowsPerPageLabel.replace(":", ":?"));
    return this.main()
      .getByRole("combobox", { name })
      .or(this.main().getByRole("button", { name }))
      .first();
  }

  /**
   * Changes the deployments list page size.
   *
   * The option list renders in a portal at the document root, so it is looked up
   * page-wide.
   *
   * @param rows - Page size to choose.
   */
  async selectRowsPerPage(rows: number): Promise<void> {
    await this.rowsPerPageSelect().click();
    await this.page
      .getByRole("option", { name: String(rows), exact: true })
      .click();
  }

  /** Next-page control. Disabled on the last page. */
  nextPageButton(): Locator {
    return this.main().getByRole("button", {
      name: MUI_PAGINATION.nextPageButton,
      exact: true,
    });
  }

  /** Previous-page control. Disabled on the first page. */
  previousPageButton(): Locator {
    return this.main().getByRole("button", {
      name: MUI_PAGINATION.previousPageButton,
      exact: true,
    });
  }

  /** The "1–10 of 87" range text beside the page controls. */
  displayedRows(): Locator {
    return this.main().getByText(MUI_PAGINATION.displayedRowsPattern);
  }

  /**
   * The first row number the list is currently showing.
   *
   * @returns The "from" of "from–to of count", or null when not rendered.
   */
  async displayedFromRow(): Promise<number | null> {
    const match = MUI_PAGINATION.displayedRowsPattern.exec(
      await this.displayedRows().innerText(),
    );
    return match ? Number(match[1]) : null;
  }

  /**
   * Whether a deployment with this exact name is listed, paging to find it.
   *
   * The tab fetches one page at a time and offers no search, so the only way to
   * answer this through the UI is to walk the pages. The page size is raised
   * first to keep that short.
   *
   * Being listed also means being active: a deactivated deployment — what the
   * delete action produces — is filtered out of the list, so a caller checking a
   * fixture is present does not need to check its state separately.
   *
   * @param name - Full deployment name.
   * @returns True when found on any page.
   */
  async isDeploymentListed(name: string): Promise<boolean> {
    const largestPageSize = DEPLOYMENTS_LIST.rowsPerPageOptions.at(-1);
    if (largestPageSize) await this.selectRowsPerPage(largestPageSize);

    // Bounded by the number of pages the list reports, so a control that stops
    // disabling itself cannot spin here forever.
    const total = (await this.displayedTotal()) ?? 0;
    const pages = Math.max(
      1,
      Math.ceil(total / (largestPageSize ?? DEPLOYMENTS_LIST.defaultRowsPerPage)),
    );

    for (let visited = 0; visited < pages; visited += 1) {
      if ((await this.deploymentEntry(name).count()) > 0) return true;

      const next = this.nextPageButton();
      if (!(await next.isEnabled())) return false;

      const from = await this.displayedFromRow();
      await next.click();
      // The rows are refetched, so wait for the range to move before reading the
      // next page — otherwise this would search the same page twice.
      await expect
        .poll(() => this.displayedFromRow(), { timeout: LOAD_TIMEOUT_MS })
        .not.toBe(from);
    }

    return (await this.deploymentEntry(name).count()) > 0;
  }

  /**
   * How many deployments the list holds in total, from the range text.
   *
   * @returns The "of N" of "from–to of N", or null when not rendered.
   */
  async displayedTotal(): Promise<number | null> {
    const match = MUI_PAGINATION.displayedRowsPattern.exec(
      await this.displayedRows().innerText(),
    );
    return match ? Number(match[3]) : null;
  }

  /**
   * The page size the list is currently showing, from the range text.
   *
   * @returns The count of rows on this page, or null when not rendered.
   */
  async displayedRowCount(): Promise<number | null> {
    const match = MUI_PAGINATION.displayedRowsPattern.exec(
      await this.displayedRows().innerText(),
    );
    return match ? Number(match[2]) - Number(match[1]) + 1 : null;
  }

  /**
   * A whole deployment card.
   *
   * Narrowed by *both* the deployment name and the presence of an edit control:
   * the toolbar buttons are named identically on every card, so the card has to
   * be identified first. `.last()` takes the innermost such container — the card
   * itself rather than the list around it.
   *
   * @param name - Full deployment name.
   * @returns Locator for the card.
   */
  deploymentCard(name: string): Locator {
    return this.main()
      .locator("div")
      .filter({ has: this.page.getByText(name, { exact: true }) })
      .filter({
        has: this.page.getByRole("button", {
          name: EDIT_DEPLOYMENT.openButton,
          exact: true,
        }),
      })
      .last();
  }

  /**
   * The edit control on a deployment card.
   *
   * @param name - Full deployment name.
   */
  editDeploymentButton(name: string): Locator {
    return this.deploymentCard(name).getByRole("button", {
      name: EDIT_DEPLOYMENT.openButton,
      exact: true,
    });
  }

  /**
   * Opens the Edit Deployment modal for a deployment.
   *
   * @param name - Full deployment name.
   */
  async openEditDeploymentModal(name: string): Promise<void> {
    await this.editDeploymentButton(name).click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(modal.getByText(EDIT_DEPLOYMENT.dialogTitle)).toBeVisible();
  }

  /**
   * Opens the Add Deployment modal's type select and reads the options it
   * offers.
   *
   * Closes it again with Escape, so the caller is left with the form as it was
   * rather than an open listbox over the fields it still has to fill.
   *
   * Waits for the first option before reading: `allInnerTexts` resolves against
   * whatever matches at that moment and does not retry, so reading straight
   * after the click can return an empty list while the portal mounts.
   *
   * @returns The option labels, in order.
   */
  async typeOptionLabels(): Promise<string[]> {
    await this.typeSelect().click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const labels = await options.allInnerTexts();

    await this.page.keyboard.press("Escape");
    return labels.map((label) => label.trim());
  }

  /** Type select of the Edit Deployment modal. */
  editDeploymentTypeSelect(): Locator {
    return this.modal().locator(EDIT_DEPLOYMENT.ids.type);
  }

  /**
   * Chooses a deployment type in the Edit Deployment modal.
   *
   * @param type - Type option label.
   */
  async selectEditDeploymentType(type: string): Promise<void> {
    await this.chooseProductOption(this.editDeploymentTypeSelect(), type);
  }

  /** Name field of the Edit Deployment modal. */
  editDeploymentNameInput(): Locator {
    return this.modal().locator(EDIT_DEPLOYMENT.ids.name);
  }

  /** Description field of the Edit Deployment modal. */
  editDeploymentDescriptionInput(): Locator {
    return this.modal().locator(EDIT_DEPLOYMENT.ids.description);
  }

  /** The Edit Deployment modal's confirm control. */
  editDeploymentSubmitButton(): Locator {
    return this.modal().getByRole("button", {
      name: EDIT_DEPLOYMENT.submitButton,
      exact: true,
    });
  }

  /**
   * Submits the deployment edit and waits for the PATCH to land.
   *
   * @param projectId - Project the deployment belongs to.
   * @returns The update response.
   */
  async submitDeploymentEdit(projectId: string): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          new RegExp(
            `/projects/${projectId}/deployments/[^/]+$`,
          ).test(new URL(r.url()).pathname) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.editDeploymentSubmitButton().click(),
    ]);
    return response;
  }

  /**
   * The delete control on a deployment card.
   *
   * @param name - Full deployment name.
   */
  deleteDeploymentButton(name: string): Locator {
    return this.deploymentCard(name).getByRole("button", {
      name: DELETE_DEPLOYMENT.openButton,
      exact: true,
    });
  }

  /**
   * Opens the delete confirmation for a deployment.
   *
   * Asserts the dialog names the deployment being deleted — the toolbar controls
   * are identical across cards, so this is what proves the right one was hit.
   *
   * @param name - Full deployment name.
   */
  async openDeleteDeploymentModal(name: string): Promise<void> {
    await this.deleteDeploymentButton(name).click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(modal).toContainText(DELETE_DEPLOYMENT.dialogTitle);
    await expect(modal).toContainText(DELETE_DEPLOYMENT.confirmMessage(name));
  }

  /** Dismisses the delete-deployment confirmation without deleting. */
  async dismissDeleteDeploymentModal(): Promise<void> {
    await this.modal()
      .getByRole("button", {
        name: DELETE_DEPLOYMENT.goBackButton,
        exact: true,
      })
      .click();
    await expect(this.modal()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Confirms the deletion and waits for the PATCH to land.
   *
   * Deleting reuses the update endpoint — it sends `{ active: false }` — so the
   * body is what tells a deletion from an edit.
   *
   * @param projectId - Project the deployment belongs to.
   * @returns The deletion response.
   */
  async confirmDeleteDeployment(projectId: string): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          new RegExp(`/projects/${projectId}/deployments/[^/]+$`).test(
            new URL(r.url()).pathname,
          ) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.modal()
        .getByRole("button", {
          name: DELETE_DEPLOYMENT.confirmButton,
          exact: true,
        })
        .click(),
    ]);
    return response;
  }

  /**
   * The accordion header of a deployment card.
   *
   * MUI's AccordionSummary is the clickable header, so this is what the chevron
   * belongs to — the icon itself is decorative and carries no accessible name.
   *
   * @param name - Full deployment name.
   * @returns Locator for the card's header.
   */
  /**
   * The name on each deployment card, in list order.
   *
   * Read off the page rather than taken from a fixture: deployment names are
   * project data and have changed under this suite before — a project that once
   * had "Production" later carried only timestamped records. A spec that merely
   * needs *a* deployment should ask the page which ones exist.
   *
   * @returns One name per card currently listed.
   */
  async deploymentNames(): Promise<string[]> {
    const headers = this.main().getByRole("button", { expanded: false });
    const texts = await headers.allInnerTexts();
    return texts
      .map((t) => t.split("\n")[0].trim())
      .filter((t) => t.length > 0);
  }

  deploymentSummary(name: string): Locator {
    return this.main()
      .getByRole("button")
      .filter({ has: this.page.getByText(name, { exact: true }) })
      .last();
  }

  /**
   * Expands a deployment card.
   *
   * The card's contents are unmounted while collapsed (`unmountOnExit`), so
   * nothing inside it — the Add Product button included — exists until this
   * runs. Waits on `aria-expanded` rather than on any particular content, since
   * what is inside depends on whether the deployment has products yet.
   *
   * @param name - Full deployment name.
   */
  async expandDeployment(name: string): Promise<void> {
    const summary = this.deploymentSummary(name);
    await expect(summary).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await summary.click();
    await expect(summary).toHaveAttribute("aria-expanded", "true", {
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /** The Add Product button inside an expanded deployment card. */
  addProductButton(): Locator {
    return this.main().getByRole("button", {
      name: ADD_PRODUCT.openButton,
      exact: true,
    });
  }

  /**
   * Opens the Add WSO2 Product modal from an expanded deployment.
   */
  async openAddProductModal(): Promise<void> {
    await this.addProductButton().click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    // Scoped to the dialog: the button that opened it carries the same name and
    // is still in the DOM behind it.
    await expect(modal.getByText(ADD_PRODUCT.dialogTitle)).toBeVisible();
  }

  productNameSelect(): Locator {
    return this.modal().locator(ADD_PRODUCT.ids.productName);
  }

  productVersionSelect(): Locator {
    return this.modal().locator(ADD_PRODUCT.ids.version);
  }

  productCoresInput(): Locator {
    return this.modal().locator(ADD_PRODUCT.ids.cores);
  }

  productTpsInput(): Locator {
    return this.modal().locator(ADD_PRODUCT.ids.tps);
  }

  productDescriptionInput(): Locator {
    return this.modal().locator(ADD_PRODUCT.ids.description);
  }

  /**
   * Chooses a product in the Add WSO2 Product modal.
   *
   * @param product - Product option label.
   */
  async selectProduct(product: string): Promise<void> {
    await this.chooseProductOption(this.productNameSelect(), product);
  }

  /**
   * Chooses a version in the Add WSO2 Product modal.
   *
   * The select stays disabled until a product is chosen — its options are
   * fetched per product — so this must follow `selectProduct`.
   *
   * @param version - Version option label.
   */
  async selectProductVersion(version: string): Promise<void> {
    await this.chooseProductOption(this.productVersionSelect(), version);
  }

  /** Dismisses the Add WSO2 Product modal without creating anything. */
  async cancelProductModal(): Promise<void> {
    await this.modal()
      .getByRole("button", { name: ADD_PRODUCT.cancelButton, exact: true })
      .click();
    await expect(this.modal()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Chooses an update level in the Add New Update section.
   *
   * @param level - Update level to choose.
   */
  async selectNewUpdateLevel(level: string): Promise<void> {
    await this.chooseProductOption(this.newUpdateLevelSelect(), level);
  }

  /** Dismisses the delete-product confirmation without deleting. */
  async dismissDeleteProductModal(): Promise<void> {
    await this.modal()
      .getByRole("button", { name: DELETE_PRODUCT.goBackButton, exact: true })
      .click();
    await expect(this.modal()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Fills the Add WSO2 Product form.
   *
   * Version is populated from the chosen product, so the two selects have to be
   * driven in order. Both are MUI Selects whose options render in a portal at
   * the document root, hence the page-wide option lookup.
   *
   * @param product - Product option label.
   * @param version - Version option label.
   * @param cores - Core count.
   * @param tps - Transactions per second.
   * @param description - Free-text description.
   */
  async fillProduct(
    product: string,
    version: string,
    cores: string,
    tps: string,
    description: string,
  ): Promise<void> {
    await this.selectProduct(product);
    await this.selectProductVersion(version);
    await this.productCoresInput().fill(cores);
    await this.productTpsInput().fill(tps);
    await this.productDescriptionInput().fill(description);
  }

  /**
   * Opens a Select in the product modal and picks an option.
   *
   * @param select - The Select to open.
   * @param option - Exact option label.
   */
  private async chooseProductOption(
    select: Locator,
    option: string,
  ): Promise<void> {
    await expect(select).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await select.click();
    await this.page
      .getByRole("option", { name: option, exact: true })
      .click({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * A product listed under an expanded deployment.
   *
   * Located by its row's edit control, whose accessible name embeds the product
   * label. Only the expanded card's products are in the DOM at all, since
   * collapsed cards unmount their contents.
   *
   * @param productLabel - Product label as the list renders it.
   * @returns Locator for the row's edit control.
   */
  deploymentProduct(productLabel: string): Locator {
    return this.main().getByRole("button", {
      name: ADD_PRODUCT.rowEditButton(productLabel),
      exact: true,
    });
  }

  /** The product modal's confirm control, scoped to the dialog. */
  productSubmitButton(): Locator {
    return this.modal().getByRole("button", {
      name: ADD_PRODUCT.submitButton,
      exact: true,
    });
  }

  /**
   * The delete control on a listed product.
   *
   * @param productLabel - Product label as the list renders it.
   */
  deleteProductButton(productLabel: string): Locator {
    return this.main().getByRole("button", {
      name: DELETE_PRODUCT.openButton(productLabel),
      exact: true,
    });
  }

  /**
   * Opens the delete confirmation for a listed product.
   *
   * Asserts the dialog names the product and version being deleted, so a
   * mis-aimed click cannot pass as the right one.
   *
   * @param productLabel - Product label as the list renders it.
   * @param version - Version as the dialog renders it.
   */
  async openDeleteProductModal(
    productLabel: string,
    version: string,
  ): Promise<void> {
    await this.deleteProductButton(productLabel).first().click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(modal).toContainText(DELETE_PRODUCT.dialogTitle);
    await expect(modal).toContainText(
      DELETE_PRODUCT.confirmMessage(productLabel, version),
    );
  }

  /**
   * Confirms the product deletion and waits for the PATCH to land.
   *
   * Deleting reuses the product update endpoint — it sends `{ active: false }`
   * — so the body is what tells a deletion from an edit.
   *
   * @returns The deletion response.
   */
  async confirmDeleteProduct(): Promise<Response> {
    return this.saveManageProduct(
      this.modal().getByRole("button", {
        name: DELETE_PRODUCT.confirmButton,
        exact: true,
      }),
    );
  }

  //
  // Manage Product modal.
  //

  /**
   * Opens the Manage Product modal from a listed product's edit control.
   *
   * @param productLabel - Product label as the list renders it.
   */
  async openManageProductModal(productLabel: string): Promise<void> {
    await this.deploymentProduct(productLabel).first().click();

    const modal = this.modal();
    await expect(modal).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(modal.getByText(MANAGE_PRODUCT.dialogTitle)).toBeVisible();
  }

  /** Closes the Manage Product modal without saving. */
  async closeManageProductModal(): Promise<void> {
    await this.modal()
      .getByRole("button", { name: MANAGE_PRODUCT.closeButton, exact: true })
      .click();
    await expect(this.modal()).toBeHidden({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * A tab of the Manage Product modal.
   *
   * @param label - Tab label.
   */
  manageProductTab(label: string): Locator {
    return this.modal().getByRole("tab", { name: label, exact: true });
  }

  /**
   * Switches tab within the Manage Product modal.
   *
   * @param label - Tab to open.
   */
  async openManageProductTab(label: string): Promise<void> {
    await this.manageProductTab(label).click();
    await expect(this.manageProductTab(label)).toHaveAttribute(
      "aria-selected",
      "true",
      { timeout: LOAD_TIMEOUT_MS },
    );
  }

  /** Description field on the modal's Product Details tab. */
  manageProductDescriptionInput(): Locator {
    return this.modal().locator(MANAGE_PRODUCT.ids.description);
  }

  /** The modal's footer save control, shown on the Product Details tab. */
  manageProductSaveButton(): Locator {
    return this.modal().getByRole("button", {
      name: MANAGE_PRODUCT.saveButton,
      exact: true,
    });
  }

  /** Update Level select in the Add New Update section. */
  newUpdateLevelSelect(): Locator {
    return this.modal().locator(MANAGE_PRODUCT.ids.updateLevel);
  }

  /** Applied On date field, which takes a "YYYY-MM-DD" value. */
  newUpdateAppliedOnInput(): Locator {
    return this.modal().locator(MANAGE_PRODUCT.ids.appliedOn);
  }

  /** Description field of the Add New Update section. */
  newUpdateDescriptionInput(): Locator {
    return this.modal().locator(MANAGE_PRODUCT.ids.updateDescription);
  }

  /**
   * Fills the Add New Update form.
   *
   * The level select offers only levels above the product's current one, so a
   * level already reached will not be on the list.
   *
   * @param level - Update level to add.
   * @param appliedOn - Date applied, as "YYYY-MM-DD".
   * @param description - Free-text description.
   */
  async fillNewUpdate(
    level: string,
    appliedOn: string,
    description: string,
  ): Promise<void> {
    await this.chooseProductOption(this.newUpdateLevelSelect(), level);
    await this.newUpdateAppliedOnInput().fill(appliedOn);
    await this.newUpdateDescriptionInput().fill(description);
  }

  /** Core Count field on the modal's Product Details tab. */
  manageProductCoresInput(): Locator {
    return this.modal().locator(MANAGE_PRODUCT.ids.cores);
  }

  /** TPS field on the modal's Product Details tab. */
  manageProductTpsInput(): Locator {
    return this.modal().locator(MANAGE_PRODUCT.ids.tps);
  }

  /** The "Current Update Level:" readout's label, which renders only once the
   * product has an update history. */
  currentUpdateLevelLabel(): Locator {
    return this.modal().getByText(MANAGE_PRODUCT.currentLevelLabel, {
      exact: true,
    });
  }

  /**
   * The level the readout reports, e.g. "U12".
   *
   * Scoped to the readout rather than matched page-wide: the same "U12" is also
   * rendered as the entry's own badge in the list below, so an unscoped locator
   * resolves to two elements. `.last()` picks the innermost container holding the
   * label — the readout box itself, whose only other child is the value.
   *
   * @param level - Expected level, without the "U" prefix.
   * @returns Locator for the value.
   */
  currentUpdateLevel(level: string): Locator {
    return this.modal()
      .locator("div")
      .filter({ has: this.currentUpdateLevelLabel() })
      .last()
      .getByText(MANAGE_PRODUCT.currentLevelValue(level), { exact: true });
  }

  /**
   * The edit control on an update-history entry.
   *
   * @param level - The entry's update level, without the "U" prefix.
   */
  updateRowEditButton(level: string): Locator {
    return this.modal().getByRole("button", {
      name: MANAGE_PRODUCT.updateRow.editButton(level),
      exact: true,
    });
  }

  /**
   * The delete control on an update-history entry. Deleting is immediate — there
   * is no confirmation step, unlike deleting a product or a deployment.
   *
   * @param level - The entry's update level, without the "U" prefix.
   */
  updateRowDeleteButton(level: string): Locator {
    return this.modal().getByRole("button", {
      name: MANAGE_PRODUCT.updateRow.deleteButton(level),
      exact: true,
    });
  }

  /**
   * The description field of an entry's inline edit form.
   *
   * Matched by an exact label, which is what separates it from the Add New
   * Update section's field ("Description" vs "Description (Optional)") — neither
   * carries an id.
   */
  editUpdateDescriptionInput(): Locator {
    return this.modal().getByLabel(MANAGE_PRODUCT.updateRow.descriptionLabel, {
      exact: true,
    });
  }

  /** The save control of an entry's inline edit form. */
  editUpdateSaveButton(): Locator {
    return this.modal().getByRole("button", {
      name: MANAGE_PRODUCT.updateRow.saveButton,
      exact: true,
    });
  }

  /**
   * Reads the levels the Add New Update section offers.
   *
   * Only levels above the product's current one are listed, so this is how the
   * filtering rule is observed.
   *
   * @returns The option labels, in order.
   */
  async newUpdateLevelOptions(): Promise<string[]> {
    await this.newUpdateLevelSelect().click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const labels = await options.allInnerTexts();

    await this.page.keyboard.press("Escape");
    return labels.map((label) => label.trim());
  }

  /** The modal's footer add control, shown on the Update History tab. */
  addUpdateButton(): Locator {
    return this.modal().getByRole("button", {
      name: MANAGE_PRODUCT.addUpdateButton,
      exact: true,
    });
  }

  /**
   * Saves a change made in the Manage Product modal and waits for the PATCH.
   *
   * Both footer buttons save through the same endpoint, so the caller says which
   * control to press and asserts on the body to tell the two apart.
   *
   * @param control - The footer button to click.
   * @returns The update response.
   */
  async saveManageProduct(control: Locator): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/deployments\/[^/]+\/products\/[^/]+$/.test(
            new URL(r.url()).pathname,
          ) && r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      control.click(),
    ]);
    return response;
  }

  /**
   * Submits the product and waits for the create POST to land.
   *
   * Waits whatever the status, then leaves the caller to assert on it: a
   * rejected create that never matched the predicate would time out with no clue
   * as to why.
   *
   * @returns The create response.
   */
  async submitProduct(): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/deployments\/[^/]+\/products$/.test(new URL(r.url()).pathname) &&
          r.request().method() === "POST",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.productSubmitButton().click(),
    ]);
    return response;
  }

  //
  // Documents, inside an expanded deployment card.
  //

  /** The Documents section of the expanded deployment. */
  documentsSection(): Locator {
    return this.main().getByText(DEPLOYMENT_DOCUMENTS.section, { exact: true });
  }

  /** The Upload control that opens the Add Document dialog. */
  uploadDocumentButton(): Locator {
    return this.main().getByRole("button", {
      name: DEPLOYMENT_DOCUMENTS.uploadButton,
      exact: true,
    });
  }

  /** The upload dialog. Matched by its own title, which in deployment mode is
   * "Add Document" rather than the case tab's "Upload Attachment". */
  uploadDocumentDialog(): Locator {
    return this.page
      .getByRole("dialog")
      .filter({ hasText: DEPLOYMENT_DOCUMENTS.uploadModal.title });
  }

  /** A document row, found by the file name it lists. */
  documentRow(name: string): Locator {
    return this.main().getByText(name, { exact: false });
  }

  /**
   * Uploads a file to the expanded deployment's Documents section.
   *
   * Sets the file on the dropzone's hidden input rather than clicking "Choose
   * file", which would open the OS picker that Playwright cannot drive.
   *
   * @param filePath - Path to the file to upload.
   * @param documentName - Name to store it under.
   * @returns The successful POST response, for asserting on the wire.
   */
  async uploadDocument(
    filePath: string,
    documentName: string,
  ): Promise<Response> {
    await expect(this.uploadDocumentButton()).toBeEnabled({
      timeout: LOAD_TIMEOUT_MS,
    });
    await this.uploadDocumentButton().click();

    const dialog = this.uploadDocumentDialog();
    await expect(dialog).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    await dialog.locator('input[type="file"]').setInputFiles(filePath);
    await dialog
      .getByLabel(DEPLOYMENT_DOCUMENTS.uploadModal.nameField)
      .fill(documentName);

    const confirm = dialog.getByRole("button", {
      name: DEPLOYMENT_DOCUMENTS.uploadModal.confirmButton,
      exact: true,
    });
    await expect(confirm).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/deployments\/[^/]+\/attachments$/.test(new URL(r.url()).pathname) &&
          r.request().method() === "POST",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      confirm.click(),
    ]);
    return response;
  }
}
