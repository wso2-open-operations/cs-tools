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
 * (`CsmChangeRequestDetailPage.tsx`). "Move to Assess" is only rendered when
 * the CR's `legalNextStates` includes `"assess"` (data-driven, via
 * `ChangeRequestActionBar`'s own `legalNextStates` filtering); it will be
 * absent for a CR already past that stage. New -> Assess is a plain, direct
 * state PATCH (`{state: "assess"}`) like every other forward transition in
 * this bar — it used to be modeled as a special "approval request" action
 * (`{requestApproval: true}`, labeled "Request approval"), which was
 * backwards relative to the real ServiceNow process; that's fixed now, but
 * the label/method names below were kept in sync with the source rather than
 * left pointing at the old wording.
 */
export class ChangeRequestDetailPage {
  constructor(private readonly page: Page) {}

  /**
   * A freshly-created CR isn't always retrievable the instant we navigate to
   * it — the real DEV-SN backend can lag between the create write and the
   * record becoming readable. Retry the navigation (full reload) until the
   * page has genuinely loaded the record, rather than failing on the first
   * attempt. The lifecycle stepper (always rendered once a CR resolves,
   * regardless of its number/state) is the readiness signal — there is no
   * "Back to change requests" control on this page (it opens as a tab
   * alongside the dashboard, like a case does; confirmed absent anywhere in
   * CsmChangeRequestDetailPage.tsx — an earlier version of this page object
   * waited on one that never existed in the current tab-strip UI, which
   * silently turned every `goto()` call into a 45s timeout).
   */
  async goto(id: string): Promise<void> {
    await expect(async () => {
      await this.page.goto(`/operations/change-requests/${id}`);
      await expect(this.lifecycleStepper()).toBeVisible({ timeout: 3_000 });
    }).toPass({ timeout: 45_000, intervals: [1_000, 2_000, 3_000, 5_000] });
  }

  lifecycleStepper(): Locator {
    return this.page.getByRole("list", { name: "Change request lifecycle" });
  }

  moveToAssessButton(): Locator {
    return this.page.getByRole("button", { name: "Move to Assess" });
  }

  async moveToAssess(): Promise<void> {
    await this.moveToAssessButton().click();
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
  //
  // ChangeRequestApprovals.tsx renders one flat table (State/Approver/
  // Assignment group/Comments/Created/Approved on/Actions) with every
  // approver from every stage shown together — not the collapsible
  // per-stage accordion cards an earlier UI revision used (see that
  // component's own history in entity-service's CLAUDE.md, "Change
  // requests" section: "a full UI redesign ... now renders as one flat
  // table"). Scope by table row, not an accordion class.

  /** The approvals table row for a named approver (e.g. "Jane Doe"). */
  approverRow(approverName: string): Locator {
    return this.page.getByRole("row", { name: approverName });
  }

  /** That approver's status chip text ("Requested" | "Approved" |
   * "Rejected" | "Cancelled" | ...). */
  approverStatus(approverName: string): Locator {
    return this.approverRow(approverName).locator(".MuiChip-label");
  }

  /** Approve/Reject buttons only render for the signed-in user's own
   * pending ("REQUESTED") approval row — scope by the approver's own display
   * name when more than one row is on the page at once. */
  approveButton(approverName?: string): Locator {
    const scope = approverName ? this.approverRow(approverName) : this.page;
    return scope.getByRole("button", { name: "Approve" });
  }

  rejectButton(approverName?: string): Locator {
    const scope = approverName ? this.approverRow(approverName) : this.page;
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
