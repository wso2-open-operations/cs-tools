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
// Escalates a support case from its detail page.
//
// The case is created by the test rather than picked from the project's
// existing ones: escalation notifies real people (EL1 is the Team Lead), so the
// test must only ever act on a record it owns, and a fresh case is also the only
// way to be sure it starts at level 0 — the one step this test can take without
// a project Lead.
//
// Scoped to the Subscription project at S4, which is the severity the project's
// own fixture already uses. Severity does not gate escalation — the level does —
// so S4 keeps the created record at the lowest-impact end while still
// exercising the flow.
//
// The levels run EL1..EL5 and each has its own notified role. A new case sits at
// level 0, so this covers EL0 → EL1 (Team Lead). It deliberately stops there:
// ESCALATION_LEAD_REQUIRED_FROM_LEVEL gates levels 3 and 4 behind project Lead
// membership, and climbing further would page increasingly senior real people
// on every run.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  CASE_INPUT,
  DEESCALATION_REASON,
  ESCALATION_REASON,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import {
  createCaseViaGetHelp,
  skipWhenUnconfigured,
  type CreatedCase,
} from "../../utils/caseFlows";
import { CASE_DETAIL, CASE_ESCALATION } from "../../utils/selectors";
import {
  permanentWriteSkipReason,
  permanentWritesAllowed,
} from "../../utils/permanentWrites";

withSession(test);

/** The level a newly created case starts at, and the one it moves to. */
const START_LEVEL = "0";
const NEXT_LEVEL = "EL1";
/** The ceiling — ESCALATION_MAX_LEVEL_ID in supportConstants.ts. */
const MAX_LEVEL = 5;

const project = PROJECTS[ProjectType.SUBSCRIPTION];
const caseInput = CASE_INPUT[ProjectType.SUBSCRIPTION];

/**
 * Creates an S4 case and escalates it EL0 → EL1, asserting the whole way.
 *
 * Shared by both tests: de-escalation needs an escalated case to act on, and
 * building one through the UI is the only way to get a case this account is
 * permitted to de-escalate — at EL1 that right belongs to a CS admin, a project
 * Lead, or whoever raised the escalation.
 *
 * @param page - Test page.
 * @returns The created case's id and number.
 */
async function escalateNewCase(page: Page): Promise<CreatedCase> {
  // The fixture is what makes this an S4 case; assert it rather than trusting
  // the name, so a change to the shared fixture cannot quietly turn this into
  // a test that escalates an S1.
  expect(caseInput.severity, "this spec covers S4 specifically").toBe(
    "S4(Query)",
  );

  const created = await createCaseViaGetHelp(page, project, caseInput);
  console.log(
    `Created case to escalate (${ProjectType.SUBSCRIPTION}, ` +
      `${caseInput.severity}): ${created.number ?? created.id}`,
  );

  // Creation must have landed on the case detail page — that is where the
  // Escalate action lives, and the rest of this depends on it.
  await expect(page).toHaveURL(new RegExp(CASE_DETAIL.pathSegment));

  const caseDetail = new CaseDetailPage(page);

  // The button's presence is itself a permission assertion: a new case is at
  // level 0, which is below the Lead-gated levels, so it must be offered
  // regardless of whether this account is a project Lead.
  await expect(caseDetail.escalateButton()).toBeVisible({ timeout: 60_000 });

  await caseDetail.openEscalateModal();

  // The modal must describe the step it is about to take. Current and next
  // level come from different sources — the case's own level vs the
  // ESCALATION_NEXT_LEVEL map — so an off-by-one between them is a real
  // failure mode worth pinning.
  await expect(caseDetail.escalationLevelChip(`EL${START_LEVEL}`)).toBeVisible();
  await expect(caseDetail.escalationLevelChip(NEXT_LEVEL)).toBeVisible();
  await expect(
    caseDetail
      .escalateDialog()
      .getByText(CASE_ESCALATION.notifiedRole[START_LEVEL], { exact: false })
      .first(),
  ).toBeVisible();

  // Reason is mandatory here, and the only thing standing between a stray click
  // and a real notification — so assert the guard before filling it.
  await expect(caseDetail.confirmEscalationButton()).toBeDisabled();

  const response = await caseDetail.confirmEscalation(ESCALATION_REASON);

  // Assert on the wire: the toast is fired from the mutation callback, so the
  // request body is the only direct evidence of what was actually recorded.
  const payload = response.request().postDataJSON() as {
    reason?: string;
    action?: string;
  };
  expect(payload.action, "escalation must be an ESCALATE action").toBe(
    "ESCALATE",
  );
  expect(payload.reason).toBe(ESCALATION_REASON);
  expect(response.url()).toContain(`/cases/${created.id}/escalations`);

  // The modal closes itself on success, and the detail page re-renders from
  // the case's refreshed `isEscalated` — De-escalate exists only once the
  // case is escalated, so it appearing is the page's own account of the
  // change rather than a restatement of the toast.
  await expect(caseDetail.escalateDialog()).toBeHidden({ timeout: 30_000 });
  await expect(caseDetail.deescalateButton()).toBeVisible({ timeout: 60_000 });

  // The header must now report the new level. The chip's text comes from the
  // API's own `escalationLevel.label` rather than a string the UI assembles
  // from the id, so this asserts the backend moved the case — the De-escalate
  // button above only proves `isEscalated` flipped.
  await expect(caseDetail.escalatedChip(NEXT_LEVEL)).toBeVisible({
    timeout: 60_000,
  });

  return created;
}

