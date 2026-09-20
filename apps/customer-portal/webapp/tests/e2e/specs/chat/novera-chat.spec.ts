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

//
// Starting a Novera conversation from Get Help.
//
// The assistant has to be on for Get Help to reach the chat at all — with it off
// the same button opens the create-case form — so the test enables it as setup
// and switches it back afterwards.
//
// Two tests: "get help" stops at the conversation, "create case" carries on into
// the case form. Each enables the assistant itself and switches it back, so
// either can run alone — at the cost of a conversation each.
//
// ⚠️ WRITES:
//
// - `hasAgent` on the project, which changes Get Help for every other spec. The
//   restore runs from `finally`, and the restore is best-effort so a run
//   that died mid-flow is corrected by the next one.
// - a conversation per test, and a case from the second, none of which can be
//   deleted — so a full run leaves two conversations and one case behind.
//
// The question deliberately contains nothing resembling personal data: the page
// runs the text past a PII check before starting the chat, and a match would open
// a warning dialog rather than submitting.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { CaseCreatePage } from "../../pages/CaseCreatePage";
import { NoveraChatPage } from "../../pages/NoveraChatPage";
import { SettingsPage } from "../../pages/SettingsPage";
import { SupportCenterPage } from "../../pages/SupportCenterPage";
import { PROJECTS, NOVERA_CHAT_INPUT } from "../../config/testData";
import {
  CASE_DETAIL,
  CREATE_CASE,
  NOVERA_CHAT,
  SETTINGS,
  SUPPORT_CENTER,
} from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";
import { conversationSearchWith } from "../../utils/listSearch";
import { setNoveraViaApi } from "../../utils/noveraFlows";
import { idPattern, projectPathPattern } from "../../utils/ids";

withSession(test);

/**
 * Settling time after submitting the issue, before anything else is done.
 *
 * A deliberate fixed wait, which the rest of this suite avoids. The conversation
 * id landing proves the record exists, but the exchange behind it is still in
 * flight — the assistant is mid-reply ("Thinking…") and the messages persist
 * asynchronously, which is why a freshly created conversation lists as "0
 * messages". There is no rendered signal for that settling, so acting straight
 * away — navigating to the history, or clicking Create Case — races it.
 */
const SUBMIT_SETTLE_MS = 50_000;

