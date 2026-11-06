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
// RBAC: Settings → AI Assistant → the AI Chat Assistant (Novera) toggle is
// admin-only.
//
// Gated differently from Add User, which is worth knowing: the AI Assistant TAB
// is visible to every user type, and it is the SWITCH that is restricted — it
// renders DISABLED for anyone without the ServiceNow `customer_admin` role,
// alongside the hint "Users with Admin role can only update this setting". So a
// non-admin can see the setting and its current state, and cannot change it.
// Asserting the tab is absent would be wrong; asserting the switch is disabled
// is the actual contract.
//
// For an admin the toggle is exercised as a ROUND TRIP: whatever state it is
// in, it is flipped to the other and then flipped back. That leaves the project
// exactly as it was found, which matters because `hasAgent` changes what Get
// Help does for every other spec in the suite — a test that left it flipped
// would quietly break the create-case specs.
//
// Covers three project types x four user types. Each test signs in as its own
// account, so `withSession` is deliberately absent — the same approach as
// user-management-access.spec.ts, and the only way to compare user types.
//
// ⚠️ TWELVE real sign-ins per run, serially. The IdP rate-limits repeated logins
// in quick succession — a run that fails with the flow restarting at the
// username screen is throttling, not a defect. Narrow with -g while iterating.
//

import { test, expect } from "../../fixtures/test";
import { SettingsPage } from "../../pages/SettingsPage";
import {
  hasRoleCredentials,
  type ProjectKey,
  type RoleKey,
} from "../../auth/credentials";
import { signInAsRole } from "../../auth/signInAsRole";
import { PROJECTS, ProjectType } from "../../config/testData";
import { SETTINGS } from "../../utils/selectors";

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

/** The three project types, each with its credential env-var prefix. */
const PROJECTS_UNDER_TEST: { type: ProjectType; key: ProjectKey }[] = [
  { type: ProjectType.SUBSCRIPTION, key: "SUB" },
  { type: ProjectType.MANAGED_CLOUD_SUBSCRIPTION, key: "MS_SUB" },
  { type: ProjectType.CLOUD_SUPPORT, key: "CLOUD" },
];

/**
 * Who may change the assistant.
 *
 * Only ADMIN holds the ServiceNow role the switch is gated on. The other three
 * are project-membership roles, which grant no admin surfaces — and since the
 * gate is a USER role, that must hold identically on every project type.
 */
const EXPECTATIONS: { role: RoleKey; canEdit: boolean }[] = [
  { role: "ADMIN", canEdit: true },
  { role: "PORTAL", canEdit: false },
  { role: "LEAD", canEdit: false },
];

const novera = SETTINGS.aiAssistant.novera;

test.describe("RBAC — AI Assistant toggle is admin-only", () => {
  // A real sign-in (username, password, TOTP), a settings load, and for an
  // admin two writes with their refetches.
  test.describe.configure({ timeout: 300_000 });

  for (const { type: PROJECT_TYPE, key: PROJECT_KEY } of PROJECTS_UNDER_TEST) {
    const project = PROJECTS[PROJECT_TYPE];

    test.describe(PROJECT_TYPE, () => {
      for (const { role, canEdit } of EXPECTATIONS) {
        test(`${role.toLowerCase()} ${canEdit ? "can" : "cannot"} toggle Novera`, async ({
          page,
          baseURL,
        }) => {
          test.skip(
            !hasRoleCredentials(PROJECT_KEY, role),
            `No credentials for ${PROJECT_KEY}/${role} — set ` +
              `E2E_${PROJECT_KEY}_${role}_* in webapp/.env.e2e.local.`,
          );
          test.skip(!project.id, `${PROJECT_TYPE} needs a project id.`);

          const origin = new URL(baseURL!).origin;

          // Signs in AND verifies the session is really this account — see
          // signInAsRole. Without that check a wrong session does not fail,
          // it inverts the result.
          await signInAsRole(page, PROJECT_KEY, role, origin);

          const settings = new SettingsPage(page);
          await settings.open(project.id);
          await settings.openTab(SETTINGS.tabs.aiAssistant);

          // The tab is visible to everyone, so its content is the readiness
          // signal for both branches below.
          await expect(settings.capabilitiesSection()).toBeVisible({
            timeout: 60_000,
          });
          await expect(settings.noveraLabel()).toBeVisible();

          if (!canEdit) {
            // The switch is present but inert, and the page says why. Both are
            // asserted: a disabled switch without the hint would leave a user
            // guessing, and the hint without a disabled switch would be a
            // cosmetic-only restriction.
            await expect(
              settings.noveraToggle(),
              `${role} must not be able to change the assistant — the switch ` +
                "is gated on the ServiceNow customer_admin role",
            ).toBeDisabled({ timeout: 30_000 });

            await expect(
              settings.settingsText(novera.adminOnlyHint),
              "a non-admin should be told why the setting is read-only",
            ).toBeVisible();

            console.log(
              `RBAC ${PROJECT_TYPE}/${role}: Novera toggle disabled as expected`,
            );
            return;
          }

          //
          // Admin: the switch is live, so exercise it and put it back.
          //
          await expect(
            settings.noveraToggle(),
            "an admin must be able to change the assistant",
          ).toBeEnabled({ timeout: 30_000 });

          // Read the starting state only once it is interactive — the switch
          // renders disabled while project details load, and isChecked() does
          // not retry, so an early read reports a state still arriving.
          const initiallyEnabled = await settings.noveraToggle().isChecked();
          console.log(
            `RBAC ${PROJECT_TYPE}/${role}: Novera starts ` +
              `${initiallyEnabled ? "enabled" : "disabled"}`,
          );

          try {
            // Flip to the opposite state...
            const flipResponse = await settings.setNovera(
              project.id,
              !initiallyEnabled,
            );
            const flipPayload = flipResponse.request().postDataJSON() as {
              hasAgent?: boolean;
            };
            expect(
              flipPayload.hasAgent,
              "the write should carry the requested state",
            ).toBe(!initiallyEnabled);

            // ...and confirm it took. The switch reverts if the write is
            // rejected, so the chip — which reflects what the backend stored —
            // is the assertion that matters, not the click.
            if (initiallyEnabled) {
              await expect(settings.noveraToggle()).not.toBeChecked({
                timeout: 30_000,
              });
              await expect(
                settings.noveraChip(novera.inactiveChip),
              ).toBeVisible({ timeout: 30_000 });
            } else {
              await expect(settings.noveraToggle()).toBeChecked({
                timeout: 30_000,
              });
              await expect(settings.noveraChip(novera.activeChip)).toBeVisible({
                timeout: 30_000,
              });
            }
          } finally {
            // Back to how it was found, whatever happened above. `hasAgent`
            // decides whether Get Help opens the chat or the case form, so
            // leaving it flipped would break other specs rather than just this
            // one.
            await settings
              .setNovera(project.id, initiallyEnabled)
              .catch(() => undefined);
          }

          // And the restore really landed.
          if (initiallyEnabled) {
            await expect(settings.noveraToggle()).toBeChecked({
              timeout: 30_000,
            });
          } else {
            await expect(settings.noveraToggle()).not.toBeChecked({
              timeout: 30_000,
            });
          }

          console.log(
            `RBAC ${PROJECT_TYPE}/${role}: toggled off and back; restored to ` +
              `${initiallyEnabled ? "enabled" : "disabled"}`,
          );
        });
      }
    });
  }
});
