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
// RBAC: how far each user type may escalate a case.
//
// Subscription only, by request — the escalation ceiling is a property of the
// USER, so covering one project type establishes it.
//
// The expectation under test:
//   - Security Contact cannot escalate at all.
//   - Admin and Lead can escalate up to EL3.
//   - Portal user can escalate all the way to EL5.
//
// ⚠️ That last pair disagrees with the app's own source, and the disagreement is
// worth knowing before reading a failure. ESCALATION_LEAD_REQUIRED_FROM_LEVEL in
// supportConstants.ts gates levels 3 and 4 on `isCurrentUserLead`, and
// CLAUDE.md states "Lead is required to escalate past EL3" — which would make
// LEAD, not PORTAL, the role that reaches EL5. Both cannot be right. The
// expectations here are as specified; if PORTAL stops at EL3 and LEAD goes
// further, the specification is what needs revisiting, not this test. Account
// naming and project membership are independent in this app, so an account
// called "portal" may well hold `isLead` on its membership.
//
// Each test signs in as its own account, so `withSession` is deliberately
// absent — the same approach as the other rbac specs.
//
// ⚠️ UNGATED, and expensive in a way that is not obvious. Every test raises a
// case that cannot be deleted AND escalates it, which notifies REAL PEOPLE —
// Team Lead at EL1, and up to CCO/CRO and CEO at EL4/EL5. A full run of this
// file pages that chain. Worth knowing before adding it to anything scheduled.
//