test.describe("Escalate Case", () => {
  // Creates a case first, so these need the create flow's budget on top of
  // their own.
  test.describe.configure({ timeout: 180_000 });

  // Every test here raises a case that cannot be deleted AND escalates it,
  // which notifies real people — the Team Lead at EL1, and up the chain to the
  // CEO at EL5. That is an acceptable cost when someone runs this deliberately
  // and an unacceptable one for a routine full-suite run or a scheduled job, so
  // it is opt-in for the same reason case-matrix is.
  test.beforeEach(() => {
    test.skip(
      !permanentWritesAllowed(),
      permanentWriteSkipReason(
        "a support case, and escalations that notify real people (up to CEO " +
          "level at EL5)",
      ),
    );
  });

  test(`${ProjectType.SUBSCRIPTION} — escalates a newly created S4 case`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const created = await escalateNewCase(page);
    const caseDetail = new CaseDetailPage(page);

    // The Escalation tab holds the audit trail, and it is a separate fetch from
    // the case itself, so it is a genuinely independent confirmation.
    await caseDetail.openEscalationTab();
    await expect(
      caseDetail.detailsText(CASE_ESCALATION.history.emptyMessage),
    ).toHaveCount(0);

    // EL1 must appear as the record's new level. Exactly one record exists at
    // this point, so one EL1 chip: a second would mean the escalation was
    // recorded twice.
    await expect(caseDetail.escalationHistoryLevel(1)).toHaveCount(1, {
      timeout: 60_000,
    });

    // EL0 appears more than once by design — as this record's previous level
    // and as the initial state the panel pins at the bottom — so it is asserted
    // as present rather than counted.
    await expect(
      caseDetail.escalationHistoryLevel(0).first(),
    ).toBeVisible();

    // The reason is stored on the record and shown in the trail, which is what
    // makes these escalations identifiable later in a shared environment.
    await expect(
      caseDetail.detailsText(ESCALATION_REASON).first(),
    ).toBeVisible();

    console.log(
      `Escalated case ${created.number ?? created.id} from ` +
        `EL${START_LEVEL} to ${NEXT_LEVEL} ` +
        `(${CASE_ESCALATION.notifiedRole[START_LEVEL]} notified)`,
    );
  });

  test(`${ProjectType.SUBSCRIPTION} — de-escalates an escalated case`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    // Escalating first is not setup that could be skipped: de-escalation needs
    // a case already at EL1, and raising it here is also what earns this
    // account the right to reverse it.
    const created = await escalateNewCase(page);
    const caseDetail = new CaseDetailPage(page);

    // The starting condition the test exists to reverse.
    await expect(caseDetail.escalatedChip(NEXT_LEVEL)).toBeVisible();

    await caseDetail.openDeescalateModal();
    await expect(
      caseDetail
        .deescalateDialog()
        .getByText(CASE_ESCALATION.deescalateModal.subtitle),
    ).toBeVisible();

    // Unlike escalation, the reason is optional here — Confirm is gated only on
    // the request being in flight. Asserting it is already enabled with the
    // field untouched pins that difference, which is otherwise easy to
    // regress into a required field.
    await expect(caseDetail.confirmDeescalationButton()).toBeEnabled();

    const response = await caseDetail.confirmDeescalation(DEESCALATION_REASON);

    // Same endpoint as escalation — only `action` distinguishes them, so it is
    // the one field worth asserting on the wire.
    const payload = response.request().postDataJSON() as {
      reason?: string;
      action?: string;
    };
    expect(payload.action, "de-escalation must be a DEESCALATE action").toBe(
      "DEESCALATE",
    );
    expect(payload.reason).toBe(DEESCALATION_REASON);
    expect(response.url()).toContain(`/cases/${created.id}/escalations`);

    // What the user asked for: the header chip must be gone. `isEscalated`
    // drives both the chip and the De-escalate button, so both disappearing is
    // the page re-rendering from a genuinely un-escalated case.
    await expect(caseDetail.deescalateDialog()).toBeHidden({ timeout: 30_000 });
    await expect(caseDetail.escalatedChip(NEXT_LEVEL)).toBeHidden({
      timeout: 60_000,
    });
    await expect(caseDetail.deescalateButton()).toHaveCount(0, {
      timeout: 60_000,
    });

    // Escalate comes back, since the case is once again escalatable.
    await expect(caseDetail.escalateButton()).toBeVisible({ timeout: 60_000 });

    // De-escalating removes the escalation from the case, not from the record:
    // the trail still shows the EL1 that happened, which is what makes it an
    // audit trail rather than current state.
    //
    // EL1 now appears exactly twice, and the count is the point — the trail
    // holds two records, and the level is written on both ends of the journey:
    // as the escalation's new level, and as the de-escalation's previous one.
    // (EL0 correspondingly appears three times: the escalation's previous
    // level, the de-escalation's new level, and the pinned initial state.)
    await caseDetail.openEscalationTab();
    await expect(caseDetail.escalationHistoryLevel(1)).toHaveCount(2, {
      timeout: 60_000,
    });

    console.log(
      `De-escalated case ${created.number ?? created.id} from ${NEXT_LEVEL}`,
    );
  });

  test(`${ProjectType.SUBSCRIPTION} — escalates a case to EL5`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    // Five escalations plus a case creation, each a round trip with its own
    // refetch, so this needs well beyond the describe's budget.
    test.setTimeout(420_000);

    const created = await escalateNewCase(page);
    const caseDetail = new CaseDetailPage(page);

    // escalateNewCase has already taken 0 → 1, so resume at 1 and climb to 5.
    for (let from = 1; from < MAX_LEVEL; from++) {
      const next = `EL${from + 1}`;
      const notified = CASE_ESCALATION.notifiedRole[String(from)];

      // Levels 3 and 4 are Lead-only. A non-Lead account simply is not offered
      // the button, so without this the test would fail on a missing element
      // and read like a broken locator rather than a permission boundary
      // working as designed.
      const leadOnly: readonly string[] = CASE_ESCALATION.leadRequiredFromLevels;
      if (leadOnly.includes(String(from))) {
        const offered = await caseDetail.escalateButton().count();
        test.skip(
          offered === 0,
          `EL${from} → ${next} requires the account to be a project Lead ` +
            `(ESCALATION_LEAD_REQUIRED_FROM_LEVEL). Case ` +
            `${created.number ?? created.id} is left at EL${from}.`,
        );
      }

      await expect(caseDetail.escalateButton()).toBeVisible({ timeout: 60_000 });
      await caseDetail.openEscalateModal();

      // Each step must name its own from/to pair and notified role — walking
      // the ladder is exactly where an off-by-one in ESCALATION_NEXT_LEVEL
      // would show up, and a single-step test cannot catch it.
      await expect(caseDetail.escalationLevelChip(`EL${from}`)).toBeVisible();
      await expect(caseDetail.escalationLevelChip(next)).toBeVisible();
      await expect(
        caseDetail
          .escalateDialog()
          .getByText(notified, { exact: false })
          .first(),
      ).toBeVisible();

      const reason = `${ESCALATION_REASON} Step EL${from} to ${next}.`;
      const response = await caseDetail.confirmEscalation(reason);

      const payload = response.request().postDataJSON() as {
        reason?: string;
        action?: string;
      };
      expect(payload.action).toBe("ESCALATE");
      expect(payload.reason).toBe(reason);

      await expect(caseDetail.escalateDialog()).toBeHidden({ timeout: 30_000 });
      await expect(caseDetail.escalatedChip(next)).toBeVisible({
        timeout: 60_000,
      });

      console.log(
        `Escalated ${created.number ?? created.id}: EL${from} → ${next} ` +
          `(${notified} notified)`,
      );
    }

    // EL5 is the ceiling (ESCALATION_MAX_LEVEL_ID): the action row stops
    // offering Escalate entirely, which is the only thing preventing a sixth
    // level being requested.
    await expect(caseDetail.escalatedChip(`EL${MAX_LEVEL}`)).toBeVisible({
      timeout: 60_000,
    });
    await expect(caseDetail.escalateButton()).toHaveCount(0, {
      timeout: 60_000,
    });

    // One record per step, so the trail holds five. Each intermediate level is
    // written twice — as one step's new level and the next step's previous one
    // — while EL5 appears once, having never been escalated from.
    await caseDetail.openEscalationTab();
    await expect(caseDetail.escalationHistoryLevel(MAX_LEVEL)).toHaveCount(1, {
      timeout: 60_000,
    });
    for (let level = 1; level < MAX_LEVEL; level++) {
      await expect(
        caseDetail.escalationHistoryLevel(level),
        `EL${level} should appear on both the step into it and the step out`,
      ).toHaveCount(2);
    }

    console.log(
      `Escalated case ${created.number ?? created.id} to EL${MAX_LEVEL}`,
    );
  });
});
