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
 * (`CsmChangeRequestDetailPage.tsx`). "Request Approval" is only rendered when
 * the CR's `legalNextStates` includes `"assess"` (data-driven, via
 * `ChangeRequestActionBar`'s own `legalNextStates` filtering); it will be
 * absent for a CR already past that stage. It is a plain, direct state PATCH
 * (`{state: "assess"}`) that starts the CR's approval flow (Peer -> CAB for
 * Normal, one CAB approval for Emergency, straight to Scheduled for Standard).
 * There is deliberately no "Schedule" button: a CR is moved to Scheduled
 * automatically by its CAB approval (or, when it requires customer
 * approval, by the customer answering in the customer portal) -- see
 * `scheduleButton()`, which exists only so specs can assert its absence.
 * Staff never record a customer's approval or review, so there is no action for
 * it either: `answerForCustomerWording()` exists only so specs can assert that.
 */
export class ChangeRequestDetailPage {
  constructor(readonly page: Page) {}

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

  requestApprovalButton(): Locator {
    return this.page.getByRole("button", { name: "Request Approval" });
  }

  async requestApproval(): Promise<void> {
    await this.requestApprovalButton().click();
  }

  /** Never expected to be visible: Scheduled is reached by approval, not by a
   * manual action. Matches a button or a menu entry containing "Schedule". */
  scheduleButton(): Locator {
    return this.page
      .getByRole("button", { name: /^schedule/i })
      .or(this.page.getByRole("menuitem", { name: /^schedule/i }));
  }

  /** The lifecycle stepper's current step (`aria-current="step"`). */
  currentStep(): Locator {
    return this.lifecycleStepper().locator('[aria-current="step"]');
  }

  /** Header note while a stage is waiting, e.g. "Awaiting CAB Approval",
   * "Awaiting customer approval" or "Awaiting customer review". */
  blockingReason(): Locator {
    return this.page.getByText(/^Awaiting .+ (Approval|review)$/i);
  }

