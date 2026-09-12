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

/**
 * Page object for `/operations/change-requests/:id`
 * (`CsmChangeRequestDetailPage.tsx`). "Request approval" is only rendered
 * when the CR's `legalNextStates` includes `"assess"` (data-driven — see
 * `canRequestApproval` in the source); it will be absent for a CR already
 * past that stage.
 */
export class ChangeRequestDetailPage {
  constructor(private readonly page: Page) {}

  /**
   * A freshly-created CR isn't always retrievable the instant we navigate to
   * it — the real DEV-SN backend can lag between the create write and the
   * record becoming readable. Retry the navigation (full reload) until the
   * stable "Back to change requests" button actually renders, rather than
   * failing on the first attempt.
   */
  async goto(id: string): Promise<void> {
    await expect(async () => {
      await this.page.goto(`/operations/change-requests/${id}`);
      await expect(this.backButton()).toBeVisible({ timeout: 3_000 });
    }).toPass({ timeout: 45_000, intervals: [1_000, 2_000, 3_000, 5_000] });
  }

  backButton(): Locator {
    return this.page.getByRole("button", { name: "Back to change requests" });
  }

  requestApprovalButton(): Locator {
    return this.page.getByRole("button", { name: "Request approval" });
  }

  async requestApproval(): Promise<void> {
    await this.requestApprovalButton().click();
  }

  editButton(): Locator {
    return this.page.getByRole("button", { name: "Edit", exact: true });
  }

  async openEditDialog(): Promise<void> {
    await this.editButton().click();
    await expect(
      this.page.getByRole("dialog").getByRole("heading", { name: "Edit change request" }),
    ).toBeVisible();
  }

  editDialog(): Locator {
    return this.page.getByRole("dialog");
  }

  /**
   * The "Planned start" `DateTimePicker` field group inside the edit dialog.
   * MUI X renders these as a sectioned `role="group"`, not a plain `<input>`
   * — see `fillDateTimeField` on `CaseDetailPage` for the fill approach (not
   * duplicated here; callers needing to set this should reuse that pattern
   * or drive it directly via keyboard input on this locator).
   */
  plannedStartGroup(): Locator {
    return this.editDialog().getByRole("group", { name: /^Planned start/ });
  }

  // "Customer approved"/"Customer reviewed" are deliberately not editable
  // controls in this dialog — see EditChangeRequestDialog.tsx's doc comment.

  saveButton(): Locator {
    return this.editDialog().getByRole("button", { name: /^(Save|Saving…)$/ });
  }

  async saveEdit(): Promise<void> {
    await this.saveButton().click();
  }

  // ── Approvals ────────────────────────────────────────────────────────────

  /** Expands an approval-stage accordion by its displayed stage label (e.g.
   * "Assess", "Authorize", "Customer Approval" — see
   * `ChangeRequestApprovals.tsx`; a repeated stage gets a "(N of M)" suffix
   * appended). */
  async expandApprovalStage(stageLabel: string): Promise<void> {
    await this.page
      .locator(".MuiAccordionSummary-root", { hasText: stageLabel })
      .click();
  }

  /** Approve/Reject buttons only render for the signed-in user's own
   * pending ("REQUESTED") approval row — scope by the approver's own display
   * name if more than one row is expanded at once. */
  approveButton(approverName?: string): Locator {
    const scope = approverName
      ? this.page.locator(".MuiAccordionDetails-root", { hasText: approverName })
      : this.page;
    return scope.getByRole("button", { name: "Approve" });
  }

  rejectButton(approverName?: string): Locator {
    const scope = approverName
      ? this.page.locator(".MuiAccordionDetails-root", { hasText: approverName })
      : this.page;
    return scope.getByRole("button", { name: "Reject" });
  }

  async approve(approverName?: string): Promise<void> {
    await this.approveButton(approverName).click();
  }

  async reject(approverName?: string): Promise<void> {
    await this.rejectButton(approverName).click();
  }

  // ── Comments ─────────────────────────────────────────────────────────────

  async openComposer(): Promise<void> {
    const opener = this.page.getByRole("button", { name: "Add a comment…" });
    if (await opener.isVisible().catch(() => false)) await opener.click();
  }

  internalNoteSwitch(): Locator {
    return this.page.getByRole("switch", { name: "Internal note" });
  }

  commentEditor(): Locator {
    return this.page.getByTestId("case-description-editor");
  }

  commentSubmitButton(): Locator {
    return this.page.getByRole("button", { name: /Send to customer|Save work note/ });
  }

  async addComment(text: string, opts: { internal?: boolean } = {}): Promise<void> {
    await this.openComposer();
    const wantInternal = !!opts.internal;
    const isChecked = await this.internalNoteSwitch().isChecked();
    if (isChecked !== wantInternal) await this.internalNoteSwitch().click();
    await this.commentEditor().click();
    await this.commentEditor().fill(text);
    await this.commentSubmitButton().click();
  }

  // ── Attachments ──────────────────────────────────────────────────────────

  async uploadAttachment(filePath: string): Promise<void> {
    await this.page.locator('input[type="file"]').first().setInputFiles(filePath);
  }

  async downloadAttachment(filename: string): Promise<void> {
    await this.page.getByRole("button", { name: `Download ${filename}` }).click();
  }
}
