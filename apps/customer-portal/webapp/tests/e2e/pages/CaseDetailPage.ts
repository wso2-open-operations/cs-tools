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
  CASE_COMMENT_INPUT,
  CASE_DETAIL,
  CASE_DETAILS_PANEL,
  CASE_ESCALATION,
} from "../utils/selectors";
import { isSuccess } from "../utils/caseFlows";

/** How long to allow for a case's detail page to resolve — the header is
 * skeletonised while the case loads, well beyond the 5s default. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the case detail page
 * (`/projects/:projectId/support/cases/:caseId`).
 *
 * Only the state-change actions are modelled here so far. Which buttons the
 * action row renders depends on the case's current status — see
 * `getAvailableCaseActions` in src/features/support/utils/support.ts.
 */
export class CaseDetailPage {
  constructor(private readonly page: Page) {}

  /** The app's <main> region. Everything on this page is scoped to it so the
   * surrounding chrome — notably a promo banner with its own "Close" dismiss
   * control — cannot make a locator ambiguous. */
  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /** The "Close" action button. Present while the case is open; once closed the
   * action row swaps it for "Open Related Case". */
  closeButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_DETAIL.closeButton,
      exact: true,
    });
  }

  /**
   * Opens a case's detail page directly.
   *
   * @param projectId - Project the case belongs to.
   * @param caseId - Case sysid.
   */
  async open(projectId: string, caseId: string): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.pathSegment}/${caseId}`,
    );
    // The header renders once the case resolves; the case number is the first
    // field guaranteed to be present for every case.
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Opens a security report analysis. It reuses the same header as a case, so
   * the field locators below apply unchanged.
   *
   * @param projectId - Project the SRA belongs to.
   * @param sraId - SRA sysid.
   */
  async openSecurityReportAnalysis(
    projectId: string,
    sraId: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.sraPathSegment}/${sraId}`,
    );
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Opens a service request. It reuses the same header as a case, so the field
   * locators below apply unchanged — bar severity, which service requests do not
   * carry.
   *
   * @param projectId - Project the request belongs to.
   * @param serviceRequestId - Service request sysid.
   */
  async openServiceRequest(
    projectId: string,
    serviceRequestId: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.serviceRequestPathSegment}/${serviceRequestId}`,
    );
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /**
   * Opens an announcement. Like SRAs and service requests it reuses the case
   * detail header, so the field locators below apply unchanged.
   *
   * @param projectId - Project the announcement belongs to.
   * @param announcementId - Announcement sysid.
   */
  async openAnnouncement(
    projectId: string,
    announcementId: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/${CASE_DETAIL.announcementPathSegment}/${announcementId}`,
    );
    await expect(this.caseNumber()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  //
  // Header fields. None carry ids, test ids or labels, so rather than guessing
  // at positions in the header row these match on the *shape* of their content —
  // which is both stable against markup changes and self-documenting.
  //

  /** The ServiceNow case number, e.g. CS0441157. */
  caseNumber(): Locator {
    return this.main().getByText(/^CS\d+$/);
  }

  /** The WSO2 case id, e.g. AUTOMATIONTESTCUSSUB-42. */
  wso2CaseId(pattern: RegExp): Locator {
    return this.main().getByText(pattern);
  }

  /** The severity chip, e.g. "S1" or "S4(Query)". */
  severityChip(severity: string): Locator {
    return this.main().getByText(severity, { exact: true });
  }

  /** The case state shown beside the number, e.g. "Open". */
  stateLabel(state: string): Locator {
    return this.main().getByText(state, { exact: true });
  }

  /** The case subject — the header's `variant="h6"` heading. */
  subject(): Locator {
    return this.main().getByRole("heading").first();
  }

  /** A comment on the Activity tab, matched as a substring. */
  comment(text: string): Locator {
    return this.main().getByText(text, { exact: false }).first();
  }

  //
  // Details tab.
  //

  /** Switches to the Details tab and waits for its first section. */
  async openDetailsTab(
    expectedSection: string = CASE_DETAILS_PANEL.sections.caseOverview,
  ): Promise<void> {
    await this.page
      .getByRole("tab", { name: CASE_DETAILS_PANEL.tab, exact: true })
      .click();
    // The first section's title names the record type: a case shows "Case
    // Overview", an engagement "Engagement Overview" (CaseDetailsDetailsPanel).
    // Waiting on the case wording for an engagement never resolves — it looks
    // like the tab is stuck loading.
    await expect(this.detailsText(expectedSection)).toHaveCount(1, {
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * Text within the page, for asserting a Details-tab label or section is shown.
   *
   * Returns the full match set rather than narrowing with `.first()`: several
   * labels legitimately repeat — the header already shows a status and a
   * severity, for instance — so callers assert on the count being non-zero,
   * which states "this is displayed" without pretending the page has only one.
   *
   * @param text - Exact label or heading text.
   * @returns Locator for every match.
   */
  detailsText(text: string): Locator {
    return this.main().getByText(text, { exact: true });
  }

  //
  // Activity tab — the comment box.
  //

  /** The comment editor. A Lexical contenteditable, so it is typed into rather
   * than filled. */
  commentEditor(): Locator {
    return this.main().getByTestId(CASE_DETAIL.commentEditorTestId);
  }

  sendCommentButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_COMMENT_INPUT.sendButton,
      exact: true,
    });
  }

  /**
   * Types a comment and sends it, waiting for the POST to land.
   *
   * @param text - Comment body.
   * @returns The create response, for the caller to assert on.
   */
  async addComment(text: string): Promise<Response> {
    const editor = this.commentEditor();
    await expect(editor).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await editor.click();
    await editor.pressSequentially(text);

    // The send control stays disabled until the editor holds submittable
    // content, so this also confirms the text registered with Lexical.
    await expect(this.sendCommentButton()).toBeEnabled();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/comments$/.test(new URL(r.url()).pathname) &&
          r.request().method() === "POST",
      ),
      this.sendCommentButton().click(),
    ]);
    return response;
  }

  confirmDialog(): Locator {
    return this.page.getByRole("dialog");
  }

  confirmButton(): Locator {
    return this.confirmDialog().getByRole("button", {
      name: CASE_DETAIL.confirmDialog.confirmButton,
      exact: true,
    });
  }

  /** Any element rendering the closed-status text — the header chip. */
  closedStatusChip(): Locator {
    return this.main()
      .getByText(CASE_DETAIL.closedStatus, { exact: true })
      .first();
  }

  /**
   * Clicks "Close" and confirms the dialog, then waits for the PATCH to
   * succeed.
   *
   * The button stays disabled until the case-states metadata resolves (the
   * action needs a state key to patch with), so this waits for it to be
   * enabled rather than clicking into a no-op.
   *
   * @returns The successful PATCH response for the caller to assert on.
   */
  async closeCase(): Promise<Response> {
    await expect(this.closeButton()).toBeEnabled();
    await this.closeButton().click();

    // Closing is confirmation-gated; the dialog must appear before confirming.
    await expect(this.confirmDialog()).toBeVisible();
    await expect(
      this.page.getByText(CASE_DETAIL.confirmDialog.title),
    ).toBeVisible();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          r.url().includes("/cases/") &&
          r.request().method() === "PATCH" &&
          isSuccess(r.status()),
      ),
      this.confirmButton().click(),
    ]);
    return response;
  }

  /** The Escalate Case action in the header row.
   *
   * Scoped to <main> for the same reason the Close action is: the page chrome
   * outside it carries its own controls. */
  escalateButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_ESCALATION.button,
      exact: true,
    });
  }

  /** The De-escalate action, which exists only once a case is escalated —
   * `showDeescalateButton` is driven by the case's own `isEscalated`, so its
   * presence is the detail page's own account of the escalation, independent of
   * the toast. */
  deescalateButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_ESCALATION.deescalateButton,
      exact: true,
    });
  }

  /** The escalation modal. Addressed by its title rather than as the only
   * dialog, so it cannot be confused with the state-change confirm dialog. */
  escalateDialog(): Locator {
    return this.page
      .getByRole("dialog")
      .filter({ hasText: CASE_ESCALATION.modal.titlePattern });
  }

  /** A level chip inside the modal, e.g. "EL0" for current or "EL1" for next.
   *
   * Both chips render the same shape of text, so this matches exactly to keep
   * "EL1" from also matching a hypothetical "EL1x", and the caller scopes by
   * which value it expects rather than by position. */
  escalationLevelChip(level: string): Locator {
    return this.escalateDialog().getByText(level, { exact: true });
  }

  /** The mandatory reason textarea. */
  escalationReasonInput(): Locator {
    return this.escalateDialog().getByLabel(
      CASE_ESCALATION.modal.reasonField,
    );
  }

  /** Confirms the escalation. Disabled until the reason is non-empty. */
  confirmEscalationButton(): Locator {
    return this.escalateDialog().getByRole("button", {
      name: CASE_ESCALATION.modal.confirmButton,
    });
  }

  /**
   * Opens the escalation modal and waits for it to be ready to fill.
   *
   * Waits for the reason field rather than the dialog alone: the modal mounts
   * with its level chips still resolving, and a caller that asserts on them the
   * moment the dialog appears races that render.
   */
  async openEscalateModal(): Promise<void> {
    await expect(this.escalateButton()).toBeEnabled();
    await this.escalateButton().click();
    await expect(this.escalateDialog()).toBeVisible();
    await expect(this.escalationReasonInput()).toBeVisible();
  }

  /**
   * Fills the reason and confirms, returning the escalation's POST response.
   *
   * Asserts on the wire rather than on the toast alone — the modal reports
   * success from the mutation callback, so the response is the only direct
   * evidence the backend recorded the escalation.
   *
   * @param reason - Reason text; the confirm button stays disabled while empty.
   * @returns The successful POST response, for asserting on the request body.
   */
  async confirmEscalation(reason: string): Promise<Response> {
    await this.escalationReasonInput().fill(reason);
    await expect(this.confirmEscalationButton()).toBeEnabled();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/escalations$/.test(r.url()) &&
          r.request().method() === "POST" &&
          isSuccess(r.status()),
      ),
      this.confirmEscalationButton().click(),
    ]);
    return response;
  }

  /** The header chip reporting the case's escalation level, e.g.
   * "Escalated to EL1". */
  escalatedChip(level: string): Locator {
    return this.main()
      .getByText(CASE_ESCALATION.escalatedChip(level), { exact: true })
      .first();
  }

  /**
   * Switches to the Escalation tab and waits for the history panel.
   *
   * Waits on the heading rather than the tab's selected state: the panel
   * fetches its own records, so a caller asserting on a level the moment the
   * tab activates races that request.
   */
  async openEscalationTab(): Promise<void> {
    await this.page.getByRole("tab", { name: CASE_ESCALATION.tab }).click();
    await expect(
      this.detailsText(CASE_ESCALATION.history.heading),
    ).toHaveCount(1, { timeout: LOAD_TIMEOUT_MS });
  }

  /** Level chips within the escalation history, e.g. "EL1".
   *
   * Returns every match: a single escalation renders EL0 as both the record's
   * previous level and the pinned initial state, so the count is meaningful
   * per level and the caller asserts on it. */
  escalationHistoryLevel(level: number): Locator {
    return this.main().getByText(CASE_ESCALATION.history.levelLabel(level), {
      exact: true,
    });
  }

  /** The de-escalation modal, matched by its title. */
  deescalateDialog(): Locator {
    return this.page
      .getByRole("dialog")
      .filter({ hasText: CASE_ESCALATION.deescalateModal.title });
  }

  /** The optional reason textarea in the de-escalation modal. */
  deescalationReasonInput(): Locator {
    return this.deescalateDialog().getByLabel(
      CASE_ESCALATION.deescalateModal.reasonField,
    );
  }

  confirmDeescalationButton(): Locator {
    return this.deescalateDialog().getByRole("button", {
      name: CASE_ESCALATION.deescalateModal.confirmButton,
    });
  }

  /**
   * Opens the de-escalation modal and waits for it to be ready.
   *
   * The button is permission-gated and rendered disabled inside a tooltip when
   * the user may not de-escalate, so this waits for it to be enabled rather
   * than clicking into a no-op.
   */
  async openDeescalateModal(): Promise<void> {
    await expect(this.deescalateButton()).toBeEnabled({
      timeout: LOAD_TIMEOUT_MS,
    });
    await this.deescalateButton().click();
    await expect(this.deescalateDialog()).toBeVisible();
    await expect(this.deescalationReasonInput()).toBeVisible();
  }

  /**
   * Confirms the de-escalation, optionally with a reason.
   *
   * @param reason - Optional reason; omitted entirely from the payload when
   *   blank, which is what the modal itself does.
   * @returns The successful POST response.
   */
  async confirmDeescalation(reason?: string): Promise<Response> {
    if (reason) await this.deescalationReasonInput().fill(reason);
    await expect(this.confirmDeescalationButton()).toBeEnabled();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/escalations$/.test(r.url()) &&
          r.request().method() === "POST" &&
          isSuccess(r.status()),
      ),
      this.confirmDeescalationButton().click(),
    ]);
    return response;
  }
}
