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
import { CASE_DETAIL, CHANGE_REQUEST_DETAILS } from "../utils/selectors";

/** How long to allow for the page to load behind the shell and the project's features. */
const LOAD_TIMEOUT_MS = 60_000;

const { buttons, currentMarker, notes, propose, rejectConfirm } = CHANGE_REQUEST_DETAILS;

/**
 * Page object for a change request's detail page, as a customer uses it: the
 * answer buttons, the confirmation before a reject, the Propose New Time dialog
 * and the lifecycle panel that says where the change is.
 *
 * No assertions about the *application's* behaviour here (the one `expect` is
 * the load gate in {@link open}); a spec decides what it expects to see.
 */
export class ChangeRequestDetailsPage {
  constructor(private readonly page: Page) {}

  /**
   * Opens the detail page by its address and waits for the change request's
   * number to be on screen, i.e. for the page to hold the change rather than
   * its loading skeleton.
   *
   * @param projectId - The project the change request belongs to (or, for a
   * spec that probes another customer's access, the viewer's own project).
   * @param changeRequestId - The change request's id.
   * @param number - Its number, e.g. `CHG-FIXED-007`, which marks the page loaded.
   */
  async open(
    projectId: string,
    changeRequestId: string,
    number: string,
  ): Promise<void> {
    await this.page.goto(
      `/projects/${projectId}/operations/change-requests/${changeRequestId}`,
    );
    await this.waitForNumber(number);
  }

