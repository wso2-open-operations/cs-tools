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

/**
 * Page object for `/operations/change-requests/new`. Subject is the only
 * required field (Type/Category/Impact/Risk come pre-selected with sensible
 * defaults — see CreateChangeRequestPage.tsx), so the happy path only needs
 * to fill Subject and submit. There is no delete endpoint for change
 * requests, so every CR this creates is a permanent staging record — the
 * subject must always be E2E-tagged (see `e2eChangeRequestSubject`).
 */
export class ChangeRequestCreatePage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(CHANGE_REQUEST_CREATE.path);
    await expect(
      this.page.getByRole("heading", { name: CHANGE_REQUEST_CREATE.heading }),
    ).toBeVisible();
  }

  /** Not `exact` — MUI's required-field asterisk (`aria-hidden` but still
   * folded into the computed accessible name) makes an exact "Subject"
   * match find nothing; the loose match is unambiguous (only one field on
   * this form is labelled anything starting with "Subject"). */
  subjectField(): Locator {
    return this.page.getByLabel("Subject");
  }

  /** Reads a pre-selected enum dropdown's current visible value (Type,
   * Category, Impact, Risk, Priority) without opening it. */
  selectValue(label: string): Locator {
    return this.page.getByLabel(label, { exact: true });
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create change request" });
  }

  /** Fills the only required field and submits. Returns once the app has
   * navigated to the new CR's detail page (`/operations/change-requests/:id`). */
  async fillSubjectAndSubmit(subject: string): Promise<void> {
    await this.subjectField().fill(subject);
    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/operations\/change-requests\/[^/]+$/, {
      timeout: 15_000,
    });
  }
}
