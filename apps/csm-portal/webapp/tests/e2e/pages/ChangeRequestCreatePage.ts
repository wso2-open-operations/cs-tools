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
 * required field (Type/Impact come pre-selected with sensible defaults —
 * see CreateChangeRequestPage.tsx), so the happy path only needs to fill
 * Subject and submit. There is no delete endpoint for change requests, so
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

  /** MUI's required-field asterisk (a thin-space + `*` folded into the
   * computed accessible name) makes an exact "Subject" match find nothing —
   * this anchored, marker-tolerant regex matches either way. */
  subjectField(): Locator {
    return this.page.getByRole("textbox", { name: /^Subject\s*\*?$/ });
  }

  /** Reads a pre-selected enum dropdown's current visible value (Type,
   * Impact, Priority) without opening it. */
  selectValue(label: string): Locator {
    const escaped = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return this.page.getByRole("combobox", { name: new RegExp(`^${escaped}\\s*\\*?$`) });
  }

  createButton(): Locator {
    return this.page.getByRole("button", { name: "Create change request" });
  }

  /** Fills the only required field and submits. Returns once the app has
   * navigated to the new CR's detail page (`/operations/change-requests/:id`).
   * The id segment must not match the literal "new" of this very create
   * route — a bare `[^/]+$` is satisfied by `/operations/change-requests/new`
   * itself, which would let this assertion pass instantly on a still-pending
   * (or failed) submit, before the app ever navigates to the created
   * record's real id. */
  async fillSubjectAndSubmit(subject: string): Promise<void> {
    await this.subjectField().fill(subject);
    await expect(this.createButton()).toBeEnabled();
    await this.createButton().click();
    await expect(this.page).toHaveURL(/\/operations\/change-requests\/(?!new(?:[/?#]|$))[^/]+$/, {
      timeout: 15_000,
    });
  }
}
