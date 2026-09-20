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
import { CASE_DETAIL, GET_HELP_BUTTON, NOVERA_CHAT } from "../utils/selectors";
import { SideNavPage } from "./SideNavPage";
import { projectPathPattern } from "../utils/ids";

/** How long to allow for the shell, the chat page and its first response. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for the Novera chat entry point — the describe-issue page reached
 * from Get Help when the assistant is enabled.
 */
export class NoveraChatPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /**
   * Opens a project and clicks Get Help, which reaches this page only when the
   * assistant is on — otherwise it goes to the create-case form.
   *
   * @param projectId - Project to start from.
   */
  async openViaGetHelp(projectId: string): Promise<void> {
    const sideNav = new SideNavPage(this.page);
    await sideNav.open(projectId);

    await this.page
      .getByRole("button", { name: GET_HELP_BUTTON, exact: true })
      .click();

    await expect(this.page).toHaveURL(
      projectPathPattern(projectId, NOVERA_CHAT.describeIssue.pathSegment),
      { timeout: LOAD_TIMEOUT_MS },
    );

    // The page's own prompt, not just the URL: the URL changes before the route
    // swaps, so this is what shows the page actually rendered.
    await expect(this.heading()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  /** The page's prompt. */
  heading(): Locator {
    return this.main().getByText(NOVERA_CHAT.describeIssue.heading, {
      exact: true,
    });
  }

  /** The issue textarea, located by its example placeholder. */
  issueInput(): Locator {
    return this.main().getByPlaceholder(
      NOVERA_CHAT.describeIssue.inputPlaceholder,
    );
  }

  /**
   * The submit control.
   *
   * Disabled until the text is non-empty, and while the deployment products load
   * — so a caller waits for it rather than clicking straight after typing.
   */
  submitButton(): Locator {
    return this.main().getByRole("button", {
      name: NOVERA_CHAT.describeIssue.submitButton,
      exact: true,
    });
  }

  /**
   * The Create Case control on the conversation.
   *
   * Rendered twice — beside the chat input and in the escalation banner — with
   * the same label, so this takes the first rather than failing strict mode.
   */
  createCaseButton(): Locator {
    return this.main()
      .getByRole("button", {
        name: NOVERA_CHAT.conversation.createCaseButton,
        exact: true,
      })
      .first();
  }

  /**
   * Leaves the conversation for the case form.
   *
   * The conversation travels as router state, so nothing about it appears in the
   * URL — the form simply opens pre-seeded.
   *
   * @param projectId - Project the conversation belongs to.
   */
  async openCreateCase(projectId: string): Promise<void> {
    await expect(this.createCaseButton()).toBeEnabled({
      timeout: LOAD_TIMEOUT_MS,
    });
    await this.createCaseButton().click();

    await expect(this.page).toHaveURL(
      projectPathPattern(projectId, NOVERA_CHAT.conversation.createCasePathSegment),
      { timeout: LOAD_TIMEOUT_MS },
    );
  }

  //
  // Chat history, and resuming a conversation.
  //

  /** The chat history search box. */
  historySearchInput(): Locator {
    return this.main().getByPlaceholder(NOVERA_CHAT.history.searchPlaceholder);
  }

  /** The "Showing X of Y chat sessions" bar. */
  historyResultsBar(): Locator {
    return this.main().getByText(NOVERA_CHAT.history.resultsCountPattern);
  }

  /**
   * The totals the results bar reports.
   *
   * @returns shown/total, or null when the bar is absent.
   */
  async historyResultsCounts(): Promise<{
    shown: number;
    total: number;
  } | null> {
    if ((await this.historyResultsBar().count()) === 0) return null;
    const match = NOVERA_CHAT.history.resultsCountPattern.exec(
      await this.historyResultsBar().innerText(),
    );
    return match
      ? { shown: Number(match[1]), total: Number(match[2]) }
      : null;
  }

  /** Opens the history's filter panel. */
  historyFiltersButton(): Locator {
    return this.main().getByRole("button", {
      name: NOVERA_CHAT.history.filtersButton,
      exact: true,
    });
  }

  /**
   * The same control once a filter is applied, where it clears instead.
   *
   * @param activeCount - How many filters are active, which the label carries.
   */
  historyClearFiltersButton(activeCount: number): Locator {
    return this.main().getByRole("button", {
      name: NOVERA_CHAT.history.clearFiltersButton(activeCount),
      exact: true,
    });
  }

  /** The State filter select, addressed by the id its definition sets. */
  stateFilterSelect(): Locator {
    return this.main().locator(`#${NOVERA_CHAT.history.stateFilter.selectId}`);
  }

  /**
   * Reads the options the State filter offers.
   *
   * Waits for the first option before reading: `allInnerTexts` does not retry, so
   * reading straight after the click can return an empty list while the portal
   * mounts.
   *
   * Drops the "All States" entry, which means *no* filter — choosing it on an
   * unfiltered list sends no request at all, so a caller picking the first option
   * would wait for something that never happens.
   *
   * @returns The real state labels, in order.
   */
  async stateFilterOptions(): Promise<string[]> {
    await this.stateFilterSelect().click();

    const options = this.page.getByRole("option");
    await expect(options.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    const labels = await options.allInnerTexts();

    await this.page.keyboard.press("Escape");
    return labels
      .map((label) => label.trim())
      .filter(
        (label) =>
          label.length > 0 &&
          label !== NOVERA_CHAT.history.stateFilter.allOption,
      );
  }

  /**
   * Chooses a state in the filter panel.
   *
   * Single-select, so the menu closes on its own — no Escape needed, unlike the
   * multi-select filters elsewhere.
   *
   * @param label - Option label to choose.
   */
  async selectStateFilter(label: string): Promise<void> {
    await this.stateFilterSelect().click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
  }

  /** The Sort by select. */
  historySortFieldSelect(): Locator {
    return this.main().locator(`#${NOVERA_CHAT.history.sort.fieldSelectId}`);
  }

  /** The Order by select. */
  historySortOrderSelect(): Locator {
    return this.main().locator(`#${NOVERA_CHAT.history.sort.orderSelectId}`);
  }

  /**
   * Chooses a sort field or order.
   *
   * @param select - The select to operate.
   * @param label - Exact option label.
   */
  async chooseHistorySortOption(
    select: Locator,
    label: string,
  ): Promise<void> {
    await select.click();
    await this.page.getByRole("option", { name: label, exact: true }).click();
  }

  /** Conversation rows in the history, each carrying a chat number. */
  historyRows(): Locator {
    return this.main()
      .getByRole("button")
      .filter({ hasText: NOVERA_CHAT.history.numberPattern });
  }

  /**
   * The chat number a listed conversation carries.
   *
   * The list is the only place it is exposed: nothing in the chat flow's
   * responses carries it, and the chat page does not show it — so a test that
   * wants to search by number has to read it from a row first.
   *
   * @param index - Zero-based row.
   * @returns The number, or null when the row has none.
   */
  async historyRowNumber(index: number): Promise<string | null> {
    const match = NOVERA_CHAT.history.numberPattern.exec(
      await this.historyRows().nth(index).innerText(),
    );
    return match ? match[0] : null;
  }

  /**
   * The creators named on the listed conversations, one per row.
   *
   * Parsed out of each row's text: the column has no test id, and reading cells
   * positionally would break the moment a column moves.
   *
   * @returns One address per row that names one.
   */
  async historyRowCreators(): Promise<string[]> {
    const rows = await this.historyRows().allInnerTexts();
    return rows
      .map((row) => /([\w.+-]+@[\w.-]+)/.exec(row)?.[1])
      .filter((creator): creator is string => !!creator);
  }

  /**
   * Searches the history and waits for the results to settle.
   *
   * @param term - What to search for.
   */
  async searchHistory(term: string): Promise<void> {
    await this.historySearchInput().fill(term);
    await expect(this.historyRows().first()).toBeVisible({
      timeout: LOAD_TIMEOUT_MS,
    });
  }

  /**
   * The Resume control on the first listed conversation.
   *
   * The list defaults to newest first, so after creating one it is the top row.
   * It cannot be addressed by id: rows show the chat number, not the sysid the
   * URL carries, and the sysid is nowhere in the list — so the caller confirms
   * the right row by checking where Resume lands.
   *
   * Only an open conversation offers Resume; a closed one reads "View", so a
   * closed top row simply has no match here.
   */
  firstResumeButton(): Locator {
    return this.historyRows()
      .first()
      .getByRole("button", {
        name: NOVERA_CHAT.history.resumeButton,
        exact: true,
      });
  }

  /**
   * The Close control on the first listed conversation.
   *
   * Offered only while the conversation is resumable, so a closed row has
   * neither this nor Resume.
   */
  firstCloseButton(): Locator {
    return this.historyRows()
      .first()
      .getByRole("button", {
        name: NOVERA_CHAT.history.closeButton,
        exact: true,
      });
  }

  /**
   * The View control on the first listed conversation.
   *
   * Offered in place of Resume once a conversation is closed, so a row that still
   * offers Resume has no match here.
   */
  firstViewButton(): Locator {
    return this.historyRows()
      .first()
      .getByRole("button", {
        name: NOVERA_CHAT.history.viewButton,
        exact: true,
      });
  }

  /**
   * A label in the conversation's summary panel.
   *
   * @param label - The field's label, e.g. "Chat Number".
   */
  sessionLabel(label: string): Locator {
    return this.main().getByText(label, { exact: true });
  }

  /** The close confirmation dialog. */
  closeDialog(): Locator {
    return this.page.getByRole("dialog");
  }

  /**
   * Confirms the close and waits for the state change to land.
   *
   * Closing is a state update on the conversation — `PATCH /conversations/{id}`
   * with a status — so the request body is what distinguishes it.
   *
   * @param conversationId - Conversation being closed.
   * @returns The update response.
   */
  async confirmClose(conversationId: string): Promise<Response> {
    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname.endsWith(`/conversations/${conversationId}`) &&
          r.request().method() === "PATCH",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      this.closeDialog()
        .getByRole("button", {
          name: NOVERA_CHAT.closeDialog.confirmButton,
          exact: true,
        })
        .click(),
    ]);
    return response;
  }

  /** The message box on an open conversation. */
  messageInput(): Locator {
    return this.main().getByPlaceholder(NOVERA_CHAT.message.inputPlaceholder);
  }

  /** The send control, which stays disabled until there is text to send. */
  sendButton(): Locator {
    return this.main().getByRole("button", {
      name: NOVERA_CHAT.message.sendButton,
      exact: true,
    });
  }

  /**
   * Sends a message on an open conversation.
   *
   * @param message - The text to send.
   */
  async sendMessage(message: string): Promise<void> {
    await this.messageInput().fill(message);
    await expect(this.sendButton()).toBeEnabled({ timeout: LOAD_TIMEOUT_MS });
    await this.sendButton().click();
  }

  /**
   * Describes an issue and submits it.
   *
   * @param question - The text to send.
   */
  async submitIssue(question: string): Promise<void> {
    await this.issueInput().fill(question);
    await expect(this.submitButton()).toBeEnabled({
      timeout: LOAD_TIMEOUT_MS,
    });
    await this.submitButton().click();
  }
}