  /**
   * Never expected to be visible: staff never record a customer's approval or review.
   * Matches any text, button or menu entry that words an engineer answering for the
   * customer ("Bypass customer approval" / "Bypass customer review", the retired
   * "Record customer approval"). Only meaningful with the "Change state" menu open
   * for its entries (a closed menu renders none).
   */
  answerForCustomerWording(): Locator {
    const wording = /bypass|record customer|on (their|the customer's) behalf/i;
    return this.page
      .getByText(wording)
      .or(this.page.getByRole("button", { name: wording }))
      .or(this.page.getByRole("menuitem", { name: wording }));
  }

  /** Every entry of the open "Change state" menu. */
  menuItems(): Locator {
    return this.page.getByRole("menuitem");
  }

  /** Opens the "Change state" menu and waits for its entries. */
  async openChangeStateMenu(): Promise<void> {
    await this.changeStateButton().click();
    await expect(this.menuItems().first()).toBeVisible();
  }

  /** Closes the "Change state" menu again. */
  async closeChangeStateMenu(): Promise<void> {
    await this.page.keyboard.press("Escape");
    await expect(this.menuItems()).toHaveCount(0);
  }

  /** Review's forward move when the CR requires customer review. */
  sendForCustomerReviewButton(): Locator {
    return this.page.getByRole("button", { name: "Send for customer review" });
  }

  /**
   * The primary "Close" button: Review's forward move when no customer review is
   * required. It is never there at Customer Review, where closing is the customer's
   * own answer, given in the Customer Portal.
   */
  closeButton(): Locator {
    return this.page.getByRole("button", { name: "Close", exact: true });
  }

  /** Read-only Yes/No on the Approval tab beside a label such as
   * "Customer approval required" / "Customer review required" ("Not applicable" on an Emergency
   * change, which acts without customer consent). */
  flagValue(label: string): Locator {
    return this.page
      .getByText(label, { exact: true })
      .locator("xpath=..")
      .getByText(/^(Yes|No|Not applicable)$/);
  }

  /**
   * The lifecycle stepper's stages, in order: the eleven of the customer portal's
   * workflow (Customer Approval / Customer Review only when ticked). Each one's
   * text is its label plus a visually-hidden status, "Review, done".
   */
  stepLabels(): Locator {
    return this.lifecycleStepper().getByRole("listitem");
  }

  /** One stage of the stepper by its label ("Review" never matches "Customer Review"). */
  stage(label: string): Locator {
    return this.stepLabels().filter({ hasText: new RegExp(`^${label}, `) });
  }

  /** The Customer Approval / Customer Review checkboxes in the edit dialog. */
  editCustomerApprovalCheckbox(): Locator {
    return this.editDialog().getByRole("checkbox", { name: "Customer Approval" });
  }

  editCustomerReviewCheckbox(): Locator {
    return this.editDialog().getByRole("checkbox", { name: "Customer Review" });
  }

  /** Explanatory notice shown to the CR's creator in the Approvals card. */
  creatorApprovalNotice(): Locator {
    return this.page.getByRole("alert").filter({ hasText: /you created this change request/i });
  }

  /** The Overview cell labelled `label` (e.g. "Customer Project", "Deployments"). */
  overviewCell(label: string): Locator {
    return this.page.getByText(label, { exact: true }).locator("xpath=..");
  }

  /** The chips an Overview cell shows (Deployments / Deployment products / Customer group). */
  overviewChips(label: string): Locator {
    return this.overviewCell(label).locator(".MuiChip-label");
  }

  /** The Clone button in the page header. */
  cloneButton(): Locator {
    return this.page.getByRole("button", { name: /clone/i });
  }

  /** The "Stage" cell of the approvals table row for a named approver
   * ("Peer Approval" | "CAB Approval"; "ECAB Approval" only on an older Emergency change). */
  approverStage(approverName: string): Locator {
    return this.approverRow(approverName).getByRole("cell").first();
  }

  /** The overflow ("Change state") menu trigger, which holds Cancel change. */
  changeStateButton(): Locator {
    return this.page.getByRole("button", { name: "Change state" });
  }

  cancelChangeMenuItem(): Locator {
    return this.page.getByRole("menuitem", { name: "Cancel change" });
  }

  /** "Roll back" -- the failed-review off-ramp; a destructive overflow-menu entry. */
  rollbackMenuItem(): Locator {
    return this.page.getByRole("menuitem", { name: "Roll back" });
  }

  /** The reason dialog the destructive transitions (Roll back, Cancel change) open. */
  reasonDialog(): Locator {
    return this.page.getByRole("dialog");
  }

  /**
   * Header note while a proposed time waits for WSO2's answer: the change is waiting for WSO2, so the "Awaiting ..."
   * note of `blockingReason()` is not shown. The note exists only when a customer is recorded as the proposer
   * ("Waiting for WSO2 to respond to the customer's proposed time"); a stored time nobody is recorded as having proposed
   * leaves the header at "Awaiting Customer Approval", since there is no proposal to respond to. This locator matches
   * the one wording there is, and `neutralProposalWaitingReason()` the wording that must never appear.
   */
  proposalWaitingReason(): Locator {
    return this.page.getByText(/^Waiting for WSO2 to respond to the (customer's )?proposed time$/);
  }

  /** The header note when the proposer is recorded. */
  customerProposalWaitingReason(): Locator {
    return this.page.getByText(/^Waiting for WSO2 to respond to the customer's proposed time$/);
  }

  /** The header note that no page says any more: "Waiting for WSO2 to respond to the proposed time" (nobody proposed it). */
  neutralProposalWaitingReason(): Locator {
    return this.page.getByText(/^Waiting for WSO2 to respond to the proposed time$/);
  }

  /**
   * The banner under the stepper (a named region): "The customer proposed a new time" when the proposer is recorded,
   * "A time is stored on this change request" when nobody is. This locator matches either.
   */
  proposalBanner(): Locator {
    return this.page.getByRole("region", { name: /^(The customer proposed a new time|A time is stored on this change request)$/ });
  }

  /** The banner when the proposer is recorded. */
  customerProposalBanner(): Locator {
    return this.page.getByRole("region", { name: "The customer proposed a new time", exact: true });
  }

  /** The banner when nobody is recorded as the proposer: a time is stored, nothing waits for an answer, the window is labelled "Stored time". */
  neutralProposalBanner(): Locator {
    return this.page.getByRole("region", { name: "A time is stored on this change request", exact: true });
  }

  /** The banner's primary answer (never the bar's: Accept lives only in the banner). */
  acceptProposedTimeButton(): Locator {
    return this.proposalBanner().getByRole("button", { name: "Accept proposed time" });
  }

  /** The banner's other answer; the action bar carries a button of the same name (the outlined `authorize`). */
  proposeDifferentTimeButton(): Locator {
    return this.proposalBanner().getByRole("button", { name: "Propose a different time" });
  }

  /** The confirmation behind Accept ("Accept the proposed time?"). */
  acceptDialog(): Locator {
    return this.page.getByRole("dialog").filter({ has: this.page.getByRole("heading", { name: "Accept the proposed time?" }) });
  }

  acceptDialogConfirm(): Locator {
    return this.acceptDialog().getByRole("button", { name: "Accept proposed time", exact: true });
  }

  /** The Re-schedule dialog in counter mode (its heading is "Propose a different time"). */
  counterDialog(): Locator {
    return this.page.getByRole("dialog").filter({ has: this.page.getByRole("heading", { name: "Propose a different time" }) });
  }

  /** The counter dialog's submit: "Propose this time" (a different window) or "Decline proposed time" (the current time kept). */
  counterSubmit(label: "Propose this time" | "Decline proposed time"): Locator {
    return this.counterDialog().getByRole("button", { name: label, exact: true });
  }

  /**
   * The page's own alert for an answer dialog that was closed for the engineer because the change is no longer what it showed
   * (a refusal with one of the stale-answer codes): the backend's words, then "The page now shows the current state." It takes
   * focus once the dialog is gone.
   */
  staleAnswerNotice(): Locator {
    return this.page.getByRole("alert").filter({ hasText: "The page now shows the current state." });
  }

  /** "Re-schedule" -- the outlined button beside the primary action in Customer Approval. */
  rescheduleButton(): Locator {
    return this.page.getByRole("button", { name: "Re-schedule", exact: true });
  }

  /** The Re-schedule dialog (its heading is "Re-schedule this change?"; with a customer's proposal waiting it is `counterDialog()`). */
  rescheduleDialog(): Locator {
    return this.page.getByRole("dialog").filter({ has: this.page.getByRole("heading", { name: "Re-schedule this change?" }) });
  }

  /** The dialog's submit button (the bar's own "Re-schedule" is behind the modal). */
  rescheduleSubmit(): Locator {
    return this.rescheduleDialog().getByRole("button", { name: "Re-schedule", exact: true });
  }

  /**
   * Types a wall-clock value (in the signed-in user's time zone, as the picker
   * shows it) into one of the Re-schedule dialog's MUI date-time pickers
   * ("Planned start" | "Planned end"): focuses the Month section, then types `MMDDYYYYhhmm` + AM/PM, which the field auto-advances through.
   */
  async fillRescheduleWindow(
    label: "Planned start" | "Planned end",
    value: { month: number; day: number; year: number; hour12: number; minute: number; pm: boolean },
    dialog: Locator = this.rescheduleDialog(),
  ): Promise<void> {
    const two = (n: number): string => String(n).padStart(2, "0");
    const group = dialog.getByRole("group", { name: new RegExp(`^${label}`) });
    // Focus the first section (Month) explicitly: a click on the group's centre
    // would land on the Year section and shift every typed digit.
    await group.getByRole("spinbutton", { name: "Month" }).click();
    await this.page.keyboard.type(
      `${two(value.month)}${two(value.day)}${value.year}${two(value.hour12)}${two(value.minute)}${value.pm ? "PM" : "AM"}`,
      { delay: 30 },
    );
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

  /** Scope fields inside the edit dialog (same labels as the create form). */
  editProjectField(): Locator {
    return this.editDialog().getByRole("combobox", { name: "Customer Project" });
  }

  editDeploymentsField(): Locator {
    return this.editDialog().getByRole("combobox", { name: "Deployments" });
  }

  /** The read-only Customer Group (the project's registered contacts) inside the edit dialog. */
  editCustomerGroupField(): Locator {
    return this.editDialog().getByLabel("Customer Group");
  }

  editDeploymentProductsField(): Locator {
    return this.editDialog().getByLabel("Deployment products");
  }

  editCategoryField(): Locator {
    return this.editDialog().getByRole("combobox", { name: "Category" });
  }

  /** The chips inside one of the edit dialog's multi-selects / read-only products field. */
  editChipsOf(field: Locator): Locator {
    return field.locator("xpath=ancestor::div[contains(@class,'MuiInputBase-root')][1]").locator(".MuiChip-label");
  }

  /** Opens an edit-dialog multi-select, toggles the named options, closes it. */
  async editToggleOptions(field: Locator, names: string[]): Promise<void> {
    await field.click();
    for (const name of names) await this.page.getByRole("option", { name, exact: true }).click();
    await field.press("Escape");
  }

  saveButton(): Locator {
    return this.editDialog().getByRole("button", { name: /^(Save|Saving…)$/ });
  }

  async saveEdit(): Promise<void> {
    await this.saveButton().click();
  }

  // ── Approvals ────────────────────────────────────────────────────────────
  //
  // ChangeRequestApprovals.tsx renders one flat table (Stage/State/Approver/
  // Assignment group/Comments/Created/Approved on/Actions) with every
  // approver from every stage shown together — not the collapsible
  // per-stage accordion cards an earlier UI revision used (see that
  // component's own history in entity-service's CLAUDE.md, "Change
  // requests" section: "a full UI redesign ... now renders as one flat
  // table"). Scope by table row, not an accordion class.

  /** The approvals table row for a named approver (e.g. "Jane Doe"). The
   * same person can sit on more than one stage (Peer Approval and CAB
   * Approval both list the seeded users), so pass `stage` ("Peer Approval" |
   * "CAB Approval") to pick one stage's row. */
  approverRow(approverName: string, stage?: string): Locator {
    const rows = this.page.getByRole("row", { name: approverName });
    return stage ? rows.filter({ has: this.page.getByRole("cell", { name: stage, exact: true }) }) : rows;
  }

  /** That approver's status chip text ("Requested" | "Approved" |
   * "Rejected" | "Cancelled" | ...). */
  approverStatus(approverName: string, stage?: string): Locator {
    return this.approverRow(approverName, stage).locator(".MuiChip-label");
  }

  /** The Assignment group of an approver's row: a link-button that opens the
   * group (its members), or the project's registered contacts for a customer
   * stage. Pass `stage` to pick one stage's row. */
  groupLink(approverName: string, groupName: string, stage?: string): Locator {
    return this.approverRow(approverName, stage).getByRole("button", { name: `View members of ${groupName}`, exact: true });
  }

  /** The group dialog the Assignment group link opens, titled with the group's name. */
  groupDialog(title: string): Locator {
    return this.page.getByRole("dialog", { name: title, exact: true });
  }

  /** Approve/Reject buttons only render for the signed-in user's own
   * pending ("REQUESTED") approval row — scope by the approver's own display
   * name when more than one row is on the page at once. */
  approveButton(approverName?: string, stage?: string): Locator {
    const scope = approverName ? this.approverRow(approverName, stage) : this.page;
    return scope.getByRole("button", { name: "Approve" });
  }

  rejectButton(approverName?: string, stage?: string): Locator {
    const scope = approverName ? this.approverRow(approverName, stage) : this.page;
    return scope.getByRole("button", { name: "Reject" });
  }

  async approve(approverName?: string, stage?: string): Promise<void> {
    await this.approveButton(approverName, stage).click();
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