  /**
   * Waits for the page to show a change request's number.
   *
   * @param number - The change request's number.
   */
  async waitForNumber(number: string): Promise<void> {
    await expect(this.page.getByText(number, { exact: true })).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * The page's own message when the change request could not be loaded: the
   * generic one (a 403, a failure), or the plain not-found one that a change
   * request which is not shared with the viewer gets (a 404 is exactly what a
   * missing one answers, so the page says neither more nor less).
   */
  loadError(): Locator {
    return this.page
      .getByText("Could not load change request details.")
      .or(this.page.getByText("This change request was not found. It may not have been shared with you."));
  }

  /**
   * Opens the detail page and waits for it to SETTLE, whichever way: the change
   * request's number (it loaded) or the load error (it did not). For a spec that
   * asks "what does a viewer who may not have this get?", where the number
   * alone would wait a minute for a page that is never coming.
   *
   * @param projectId - The project in the address.
   * @param changeRequestId - The change request's id.
   * @param number - Its number.
   * @returns Whether the change request loaded.
   */
  async openAndSettle(
    projectId: string,
    changeRequestId: string,
    number: string,
  ): Promise<{ loaded: boolean }> {
    await this.page.goto(
      `/projects/${projectId}/operations/change-requests/${changeRequestId}`,
    );
    const loaded = this.page.getByText(number, { exact: true });
    await expect(loaded.or(this.loadError())).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
    return { loaded: await loaded.isVisible() };
  }

  /**
   * An answer button by its name — Approve, Reject, Propose New Time, Successful
   * or Unsuccessful ({@link buttons}). Exact, so "Reject" never matches the
   * confirmation's "Reject change request".
   *
   * @param name - The button's label.
   */
  button(name: string): Locator {
    return this.page.getByRole("button", { name, exact: true });
  }

  /** The three buttons of Customer Approval. */
  approvalButtons(): Locator {
    return this.button(buttons.approve)
      .or(this.button(buttons.reject))
      .or(this.button(buttons.proposeNewTime));
  }

  /** The two buttons of Customer Review. */
  reviewButtons(): Locator {
    return this.button(buttons.successful).or(this.button(buttons.unsuccessful));
  }

  /** Every button by which a customer answers, Customer Approval's or Customer Review's. */
  answerButtons(): Locator {
    return this.approvalButtons().or(this.reviewButtons());
  }

  /**
   * The name of the stage the lifecycle panel marks "Current".
   *
   * The panel prints each stage's name in a paragraph and, for the current one
   * only, a small "Current" marker beside it; the marker's grandparent holds the
   * name. A locator rather than a string, so `toHaveText` retries while the page
   * refetches after an answer.
   */
  currentStage(): Locator {
    return this.page
      .getByText(currentMarker, { exact: true })
      .locator("xpath=../../p");
  }

  /**
   * A banner (success or error) whose text contains `message`.
   *
   * Both are an MUI Alert, i.e. role "alert". They disappear after five seconds,
   * so assert one straight after the action that raises it.
   *
   * @param message - The banner's text, or a pattern for it.
   */
  banner(message: string | RegExp): Locator {
    return this.page.getByRole("alert").filter({ hasText: message });
  }

  /** The page's own heading: the change request's title (where focus goes after an answer). */
  heading(): Locator {
    return this.page.getByRole("heading", { level: 5 }).first();
  }

  /** The group the answer buttons sit in (Customer Review's is named by its question). */
  answerGroup(name: string): Locator {
    return this.page.getByRole("group", { name, exact: true });
  }

  /** The note beside a Propose New Time that is off because the change is on hold. */
  holdNote(): Locator {
    return this.page.getByText(notes.onHold, { exact: true });
  }

  /** The note that stays on the page while a proposed time waits for WSO2's review. */
  internalReviewNote(): Locator {
    return this.page.getByRole("status").filter({ hasText: notes.internalReview });
  }

  /** "Back to Change Requests": the way back to the list. */
  backButton(): Locator {
    return this.page.getByRole("button", { name: "Back to Change Requests", exact: true });
  }

  /** The confirmation shown before a reject / an "unsuccessful". */
  rejectDialog(title: string): Locator {
    return this.page.getByRole("dialog", { name: title });
  }

  /** The Confirm button of the reject confirmation, by its label. */
  rejectConfirmButton(label: string): Locator {
    return this.rejectDialog(this.rejectDialogTitleFor(label)).getByRole(
      "button",
      { name: label, exact: true },
    );
  }

  /** The Propose New Implementation Time dialog. */
  proposeDialog(): Locator {
    return this.page.getByRole("dialog", { name: propose.title });
  }

  /** The Proposed start field of the Propose dialog. */
  proposedStart(): Locator {
    return this.proposeDialog().getByLabel(propose.startLabel);
  }

  /** The Proposed end field of the Propose dialog. */
  proposedEnd(): Locator {
    return this.proposeDialog().getByLabel(propose.endLabel);
  }

  /** The Submit Proposal button. */
  submitProposalButton(): Locator {
    return this.proposeDialog().getByRole("button", {
      name: propose.submit,
      exact: true,
    });
  }

  /**
   * The time zone the dialog says it reads the two fields in
   * ("Times are in your time zone: Asia/Colombo.").
   *
   * @returns The IANA zone name.
   */
  async proposeTimeZone(): Promise<string> {
    const text = await this.proposeDialog()
      .getByText(/Times are in your time zone: /)
      .innerText();
    const match = /time zone: ([^\s]+?)\./.exec(text);
    if (!match) throw new Error(`no time zone found in "${text}"`);
    return match[1];
  }

  /**
   * Types a window into the open Propose dialog.
   *
   * The end is typed AFTER the start on purpose: while the customer has not
   * edited the end, the dialog moves it with every start change (keeping the
   * current duration); typing the end last is what makes the window the one the
   * caller asked for.
   *
   * @param start - `YYYY-MM-DDTHH:mm`, in the dialog's time zone.
   * @param end - `YYYY-MM-DDTHH:mm`, in the dialog's time zone.
   */
  async fillProposedWindow(start: string, end: string): Promise<void> {
    await this.proposedStart().fill(start);
    await this.proposedEnd().fill(end);
  }

  /** The confirmation's title for the button label it carries. */
  private rejectDialogTitleFor(label: string): string {
    return label === rejectConfirm.reviewConfirm
      ? rejectConfirm.reviewTitle
      : rejectConfirm.approvalTitle;
  }

  /** The page's main region, for assertions about what the page as a whole shows. */
  main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }
}
