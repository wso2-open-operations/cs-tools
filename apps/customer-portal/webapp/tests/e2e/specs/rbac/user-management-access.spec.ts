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
// RBAC: Settings → User Management → "Add User" is admin-only.
//
// The button is gated on the ServiceNow `customer_admin` USER role
// (SettingsPage.tsx reads it from GET /users/me), NOT on project membership.
// That distinction is the point of this spec: Lead, Security Contact and Portal
// user are membership roles, and a contact can hold any of them — even
// `isCsAdmin` on their membership — and still see no admin UI without the
// ServiceNow role.
//
// Covers the full matrix: three project types x four user types = twelve
// accounts. Each test signs in as its own, so this is the one spec that does NOT
// replay the shared session — `withSession` is deliberately absent and each test
// performs a real login. That is slower, and it is the only way to compare what
// different users see.
//
// Read-only: the button's presence is asserted, never clicked, so no user is
// ever created.
//
// ⚠️ TWELVE real sign-ins per run, serially. The IdP rate-limits repeated logins
// in quick succession — a run that fails with the flow restarting at the
// username screen is throttling, not a defect. Narrow with -g to one project
// type while iterating, and re-run the full matrix after a pause.
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

/**
 * The three project types, each with its env-var prefix.
 *
 * `CLOUD` is spelt "Cloud Support" in the fixtures while the accounts are named
 * `customer-portal-cloud-*` — the same project, two naming conventions, mapped
 * here once so the mismatch cannot mislead anyone reading a failure.
 */
const PROJECTS_UNDER_TEST: { type: ProjectType; key: ProjectKey }[] = [
  { type: ProjectType.SUBSCRIPTION, key: "SUB" },
  { type: ProjectType.MANAGED_CLOUD_SUBSCRIPTION, key: "MS_SUB" },
  { type: ProjectType.CLOUD_SUPPORT, key: "CLOUD" },
];

/**
 * Who may see "Add User".
 *
 * Only ADMIN carries the ServiceNow role the button is gated on. The other
 * three are project-membership roles, which grant no admin surfaces at all —
 * and that must hold on every project type, since the gate is a USER role and
 * has nothing to do with the project.
 */
const EXPECTATIONS: { role: RoleKey; canAddUsers: boolean }[] = [
  { role: "ADMIN", canAddUsers: true },
  { role: "PORTAL", canAddUsers: false },
  { role: "LEAD", canAddUsers: false },
];

test.describe("RBAC — Add User is admin-only", () => {
  // A real sign-in (username, password, TOTP) plus a settings load, per test.
  test.describe.configure({ timeout: 300_000 });

  for (const { type: PROJECT_TYPE, key: PROJECT_KEY } of PROJECTS_UNDER_TEST) {
    const project = PROJECTS[PROJECT_TYPE];

    test.describe(PROJECT_TYPE, () => {
      for (const { role, canAddUsers } of EXPECTATIONS) {
        test(`${role.toLowerCase()} ${canAddUsers ? "sees" : "does not see"} Add User`, async ({
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
          await settings.openTab(SETTINGS.tabs.userManagement);

          // Wait for the tab's own content before judging the button's
          // absence: "not rendered yet" and "not permitted" look identical
          // until the list lands, which would make the non-admin assertions
          // pass for the wrong reason.
          //
          // The marker is the User column header plus a row carrying an email —
          // both present for every role. NOT `userRows()`, which identifies
          // rows by their edit control: that control is admin-only, so waiting
          // on it would hang for exactly the three roles under test here.
          await expect(
            settings.userTableHeader(SETTINGS.userManagement.headers[0]),
            "the user list should render for every role",
          ).toBeVisible({ timeout: 60_000 });
          await expect(settings.anyUserRows().first()).toBeVisible({
            timeout: 60_000,
          });

          if (canAddUsers) {
            await expect(
              settings.addUserButton(),
              "an admin must be offered Add User",
            ).toBeVisible({ timeout: 30_000 });
          } else {
            await expect(
              settings.addUserButton(),
              `${role} must NOT be offered Add User — it is gated on the ` +
                "ServiceNow customer_admin role, which only the admin account holds",
            ).toHaveCount(0);
          }

          console.log(
            `RBAC ${PROJECT_TYPE}/${role}: Add User ` +
              `${canAddUsers ? "present" : "absent"} as expected`,
          );
        });
      }
    });
  }
});
