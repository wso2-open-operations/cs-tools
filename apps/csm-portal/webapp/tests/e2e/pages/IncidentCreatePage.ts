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
import { INCIDENT_CREATE } from "../utils/selectors";

/**
 * Page object for `/operations/incidents/new`. Unlike change requests
 * (Subject-only), the backend hard-requires Short description, Category,
 * Subcategory, Contact type, Impact, Urgency, Caller, and Service (see
 * `validateCreateIncidentBody` in incidents.go) — Caller auto-fills to the
 * signed-in user, everything else needs an explicit pick.
 */
export class IncidentCreatePage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(INCIDENT_CREATE.path);
    await expect(
      this.page.getByRole("heading", { name: INCIDENT_CREATE.heading }),
    ).toBeVisible();
  }

  shortDescriptionField(): Locator {
    return this.page.getByLabel("Short description");
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create incident" });
  }

  /** Opens a MUI Select by its field label and clicks the named option —
   * Category, Subcategory, Contact type, Impact, Urgency all use this.
   * Anchored regex, not `exact: true` — every one of these is a required
   * field, and MUI's FormControl appends a required-field marker to the
   * label. Confirmed live it's not a plain " *": the actual separator is
   * U+2009 (thin space), not U+0020, so `\s*` (which covers U+2009) is used
   * rather than a literal space. A loose (non-exact) match isn't safe
   * either — "Category" is a substring of "Subcategory", so it'd match
   * both; the `^...$` anchors rule that out. */
  async selectOption(label: string, optionLabel: string): Promise<void> {
    await this.requiredSelectLocator(label).click();
    await this.page.getByRole("option", { name: optionLabel, exact: true }).click();
  }

  /** The same required-field-marker-tolerant locator `selectOption` clicks
   * through — exposed separately for specs that need to open a select and
   * inspect its options without picking one (e.g. asserting Subcategory's
   * options change with Category, rather than driving a full pick). */
  requiredSelectLocator(label: string): Locator {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return this.page.getByLabel(new RegExp(`^${escaped}\\s*\\*?$`));
  }

  /** Types into the Service async type-ahead and picks the first real
   * result. There's no seeded/known service name in staging to search for,
   * so callers pass a short, broad query (e.g. a single common letter) —
   * mirrors how a user would actually search when they don't know the
   * exact catalog entry. */
  async pickService(query: string): Promise<void> {
    const input = this.page.getByLabel("Service", { exact: true });
    await input.click();
    await input.fill(query);
    const option = this.page.getByRole("option").first();
    await option.waitFor({ state: "visible", timeout: 10_000 });
    await option.click();
  }

  /** Fills every backend-required field (short description, the five
   * classification selects, and a Service pick) and submits. Caller is left
   * alone — it's already auto-filled to the signed-in user. Returns once
   * the app has navigated to the new incident's detail page
   * (`/operations/incidents/:id`). */
  async fillRequiredFieldsAndSubmit(opts: {
    shortDescription: string;
    category: string;
    subcategory: string;
    contactType: string;
    impact: string;
    urgency: string;
    serviceQuery: string;
  }): Promise<void> {
    await this.shortDescriptionField().fill(opts.shortDescription);
    await this.selectOption("Category", opts.category);
    await this.selectOption("Subcategory", opts.subcategory);
    await this.selectOption("Contact type", opts.contactType);
    await this.selectOption("Impact", opts.impact);
    await this.selectOption("Urgency", opts.urgency);
    await this.pickService(opts.serviceQuery);

    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/operations\/incidents\/[^/]+$/, {
      timeout: 15_000,
    });
  }
}