test.describe("Novera Chat", () => {
  // Enabling the assistant, a shell load, Get Help, and the chat's own start
  // request — well past the 30s default.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[NOVERA_CHAT_INPUT.projectType];

  /**
   * Enables the assistant, opens Get Help and submits the question.
   *
   * Shared so each test can run alone: both need a live conversation, and the
   * second one's create-case flow is only reachable from inside it.
   *
   * The caller owns switching the assistant back off — it is left on when this
   * returns, since the conversation is still open.
   *
   * @param page - Test page.
   * @returns The chat page object and the conversation's id.
   */
  async function getHelpAndSubmit(
    page: Page,
  ): Promise<{ chat: NoveraChatPage; conversationId: string }> {
    // STEP ONE, always: the AI Chat Assistant (Novera) has to be active before
    // anything else here means anything. Get Help branches on this project's
    // `hasAgent` — with the assistant off it opens the create-case form instead
    // of the chat, and the failure then surfaces several steps later pointing at
    // the wrong thing entirely.
    //
    // Enabled through the API rather than the Settings toggle: the toggle's own
    // PATCH currently aborts in the browser, so clicking it cannot turn the
    // assistant on at all. See setNoveraViaApi for the detail. The toggle
    // remains covered by settings.spec.ts, which drives the real control.
    await page.goto(`/projects/${project.id}/dashboard`);
    await setNoveraViaApi(page, project.id, true);

    // Then confirm it through the UI, in Settings → AI Assistant → Support
    // Capabilities: the chip is what a user sees, and reading it back proves the
    // write actually landed rather than merely returning 2xx.
    const settings = new SettingsPage(page);
    await settings.open(project.id);
    await settings.openTab(SETTINGS.tabs.aiAssistant);
    await expect(
      settings.noveraChip(SETTINGS.aiAssistant.novera.activeChip),
      "Novera must report Active before the chat flow can be exercised — " +
        "Get Help opens the create-case form while it is off",
    ).toBeVisible({ timeout: 30_000 });

    const chat = new NoveraChatPage(page);

    // Get Help now reaches the chat rather than the case form, which is the
    // branch the assistant controls.
    await chat.openViaGetHelp(project.id);
    await expect(chat.heading()).toBeVisible();

    // Empty to begin with, and submit withheld until there is something to
    // send.
    await expect(chat.issueInput()).toHaveValue("");
    await expect(chat.submitButton()).toBeDisabled();

    await chat.issueInput().fill(NOVERA_CHAT_INPUT.question);
    await expect(chat.issueInput()).toHaveValue(NOVERA_CHAT_INPUT.question);
    await expect(chat.submitButton()).toBeEnabled({ timeout: 30_000 });

    await chat.submitButton().click();

    // Wait for the conversation's own id, not just the chat route.
    //
    // Submitting navigates immediately and the id arrives a second or two later,
    // so asserting the bare route passes before the backend has created
    // anything — and a caller that then navigates away (to restore the
    // assistant, say) aborts the creation and leaves no chat entry. Verified
    // live: the id appeared at ~3s.
    await expect(page).toHaveURL(NOVERA_CHAT.conversationIdPattern, {
      timeout: 60_000,
    });
    await expect(page).not.toHaveURL(
      new RegExp(NOVERA_CHAT.describeIssue.pathSegment),
    );

    // And the question is carried into the conversation, which is what shows
    // the text was handed over rather than the page merely navigating.
    await expect(
      page.getByText(NOVERA_CHAT_INPUT.question, { exact: false }).first(),
    ).toBeVisible({ timeout: 60_000 });

    const conversationId = new URL(page.url()).pathname.split("/").pop();

    // Let the exchange settle before the caller does anything else.
    await page.waitForTimeout(SUBMIT_SETTLE_MS);

    console.log(
      `Novera chat: submitted "${NOVERA_CHAT_INPUT.question}" and opened ` +
        `conversation ${conversationId}`,
    );

    return { chat, conversationId: conversationId as string };
  }

  /**
   * Finds the conversation just created in the chat history, by its number.
   *
   * The conversation cannot be looked up by the id it was created with: rows show
   * a chat number, the sysid appears nowhere in the list, and no response in the
   * chat flow carries the number either. So the number is read off the newest row
   * — the list sorts by Updated on, newest first — and then searched for, which
   * unlike the shared question text narrows to exactly one row.
   *
   * @param page - Test page.
   * @param chat - The chat page object, on the history list.
   * @returns The conversation's chat number.
   */
  async function findByChatNumber(
    page: Page,
    chat: NoveraChatPage,
  ): Promise<string> {
    await chat.searchHistory(NOVERA_CHAT_INPUT.question);

    const chatNumber = await chat.historyRowNumber(0);
    expect(chatNumber, "the newest row should carry a chat number").toMatch(
      NOVERA_CHAT.history.numberPattern,
    );

    await chat.searchHistory(chatNumber as string);
    await expect(
      chat.historyRows(),
      `searching for ${chatNumber} should match exactly one conversation`,
    ).toHaveCount(1);
    await expect(chat.historyRows().first()).toContainText(
      chatNumber as string,
    );

    return chatNumber as string;
  }

  /**
   * Opens the full chat history from Support Center, as a user would.
   *
   * @param page - Test page.
   * @returns The chat page object, on the history list.
   */
  async function openChatHistory(page: Page): Promise<NoveraChatPage> {
    const support = new SupportCenterPage(page);
    await support.openViaSideNav(project.id);

    await support
      .chatHistoryFooterButton(SUPPORT_CENTER.chatHistory.allChatHistoryButton)
      .click();

    await expect(page).toHaveURL(
      projectPathPattern(
        project.id,
        `${NOVERA_CHAT.history.pathSegment}$`,
      ),
      { timeout: 60_000 },
    );

    return new NoveraChatPage(page);
  }

  /**
   * Closes the conversation the caller just created, from the history list.
   *
   * Shared with the view test, which needs a closed conversation to reach the
   * View action at all — a resumable one offers Resume instead.
   *
   * @param page - Test page.
   * @param chat - The chat page object, on the history list.
   * @param conversationId - The conversation to close.
   */
  async function closeFirstConversation(
    page: Page,
    chat: NoveraChatPage,
    conversationId: string,
  ): Promise<void> {
    await chat.firstCloseButton().click();

    const dialog = chat.closeDialog();
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(NOVERA_CHAT.closeDialog.title);

    const response = await chat.confirmClose(conversationId);
    await expectSuccess(response, "close the conversation");
    await expect(dialog).toBeHidden();
  }

  /**
   * Raises a case from the open conversation and returns it.
   *
   * ⚠️ Creates a permanent case — POST /cases has no delete counterpart. Shared
   * with the view test, which needs a converted conversation to reach the View
   * action at all.
   *
   * The form arrives *pre-populated* here, which is the difference from the Get
   * Help route the create-case suite drives: that one passes `skipChat` and opens
   * a blank form, while this one carries the conversation and has the deployment,
   * product, title and description filled in for review. So this is a
   * review-and-submit flow — selecting a deployment would find no placeholder,
   * because one is already chosen.
   *
   * @param page - Test page.
   * @param chat - The chat page object, on the open conversation.
   * @returns The created case's id and number.
   */
  async function createCaseFromConversation(
    page: Page,
    chat: NoveraChatPage,
  ): Promise<{ id?: string; number?: string }> {
      //
    // And on to a case from the conversation.
    //
    // ⚠️ Creates a permanent case — POST /cases has no delete counterpart.
    //
    // The form arrives *pre-populated* here, which is the difference from the
    // Get Help route the create-case suite drives: that one passes `skipChat`
    // and opens a blank form, while this one carries the conversation and has
    // the deployment, product, title and description filled in for review.
    // So this is a review-and-submit flow — selecting a deployment would find
    // no placeholder, because one is already chosen.
    //
    await chat.openCreateCase(project.id);

    const createCase = new CaseCreatePage(page);
    const generated = NOVERA_CHAT.generatedCaseForm;

    await expect(
      page.getByRole("heading", { name: CREATE_CASE.heading }),
    ).toBeVisible({ timeout: 60_000 });

    // The form says it is auto-populated and summarises what it came from.
    // Soft, so one missing marker does not hide the others.
    for (const marker of [
      generated.aiBadge,
      generated.reviewHint,
      generated.conversationSummarySection,
    ]) {
      await expect
        .soft(page.getByText(marker, { exact: true }).first(), marker)
        .toBeVisible({ timeout: 60_000 });
    }

    // Deployment and product came across already chosen — the fields carry the
    // project's own values rather than a placeholder.
    await expect(
      page.getByText(project.deployment, { exact: true }).first(),
    ).toBeVisible();
    await expect(
      page.getByText(project.productVersion, { exact: true }).first(),
    ).toBeVisible();
    await expect(
      page.getByText(generated.autoDetectedHint).first(),
    ).toBeVisible();

    // The title is generated too; it is overwritten so the case is
    // recognisable as this test's, and to prove the field is editable rather
    // than locked to what the AI produced.
    await expect(createCase.titleInput()).not.toHaveValue("");
    await createCase.fillTitle(NOVERA_CHAT_INPUT.caseTitle);
    await expect(createCase.titleInput()).toHaveValue(
      NOVERA_CHAT_INPUT.caseTitle,
    );

    await expect(createCase.submitButton()).toBeEnabled();

    const [createResponse] = await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes("/cases") && r.request().method() === "POST",
        { timeout: 60_000 },
      ),
      createCase.submit(),
    ]);
    await expectSuccess(createResponse, "create case from the conversation");

    const created = (await createResponse.json()) as {
      id?: string;
      number?: string;
    };
    expect(created.id, "backend returned no case id").toBeTruthy();

    // Lands on the new case, with the banner the form shows on success.
    await expect(page.getByText(CREATE_CASE.successMessage)).toBeVisible({
      timeout: 60_000,
    });
    await expect(page).toHaveURL(
      projectPathPattern(
        project.id,
        `support/cases/${idPattern(created.id!)}`,
      ),
    );

    return created;
  }

  test("get help", async ({ page }) => {
    test.skip(
      !project.id,
      `${NOVERA_CHAT_INPUT.projectType} needs a project id. ` +
        `Fill it in tests/e2e/config/testData.ts.`,
    );

    try {
      // Driven entirely through the UI, unlike the other tests here: they enable
      // the assistant through the API because it is setup for them, whereas for
      // this test turning it on IS part of what is being covered. Every step
      // below is one a user performs.
      const settings = new SettingsPage(page);

      // Settings from the side menu, not a typed URL.
      await settings.openViaSideNav(project.id);
      await settings.openTab(SETTINGS.tabs.aiAssistant);

      // The section the toggle lives under — asserted so a moved control is a
      // clear failure rather than a missing-element one.
      await expect(settings.capabilitiesSection()).toBeVisible({
        timeout: 30_000,
      });
      await expect(settings.noveraLabel()).toBeVisible();

      const novera = SETTINGS.aiAssistant.novera;

      // The switch renders disabled while the project details load, and
      // `isChecked()` does not retry — so wait for it to become interactive
      // before reading it, or the branch below decides on a state that is still
      // arriving.
      await expect(settings.noveraToggle()).toBeEnabled({ timeout: 30_000 });

      // Tick it if it is not already ticked. The switch reverts when the write
      // is rejected, so the assertions that follow are what establish the state
      // — not the click.
      if (!(await settings.noveraToggle().isChecked())) {
        await settings.setNovera(project.id, true);
      }

      await expect(
        settings.noveraToggle(),
        "the AI Chat Assistant (Novera) toggle must be ticked",
      ).toBeChecked({ timeout: 30_000 });

      // Ticked and reporting Active are different things: the switch is local
      // state, the chip reflects what the backend stored. Both are asserted, and
      // Inactive must be gone rather than merely co-existing.
      await expect(
        settings.noveraChip(novera.activeChip),
        "the assistant must report Active, not merely appear ticked",
      ).toBeVisible({ timeout: 30_000 });
      await expect(settings.noveraChip(novera.inactiveChip)).toHaveCount(0);

      console.log("Novera is active; opening the chat from Get Help");

      // Only now does Get Help reach the chat: it branches on the project's
      // `hasAgent`, which is exactly what was just switched on.
      const chat = new NoveraChatPage(page);
      await chat.openViaGetHelp(project.id);
      await expect(chat.heading()).toBeVisible();

      await expect(chat.issueInput()).toHaveValue("");
      await expect(chat.submitButton()).toBeDisabled();

      await chat.issueInput().fill(NOVERA_CHAT_INPUT.question);
      await expect(chat.issueInput()).toHaveValue(NOVERA_CHAT_INPUT.question);
      await expect(chat.submitButton()).toBeEnabled({ timeout: 30_000 });

      await chat.submitButton().click();

      // The conversation's own id, not just the chat route — the id arrives a
      // second or two after the navigation, and asserting the bare route passes
      // before the backend has created anything.
      await expect(page).toHaveURL(NOVERA_CHAT.conversationIdPattern, {
        timeout: 60_000,
      });
      await expect(page).not.toHaveURL(
        new RegExp(NOVERA_CHAT.describeIssue.pathSegment),
      );

      const conversationId =
        page.url().match(NOVERA_CHAT.conversationIdPattern)?.[0] ?? "";
      console.log(`Novera chat: opened conversation at ${conversationId}`);

      // Let the assistant settle before the restore navigates away, so the
      // conversation is not aborted mid-creation.
      await page.waitForTimeout(SUBMIT_SETTLE_MS);
    } finally {
      // Off again whatever happened: leaving the assistant on sends every
      // create-case spec to the chat instead of the form.
      await setNoveraViaApi(page, project.id, false).catch(() => undefined);
    }
  });

  test("create case", async ({ page }) => {
    test.skip(
      !project.id,
      `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
    );

    try {
      const { chat } = await getHelpAndSubmit(page);

      const created = await createCaseFromConversation(page, chat);

      console.log(
        `Novera chat: created case ${created.number ?? created.id} from the ` +
          `conversation`,
      );
    } finally {
      await setNoveraViaApi(page, project.id, false).catch(() => undefined);
    }
  });

  test("resume chat", async ({ page }) => {
    test.skip(
      !project.id,
      `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
    );

    try {
      const { conversationId } = await getHelpAndSubmit(page);

      // Out to Support Center and into the full chat history, the way a user
      // returns to an earlier conversation.
      const chat = await openChatHistory(page);

      const chatNumber = await findByChatNumber(page, chat);

      const resume = chat.firstResumeButton();
      await expect(
        resume,
        `the newest conversation should be resumable — a closed one offers ` +
          `"View" instead`,
      ).toBeVisible({ timeout: 60_000 });

      await resume.click();

      // Back on the conversation itself — and this is what proves the right row
      // was picked, since the list gave no id to match on.
      //
      // Resume opens the conversation *detail* page rather than the chat route,
      // and that page renders the same message box when the conversation is
      // resumable.
      await expect(page).toHaveURL(NOVERA_CHAT.resumedConversationPattern, {
        timeout: 60_000,
      });
      expect(
        page.url(),
        "Resume should reopen the conversation this test created",
      ).toContain(conversationId);

      // The conversation opens resumable, which is the point: the message box
      // renders only when the page is reached through the list — a direct visit
      // to the same URL is read-only.
      //
      // The question itself is deliberately not asserted here. A freshly created
      // conversation has no messages stored — the text is kept as the
      // conversation's `initialMessage`, which is what the history row showed —
      // so the detail page reports "No messages found" until something is sent.
      await expect(chat.messageInput()).toBeVisible({ timeout: 60_000 });

      // And it accepts a follow-up.
      await expect(chat.messageInput()).toHaveValue("");
      await expect(chat.sendButton()).toBeDisabled();

      await chat.sendMessage(NOVERA_CHAT_INPUT.followUp);

      // The message is posted into the conversation, and the box clears ready for
      // the next one.
      await expect(
        page.getByText(NOVERA_CHAT_INPUT.followUp, { exact: false }).first(),
      ).toBeVisible({ timeout: 60_000 });
      await expect(chat.messageInput()).toHaveValue("", { timeout: 30_000 });

      console.log(
        `Novera chat: found ${chatNumber} by number, resumed ` +
          `${conversationId} and posted "${NOVERA_CHAT_INPUT.followUp}"`,
      );
    } finally {
      await setNoveraViaApi(page, project.id, false).catch(() => undefined);
    }
  });

  test("close chat", async ({ page }) => {
    test.skip(
      !project.id,
      `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
    );

    try {
      const { conversationId } = await getHelpAndSubmit(page);

      const chat = await openChatHistory(page);
      const chatNumber = await findByChatNumber(page, chat);

      // Open to begin with, which is why Close is offered at all — the same rule
      // decides Close and Resume, so a closed conversation has neither.
      const row = chat.historyRows().first();
      await expect(row).toContainText(NOVERA_CHAT.history.activeState);
      await expect(chat.firstResumeButton()).toBeVisible();

      await closeFirstConversation(page, chat, conversationId);

      // The actions it offered are gone and View has taken their place — a closed
      // conversation cannot be closed again or resumed, only viewed. These come
      // first because they are unambiguous.
      await expect(chat.firstCloseButton()).toHaveCount(0, {
        timeout: 60_000,
      });
      await expect(chat.firstResumeButton()).toHaveCount(0);
      await expect(
        row.getByRole("button", {
          name: NOVERA_CHAT.history.viewButton,
          exact: true,
        }),
      ).toBeVisible();

      // And the state chip reports it. Asserted only now: the chip reads "Close"
      // — the same word as the button that closed it — so it means the state
      // only once that button has gone.
      await expect(row).toContainText(NOVERA_CHAT.history.closedState);
      await expect(row).not.toContainText(NOVERA_CHAT.history.activeState);

      console.log(
        `Novera chat: closed ${chatNumber} (${conversationId}) and the row ` +
          `offers View only`,
      );
    } finally {
      await setNoveraViaApi(page, project.id, false).catch(() => undefined);
    }
  });


  test("view chat", async ({ page }) => {
    test.skip(
      !project.id,
      `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
    );

    try {
      const { conversationId } = await getHelpAndSubmit(page);

      const chat = await openChatHistory(page);
      const chatNumber = await findByChatNumber(page, chat);

      // Closed first, because View is only offered once it is: a resumable
      // conversation shows Resume in that slot.
      await closeFirstConversation(page, chat, conversationId);

      const view = chat.firstViewButton();
      await expect(
        view,
        `a closed conversation should offer View in place of Resume`,
      ).toBeVisible({ timeout: 60_000 });

      await view.click();

      // The conversation that was listed, not merely some conversation.
      await expect(page).toHaveURL(NOVERA_CHAT.resumedConversationPattern, {
        timeout: 60_000,
      });
      expect(page.url()).toContain(conversationId);

      // The summary panel and the two fields asked for. Soft, so one missing
      // label reports alongside the rest.
      const session = NOVERA_CHAT.session;
      for (const label of [
        session.section,
        session.statusLabel,
        session.chatNumberLabel,
      ]) {
        await expect
          .soft(chat.sessionLabel(label).first(), label)
          .toBeVisible({ timeout: 60_000 });
      }

      // The chat number shown is the one the list was searched by, which ties
      // this view to the conversation the test created.
      await expect(chat.sessionLabel(chatNumber).first()).toBeVisible();

      // And the status reports it closed — the same wording the row's chip uses.
      await expect(
        page.getByTestId(CASE_DETAIL.mainTestId),
      ).toContainText(NOVERA_CHAT.history.closedState);

      // View is read-only: a closed conversation offers no message box, which is
      // what separates this from the resumed view.
      await expect(chat.messageInput()).toHaveCount(0);

      console.log(
        `Novera chat: viewed closed ${chatNumber} (${conversationId})`,
      );
    } finally {
      await setNoveraViaApi(page, project.id, false).catch(() => undefined);
    }
  });


  test("view case", async ({ page }) => {
    test.skip(
      !project.id,
      `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
    );

    // The only test here that settles TWICE — once after submitting the issue
    // (inside getHelpAndSubmit) and again after raising the case — so 100s of
    // the describe's 180s budget is spent waiting before any of its own work
    // begins. That leaves too little for a create, a history search and the
    // detail load on a slow backend.
    test.setTimeout(300_000);

    try {
      const { chat, conversationId } = await getHelpAndSubmit(page);
      const created = await createCaseFromConversation(page, chat);

      const history = await openChatHistory(page);
      const chatNumber = await findByChatNumber(page, history);

      // Raising a case converts the conversation, and a converted one is
      // terminal — so the row offers View where it offered Resume before.
      const row = history.historyRows().first();
      await expect(row).toContainText(NOVERA_CHAT.history.convertedState, {
        timeout: 60_000,
      });
      await expect(history.firstResumeButton()).toHaveCount(0);

      await history.firstViewButton().click();

      // The conversation the case came from, not merely some conversation.
      await expect(page).toHaveURL(NOVERA_CHAT.resumedConversationPattern, {
        timeout: 60_000,
      });
      expect(page.url()).toContain(conversationId);

      // The summary panel reports the conversion and identifies the chat.
      const session = NOVERA_CHAT.session;
      const main = page.getByTestId(CASE_DETAIL.mainTestId);

      for (const label of [
        session.section,
        session.statusLabel,
        session.chatNumberLabel,
      ]) {
        await expect
          .soft(history.sessionLabel(label).first(), label)
          .toBeVisible({ timeout: 60_000 });
      }
      await expect(history.sessionLabel(chatNumber).first()).toBeVisible();
      await expect(main).toContainText(NOVERA_CHAT.history.convertedState);

      // The exchange is there to read, and read-only: a converted conversation
      // offers no message box, the same as a closed one.
      await expect(
        main.getByText(NOVERA_CHAT_INPUT.question, { exact: false }).first(),
      ).toBeVisible();
      await expect(history.messageInput()).toHaveCount(0);

      // ⚠️ The case number is deliberately not asserted here: the conversation
      // detail does not show it (verified live), so there is no link back from
      // the chat to the case it produced. Worth raising as a product gap rather
      // than asserting something that is not rendered.
      console.log(
        `Novera chat: viewed converted ${chatNumber} behind case ` +
          `${created.number ?? created.id}`,
      );
    } finally {
      await setNoveraViaApi(page, project.id, false).catch(() => undefined);
    }
  });

  //
  // The Chat History card's two views.
  //
  // ✅ READ-ONLY, and no assistant toggling: the card is not gated on `hasAgent`,
  // so the history is reachable whether the assistant is on or off. Which is why
  // these do not create a conversation either — they read whatever the project
  // already has, including everything the tests above left behind.
  //
  test.describe("chat history", () => {

    //
    // Search, filter and sort, run against both views.
    //
    // Parameterised rather than duplicated: the controls are the same in each,
    // and what has to differ is the `createdByMe` the requests carry — which is
    // asserted on the wire, since the heading only reports the view.
    //
    for (const [viewKey, view] of Object.entries(NOVERA_CHAT.history.views)) {
      const isMine = viewKey === "mine";
      const footerButton = isMine
        ? SUPPORT_CENTER.chatHistory.myChatHistoryButton
        : SUPPORT_CENTER.chatHistory.allChatHistoryButton;

      /**
       * Opens the view under test and waits for its first results.
       *
       * @param page - Test page.
       * @returns The chat page object, on the history list.
       */
      async function openView(page: Page): Promise<NoveraChatPage> {
        const support = new SupportCenterPage(page);
        await support.openViaSideNav(project.id);
        await support.chatHistoryFooterButton(footerButton).click();

        const chat = new NoveraChatPage(page);
        await expect(
          page.getByTestId(CASE_DETAIL.mainTestId).getByRole("heading", {
            name: view.title,
            exact: true,
          }),
        ).toBeVisible({ timeout: 60_000 });
        await expect(chat.historyRows().first()).toBeVisible({
          timeout: 60_000,
        });
        return chat;
      }

      test(`${view.title} — searches by chat number`, async ({ page }) => {
        test.skip(
          !project.id,
          `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
        );

        const chat = await openView(page);

        const before = await chat.historyResultsCounts();
        expect(before, "the results bar should report counts").not.toBeNull();

        // Searched by a chat number taken from the list, which unlike the shared
        // question text narrows to exactly one conversation.
        const chatNumber = await chat.historyRowNumber(0);
        expect(chatNumber).toMatch(NOVERA_CHAT.history.numberPattern);

        const searched = conversationSearchWith(page, {
          searchQuery: chatNumber as string,
          createdByMe: isMine,
        });
        await chat.historySearchInput().fill(chatNumber as string);
        await expectSuccess(await searched, `${view.title} search`);

        await expect(chat.historyRows()).toHaveCount(1, { timeout: 60_000 });
        await expect(chat.historyRows().first()).toContainText(
          chatNumber as string,
        );

        // A term nobody matches empties the list — which is what shows the
        // search is applied rather than the row surviving by coincidence.
        const missed = conversationSearchWith(page, {
          searchQuery: "CHAT000000000000",
          createdByMe: isMine,
        });
        await chat.historySearchInput().fill("CHAT000000000000");
        await expectSuccess(await missed, `${view.title} search with no match`);
        await expect(chat.historyRows()).toHaveCount(0);

        console.log(
          `Novera chat (${view.title}): searched ${chatNumber} in a list of ` +
            `${(before as { total: number }).total}`,
        );
      });

      test(`${view.title} — filters by state and clears it`, async ({
        page,
      }) => {
        test.skip(
          !project.id,
          `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
        );

        const chat = await openView(page);

        // The panel is collapsed on load and its contents unmounted, so the
        // select does not exist until it is opened.
        await expect(chat.stateFilterSelect()).toHaveCount(0);
        await chat.historyFiltersButton().click();
        await expect(chat.stateFilterSelect()).toBeVisible();

        const options = await chat.stateFilterOptions();
        expect(
          options.length,
          "the state filter should offer options",
        ).toBeGreaterThan(0);

        // The state ids on the wire are what show the filter was applied, rather
        // than merely ticked in the menu.
        const filtered = conversationSearchWith(page, {
          stateKeys: true,
          createdByMe: isMine,
        });
        await chat.selectStateFilter(options[0]);
        await expectSuccess(await filtered, `${view.title} filtered by state`);

        await expect(chat.historyClearFiltersButton(1)).toBeVisible();
        await expect(chat.historyFiltersButton()).toHaveCount(0);

        // Clearing drops the states from the request. Armed first, because the
        // view's own initial load matches this predicate too.
        const cleared = conversationSearchWith(page, {
          stateKeys: false,
          createdByMe: isMine,
        });
        await chat.historyClearFiltersButton(1).click();
        await expectSuccess(await cleared, `${view.title} after clearing`);

        await expect(chat.historyFiltersButton()).toBeVisible();

        console.log(
          `Novera chat (${view.title}): filtered by "${options[0]}" and cleared`,
        );
      });

      test(`${view.title} — sorts by field and order`, async ({ page }) => {
        test.skip(
          !project.id,
          `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
        );

        const chat = await openView(page);
        const { fields, orders } = NOVERA_CHAT.history.sort;

        // Switching the field. `sortBy` sits at the root of the request body
        // rather than inside `filters`.
        const byCreated = conversationSearchWith(page, {
          sortField: fields.createdOn.value,
          createdByMe: isMine,
        });
        await chat.chooseHistorySortOption(
          chat.historySortFieldSelect(),
          fields.createdOn.label,
        );
        await expectSuccess(await byCreated, `${view.title} sorted by created`);
        await expect(chat.historySortFieldSelect()).toContainText(
          fields.createdOn.label,
        );

        // And the order. Both fields here are chronological, so the labels stay
        // Newest/Oldest first — there is no ordinal field to reword them.
        const ascending = conversationSearchWith(page, {
          sortField: fields.createdOn.value,
          sortOrder: orders.oldestFirst.value,
          createdByMe: isMine,
        });
        await chat.chooseHistorySortOption(
          chat.historySortOrderSelect(),
          orders.oldestFirst.label,
        );
        await expectSuccess(await ascending, `${view.title} sorted ascending`);
        await expect(chat.historySortOrderSelect()).toContainText(
          orders.oldestFirst.label,
        );

        // Back to the default field, which must carry its own value through.
        const byUpdated = conversationSearchWith(page, {
          sortField: fields.updatedOn.value,
          createdByMe: isMine,
        });
        await chat.chooseHistorySortOption(
          chat.historySortFieldSelect(),
          fields.updatedOn.label,
        );
        await expectSuccess(await byUpdated, `${view.title} sorted by updated`);

        console.log(
          `Novera chat (${view.title}): sorted by both fields and reversed the ` +
            `order`,
        );
      });
    }

    test("view my chat history", async ({ page }) => {
      test.skip(
        !project.id,
        `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
      );

      const support = new SupportCenterPage(page);
      await support.openViaSideNav(project.id);

      const mine = NOVERA_CHAT.history.views.mine;
      await support
        .chatHistoryFooterButton(SUPPORT_CENTER.chatHistory.myChatHistoryButton)
        .click();

      // The query parameter is what filters the list; the heading below merely
      // reports it, and both views share this route.
      await expect(page).toHaveURL(
        new RegExp(
          `/projects/${idPattern(project.id)}/${NOVERA_CHAT.history.pathSegment}\\?${mine.query}`,
        ),
        { timeout: 60_000 },
      );

      const main = page.getByTestId(CASE_DETAIL.mainTestId);
      await expect(
        main.getByRole("heading", { name: mine.title, exact: true }),
      ).toBeVisible({ timeout: 60_000 });
      await expect(
        main.getByText(mine.description, { exact: true }),
      ).toBeVisible();

      const chat = new NoveraChatPage(page);
      await expect(chat.historyRows().first()).toBeVisible({
        timeout: 60_000,
      });

      // Every row is this account's — which is what "my" has to mean. Counted as
      // one distinct creator rather than matched against an address, so it holds
      // for whichever account the captured session belongs to.
      const creators = await chat.historyRowCreators();
      expect(creators.length, "no row named a creator").toBeGreaterThan(0);
      expect(
        new Set(creators).size,
        "every row in My Chat History should have the same creator",
      ).toBe(1);

      console.log(
        `Novera chat: My Chat History listed ${creators.length} conversations`,
      );
    });

    test("view all chat history", async ({ page }) => {
      test.skip(
        !project.id,
        `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
      );

      const support = new SupportCenterPage(page);
      await support.openViaSideNav(project.id);

      const all = NOVERA_CHAT.history.views.all;
      await support
        .chatHistoryFooterButton(SUPPORT_CENTER.chatHistory.allChatHistoryButton)
        .click();

      // No query string: the same route, unfiltered. Anchored so a
      // `?createdByMe=true` on it could not satisfy this.
      await expect(page).toHaveURL(
        new RegExp(
          `/projects/${idPattern(project.id)}/${NOVERA_CHAT.history.pathSegment}$`,
        ),
        { timeout: 60_000 },
      );

      const main = page.getByTestId(CASE_DETAIL.mainTestId);
      await expect(
        main.getByRole("heading", { name: all.title, exact: true }),
      ).toBeVisible({ timeout: 60_000 });
      await expect(
        main.getByText(all.description, { exact: true }),
      ).toBeVisible();

      const chat = new NoveraChatPage(page);
      await expect(chat.historyRows().first()).toBeVisible({
        timeout: 60_000,
      });

      // Rows carry a creator, but no assertion that several do: this account
      // created nearly every conversation on the project, so a single creator
      // here is expected rather than a filtering failure.
      const creators = await chat.historyRowCreators();
      expect(creators.length, "no row named a creator").toBeGreaterThan(0);

      console.log(
        `Novera chat: All Chat History listed ${creators.length} conversations ` +
          `from ${new Set(creators).size} creator(s) on this page`,
      );
    });
  });

  //
  // Validation on the describe-issue page.
  //
  // The assistant still has to be on for the page to be reachable at all, so each
  // test enables it and switches it back.
  //
  // ✅ The first two create nothing: they never submit, so no conversation is
  // started. The Enter-key one does submit by design — that is the behaviour
  // under test — and so leaves a conversation like the flow tests above.
  //
  test.describe("validation", () => {
    /**
     * Enables the assistant and opens the describe-issue page.
     *
     * Stops short of submitting, unlike `getHelpAndSubmit` — these tests are
     * about the form's gating rather than what follows it.
     *
     * @param page - Test page.
     * @returns The chat page object, on the describe-issue page.
     */
    async function openDescribeIssue(page: Page): Promise<NoveraChatPage> {
      // Same first step as the flows above, and for the same reason: the
      // describe-issue page only exists while the assistant is on.
      await page.goto(`/projects/${project.id}/dashboard`);
      await setNoveraViaApi(page, project.id, true);

      const chat = new NoveraChatPage(page);
      await chat.openViaGetHelp(project.id);
      return chat;
    }

    test("withholds submit for whitespace-only text", async ({ page }) => {
      test.skip(
        !project.id,
        `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
      );

      try {
        const chat = await openDescribeIssue(page);

        await expect(chat.submitButton()).toBeDisabled();

        // Spaces are not a question: the gate trims before checking, so this must
        // read as empty.
        await chat.issueInput().fill("     ");
        await expect(chat.issueInput()).toHaveValue("     ");
        await expect(chat.submitButton()).toBeDisabled();

        // Real text enables it, which is what shows the rule above is about the
        // whitespace rather than the form being broken.
        await chat.issueInput().fill(NOVERA_CHAT_INPUT.question);
        await expect(chat.submitButton()).toBeEnabled({ timeout: 30_000 });

        // Back to spaces, and it is withheld again.
        await chat.issueInput().fill("   ");
        await expect(chat.submitButton()).toBeDisabled();

        // Still on the form — nothing was submitted, so no conversation exists.
        await expect(page).toHaveURL(
          new RegExp(NOVERA_CHAT.describeIssue.pathSegment),
        );

        console.log(`Novera chat: whitespace-only text cannot be submitted`);
      } finally {
        await setNoveraViaApi(page, project.id, false).catch(() => undefined);
      }
    });

    test("withholds submit over the character cap", async ({ page }) => {
      test.skip(
        !project.id,
        `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
      );

      try {
        const chat = await openDescribeIssue(page);
        const max = NOVERA_CHAT.describeIssue.maxChars;

        // At the cap exactly, which is accepted. Asserted alongside the rejection
        // below because it is what pins the limit at this number — without it,
        // "long text is rejected" would pass for any cap at all.
        await chat.issueInput().fill("a".repeat(max));
        await expect(chat.submitButton()).toBeEnabled({ timeout: 30_000 });
        await expect(
          page.getByText(
            NOVERA_CHAT.describeIssue.tooLongMessage(max, max),
            { exact: false },
          ),
        ).toHaveCount(0);

        // One character over, and it is refused with the count quoted back.
        await chat.issueInput().fill("a".repeat(max + 1));
        await expect(chat.submitButton()).toBeDisabled();
        await expect(
          page.getByText(
            NOVERA_CHAT.describeIssue.tooLongMessage(max + 1, max),
            { exact: false },
          ),
        ).toBeVisible({ timeout: 30_000 });

        // Still on the form.
        await expect(page).toHaveURL(
          new RegExp(NOVERA_CHAT.describeIssue.pathSegment),
        );

        console.log(
          `Novera chat: ${max} characters accepted, ${max + 1} refused`,
        );
      } finally {
        await setNoveraViaApi(page, project.id, false).catch(() => undefined);
      }
    });

    test("submits on Enter but not on Shift+Enter", async ({ page }) => {
      test.skip(
        !project.id,
        `${NOVERA_CHAT_INPUT.projectType} needs a project id.`,
      );

      try {
        const chat = await openDescribeIssue(page);
        await chat.issueInput().fill(NOVERA_CHAT_INPUT.enterKeyQuestion);

        // Shift+Enter is a newline, not a submit — the handler checks for it
        // explicitly. Staying on the form is what shows nothing was sent.
        await chat.issueInput().press("Shift+Enter");
        await expect(page).toHaveURL(
          new RegExp(NOVERA_CHAT.describeIssue.pathSegment),
          { timeout: 15_000 },
        );
        await expect(chat.heading()).toBeVisible();

        // ⚠️ Enter does submit, so this creates a conversation — the same
        // permanent record the flow tests leave. The text is still there to send,
        // now carrying the newline Shift+Enter added.
        await chat.issueInput().press("Enter");

        // Waited through to the conversation id rather than just the route: a
        // caller that navigates away before it lands aborts the creation, which
        // is how the get-help test once passed while persisting nothing.
        await expect(page).toHaveURL(NOVERA_CHAT.conversationIdPattern, {
          timeout: 60_000,
        });
        await page.waitForTimeout(SUBMIT_SETTLE_MS);

        const conversationId = new URL(page.url()).pathname.split("/").pop();
        console.log(
          `Novera chat: Shift+Enter did not submit, Enter opened ` +
            `conversation ${conversationId}`,
        );
      } finally {
        await setNoveraViaApi(page, project.id, false).catch(() => undefined);
      }
    });
  });
});
