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
import { CHANGE_REQUEST_CREATE } from "../utils/selectors";

/** The three change types the create form offers, in on-screen order. */
export type ChangeType = "Normal" | "Standard" | "Emergency";

/**
 * Page object for `/operations/change-requests/new`. Two fields are required:
 * the change type ("What type of change is required?" -- exactly Normal,
 * Standard or Emergency, no default) and the Subject. Impact comes
 * pre-selected, so the happy path only needs to pick a type, fill Subject
 * and submit. There is no delete endpoint for change requests, so
 * every CR this creates is a permanent staging record — the subject must
 * always be E2E-tagged (see `e2eChangeRequestSubject`).
 */
export class ChangeRequestCreatePage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(CHANGE_REQUEST_CREATE.path);
    await expect(
      this.page.getByRole("heading", { name: CHANGE_REQUEST_CREATE.heading }),
    ).toBeVisible();
  }

  /** The "What type of change is required?" radio group. */
  typeGroup(): Locator {
    return this.page.getByRole("group", { name: /what type of change is required/i });
  }

  typeRadio(type: ChangeType): Locator {
    return this.typeGroup().getByRole("radio", { name: new RegExp(`^${type}`) });
  }

  async selectType(type: ChangeType): Promise<void> {
    await this.typeRadio(type).check();
  }

  /** The "Customer Approval" checkbox (a real checkbox, unchecked by default). */
  customerApprovalCheckbox(): Locator {
    return this.page.getByRole("checkbox", { name: "Customer Approval" });
  }

  /** The "Customer Review" checkbox (a real checkbox, unchecked by default). */
  customerReviewCheckbox(): Locator {
    return this.page.getByRole("checkbox", { name: "Customer Review" });
  }

  /** The one line saying why an Emergency change's two customer boxes are disabled and unticked. */
  emergencyCustomerStepsNote(): Locator {
    return this.page.getByText("Emergency changes proceed without customer approval or review.");
  }

  /** MUI's required-field asterisk (a thin-space + `*` folded into the
   * computed accessible name) makes an exact "Subject" match find nothing —
   * this anchored, marker-tolerant regex matches either way. */
  subjectField(): Locator {
    return this.page.getByRole("textbox", { name: /^Subject\s*\*?$/ });
  }

  /** Reads a pre-selected enum dropdown's current visible value (Impact,
   * Priority) without opening it. */
  selectValue(label: string): Locator {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return this.page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) });
  }

  // -------------------------------------------------------------------------
  // Customer Project / Deployments / Deployment products, the read-only
  // Customer Group, Category and the Communication area.
  // -------------------------------------------------------------------------

  /** The "Customer Project" picker (a combobox that searches projects). */
  projectField(): Locator {
    return this.page.getByRole("combobox", { name: "Customer Project" });
  }

  /** Opens the project picker and picks the project called `name`. */
  async selectProject(name: string): Promise<void> {
    await this.projectField().click();
    await this.page.getByRole("option", { name }).click();
    await expect(this.projectField()).toHaveValue(name);
  }

  /** Clears the picked project through the picker's clear button. */
  async clearProject(): Promise<void> {
    await this.projectField().hover();
    await this.page.getByRole("button", { name: "Clear" }).first().click();
  }

  deploymentsField(): Locator {
    return this.page.getByRole("combobox", { name: "Deployments" });
  }

  /** The read-only, derived "Deployment products" field. */
  deploymentProductsField(): Locator {
    return this.page.getByLabel("Deployment products");
  }

  /** The chips shown inside a multi-select (or the read-only products field). */
  chipsOf(field: Locator): Locator {
    return field.locator("xpath=ancestor::div[contains(@class,'MuiInputBase-root')][1]").locator(".MuiChip-label");
  }

  /** Opens a multi-select and toggles each named option, then closes it. */
  async toggleOptions(field: Locator, names: string[]): Promise<void> {
    await field.click();
    for (const name of names) await this.page.getByRole("option", { name, exact: true }).click();
    await field.press("Escape");
  }

  async selectDeployments(names: string[]): Promise<void> {
    await this.toggleOptions(this.deploymentsField(), names);
  }

  /** The names a multi-select currently offers (opens and closes it). */
  async optionsOf(field: Locator): Promise<string[]> {
    await field.click();
    // Scoped to the open autocomplete popup: the rich-text editors on this
    // form carry native <option>s of their own.
    const options = this.page.locator(".MuiAutocomplete-popper").getByRole("option");
    await expect(options.first()).toBeVisible();
    const names = await options.allTextContents();
    await field.press("Escape");
    return names;
  }

  /**
   * The read-only "Customer Group": the chosen project's registered contacts,
   * shown as chips in a locked text field (not a picker).
   */
  customerGroupField(): Locator {
    return this.page.getByLabel("Customer Group");
  }

  /** The contacts the Customer Group lists, as chip labels. */
  customerGroupChips(): Locator {
    return this.chipsOf(this.customerGroupField());
  }

  /** The "Category" dropdown (pre-selected to Other). */
  categoryField(): Locator {
    return this.page.getByRole("combobox", { name: "Category" });
  }

  async selectCategory(label: string): Promise<void> {
    await this.categoryField().click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
  }

  commentField(): Locator {
    return this.page.getByLabel("Additional comments (Customer visible)");
  }

  workNotesField(): Locator {
    return this.page.getByLabel("Work notes");
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create change request" });
  }

  /** Picks the change type, fills the subject and submits. Returns once the app has
   * navigated to the new CR's detail page (`/operations/change-requests/:id`).
   * The id segment must not match the literal "new" of this very create
   * route — a bare `[^/]+$` is satisfied by `/operations/change-requests/new`
   * itself, which would let this assertion pass instantly on a still-pending
   * (or failed) submit, before the app ever navigates to the created
   * record's real id. */
  async fillSubjectAndSubmit(
    subject: string,
    type: ChangeType = "Normal",
    opts: { customerApproval?: boolean; customerReview?: boolean } = {},
  ): Promise<void> {
    await this.selectType(type);
    await this.subjectField().fill(subject);
    if (opts.customerApproval) await this.customerApprovalCheckbox().check();
    if (opts.customerReview) await this.customerReviewCheckbox().check();
    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/operations\/change-requests\/(?!new(?:[/?#]|$))[^/]+$/, {
      timeout: 15_000,
    });
  }
}