import { test, expect } from "../../fixtures/test";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import { hasRoleCredentials, type RoleKey } from "../../auth/credentials";
import { signInAsRole } from "../../auth/signInAsRole";
import {
  CASE_INPUT,
  ESCALATION_REASON,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { CASE_DETAIL, CASE_ESCALATION } from "../../utils/selectors";
import { createCaseDirect, skipWhenUnconfigured } from "../../utils/caseFlows";

// These specs perform a REAL sign-in — a password and a TOTP code are typed into
// the page. The chromium project records trace and video `retain-on-failure`, and
// both capture keystrokes and DOM, so a failing test would write those
// credentials into an artefact that CI then uploads and the container emails.
//
// Disabled here rather than in signInAsRole: a helper cannot change project
// recording settings, and doing it globally would strip the diagnostics every
// other spec relies on. The `auth` setup project is configured the same way for
// the same reason (see playwright.config.ts).
test.use({ trace: "off", video: "off" });

const PROJECT_TYPE = ProjectType.SUBSCRIPTION;
const PROJECT_KEY = "SUB" as const;

const project = PROJECTS[PROJECT_TYPE];
const caseInput = CASE_INPUT[PROJECT_TYPE];

/** The ceiling — ESCALATION_MAX_LEVEL_ID in supportConstants.ts. */
const MAX_LEVEL = 5;

/**
 * How far each user type may escalate.
 *
 * `maxLevel` is the highest level the role can REACH. 0 means the role is not
 * offered escalation at all, so no case of theirs ever leaves EL0.
 */
const EXPECTATIONS: { role: RoleKey; maxLevel: number }[] = [
  { role: "ADMIN", maxLevel: 3 },
  { role: "PORTAL", maxLevel: 3 },
  { role: "LEAD", maxLevel: MAX_LEVEL },
  // ⚠️ SECURITY = 0 is a REQUIREMENT, not a description of the current build.
  // `showEscalateButton` in CaseDetailsActionRow gates only on the escalation
  // level, the case not being Closed, EL5 being the ceiling, and `isCurrentUserLead`
  // for levels 3-4. There is no Security-contact check anywhere in that path, so
  // as implemented a Security Contact IS offered escalation at EL0 and this case
  // will fail.
  //
  // Left as specified on purpose: flipping it to match the code would turn a
  // missing restriction into a green test and erase the requirement. If the
  // restriction is not wanted, delete this entry rather than invert it — an
  // inverted assertion would then claim to verify something nobody asked for.
  { role: "SECURITY", maxLevel: 0 },
];

test.describe("RBAC — escalation ceiling per user type", () => {
  // A real sign-in, a case creation, and up to five escalations with their
  // refetches.
  test.describe.configure({ timeout: 600_000 });

  for (const { role, maxLevel } of EXPECTATIONS) {
    const summary =
      maxLevel === 0
        ? "cannot escalate"
        : `can escalate to EL${maxLevel} and no further`;

    test(`${role.toLowerCase()} ${summary}`, async ({ page, baseURL }) => {
      test.skip(
        !hasRoleCredentials(PROJECT_KEY, role),
        `No credentials for ${PROJECT_KEY}/${role} — set ` +
          `E2E_${PROJECT_KEY}_${role}_* in webapp/.env.e2e.local.`,
      );
      skipWhenUnconfigured(project);

      const origin = new URL(baseURL!).origin;

      // Signs in AND verifies the session is really this account — see
      // signInAsRole. Without that check a wrong session does not fail,
      // it inverts the result.
      await signInAsRole(page, PROJECT_KEY, role, origin);

      // Each role escalates a case of ITS OWN. Escalation acts on a specific
      // record, and reusing one across roles would leave later tests unable to
      // start from EL0.
      //
      // Created by URL rather than through Get Help: that button opens the chat
      // whenever the project's assistant is enabled, and only an admin can turn
      // the assistant off — so routing through it would make three of these four
      // tests depend on a global flag they cannot control.
      const created = await createCaseDirect(page, project, caseInput);
      console.log(
        `RBAC escalation: ${role} created ${created.number ?? created.id}`,
      );

      const caseDetail = new CaseDetailPage(page);
      await expect(page).toHaveURL(new RegExp(CASE_DETAIL.pathSegment));
      await expect(caseDetail.caseNumber()).toBeVisible({ timeout: 60_000 });

      if (maxLevel === 0) {
        // Not offered at all. A fresh case sits at EL0, which every other role
        // can escalate from — so the button's absence here is the restriction
        // itself rather than a state that happens to forbid it.
        await expect(
          caseDetail.escalateButton(),
          `${role} must not be offered escalation on a case at EL0. If this ` +
            "fails, the restriction is absent from the app rather than broken " +
            "in the test: showEscalateButton (CaseDetailsActionRow) has no " +
            "Security-contact check, so the button renders for every user type " +
            "at EL0. Implement the gate, or drop SECURITY from EXPECTATIONS.",
        ).toHaveCount(0, { timeout: 60_000 });

        console.log(
          `RBAC escalation: ${role} — not offered escalation, as expected`,
        );
        return;
      }

      // Climb, one level at a time, asserting each step names its own from/to
      // pair. Walking it is what catches an off-by-one in the level map, which
      // a single-step check cannot.
      for (let from = 0; from < maxLevel; from += 1) {
        const next = `EL${from + 1}`;

        await expect(
          caseDetail.escalateButton(),
          `${role} should be able to escalate from EL${from} to ${next}`,
        ).toBeVisible({ timeout: 60_000 });

        await caseDetail.openEscalateModal();
        await expect(caseDetail.escalationLevelChip(`EL${from}`)).toBeVisible();
        await expect(caseDetail.escalationLevelChip(next)).toBeVisible();

        const reason = `${ESCALATION_REASON} Step EL${from} to ${next}.`;
        const response = await caseDetail.confirmEscalation(reason);
        const payload = response.request().postDataJSON() as {
          action?: string;
        };
        expect(payload.action).toBe("ESCALATE");

        await expect(caseDetail.escalateDialog()).toBeHidden({
          timeout: 30_000,
        });
        await expect(caseDetail.escalatedChip(next)).toBeVisible({
          timeout: 60_000,
        });

        console.log(
          `RBAC escalation: ${role} — EL${from} → ${next} ` +
            `(${CASE_ESCALATION.notifiedRole[String(from)]} notified)`,
        );
      }

      // And no further. Two different reasons produce this, so the message
      // distinguishes them: at EL5 the action row stops offering Escalate to
      // everyone, whereas below it the stop is the role's own ceiling.
      const ceilingReason =
        maxLevel === MAX_LEVEL
          ? `EL${MAX_LEVEL} is the maximum for any user`
          : `${role} should not be permitted past EL${maxLevel}`;

      await expect(caseDetail.escalateButton(), ceilingReason).toHaveCount(0, {
        timeout: 60_000,
      });

      console.log(
        `RBAC escalation: ${role} reached EL${maxLevel} and stopped, as ` +
          `expected (${created.number ?? created.id})`,
      );
    });
  }
});
