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
// Signs in as one project/role account and PROVES it is that account.
//
// The proof matters more here than anywhere else in the suite. An RBAC test
// asserts what a particular user may do, so signing in as the wrong one does
// not fail — it quietly inverts the result: a control correctly hidden from a
// Portal user looks like a defect if an admin was expected, and worse, a
// control an admin can see passes a "non-admin cannot see it" assertion if the
// session was never actually switched.
//
// Nothing in a plain `signIn` catches that. The identity is therefore read back
// from GET /users/me — the same endpoint the portal itself uses to decide
// whether someone holds the ServiceNow admin role — and compared against the
// account the credentials name.
//

import { expect, type Page } from "../fixtures/test";
import { CASE_DETAIL } from "../utils/selectors";
import { LoginPage } from "./LoginPage";
import {
  readRoleCredentials,
  roleUsername,
  type ProjectKey,
  type RoleKey,
} from "./credentials";

/** The parts of GET /users/me this needs. */
interface UserDetails {
  email?: string;
  userName?: string;
  roles?: string[];
}

/**
 * Signs in as the given project/role account and verifies the session is really
 * that account.
 *
 * @param page - Test page, which will be navigated to the portal.
 * @param project - Project key, e.g. "SUB".
 * @param role - Role key, e.g. "PORTAL".
 * @param origin - Portal origin to land on.
 * @returns The signed-in user's details, for a caller that wants the roles.
 */
export async function signInAsRole(
  page: Page,
  project: ProjectKey,
  role: RoleKey,
  origin: string,
): Promise<UserDetails> {
  const expectedUsername = roleUsername(project, role);
  console.log(`RBAC: signing in as ${role} (${expectedUsername})`);

  // Capture the identity call the shell makes on boot, before navigating.
  const userDetailsResponse = page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname.endsWith("/users/me") &&
      r.request().method() === "GET",
    { timeout: 120_000 },
  );

  await page.goto("/");
  await new LoginPage(page).signIn(readRoleCredentials(project, role), origin);

  // The identity call settles first: the shell keeps showing its loading state
  // until /users/me resolves, so nothing below is meaningful before it.
  const details = (await (await userDetailsResponse).json()) as UserDetails;

  // Authentication is not authorisation: an account can sign in correctly and
  // still be refused the portal, which would make a "cannot see X" assertion
  // pass for entirely the wrong reason.
  //
  // Asserting the denial heading is ABSENT is vacuous on its own: it holds
  // before the shell has rendered anything. The shell swaps its loading
  // progress bar for either the page or the denial heading in a single render
  // once /users/me settles, so wait for the bar to go inside <main> first. Only
  // then does the absence of the heading mean "authorised".
  const main = page.getByTestId(CASE_DETAIL.mainTestId);
  await expect(main, "the portal shell never rendered").toBeVisible({
    timeout: 60_000,
  });
  await expect(
    main.getByRole("progressbar"),
    "the portal shell is still loading",
  ).toHaveCount(0, { timeout: 60_000 });
  await expect(
    page.getByRole("heading", { name: /portal access required/i }),
    `${role} signed in but has no access to the portal — that is an ` +
      "entitlement gap, not an RBAC result",
  ).toHaveCount(0);

  const actual = details.email ?? details.userName ?? "(no email in /users/me)";

  // The check this module exists for. Compared case-insensitively because the
  // IdP echoes the address as typed while ServiceNow stores it normalised.
  expect(
    actual.toLowerCase(),
    `signed in as the wrong account — expected ${expectedUsername} for ` +
      `${project}/${role}, got ${actual}. Every assertion after this would be ` +
      "about the wrong user.",
  ).toBe(expectedUsername.toLowerCase());

  console.log(
    `RBAC: confirmed ${role} = ${actual}` +
      (details.roles?.length ? ` (roles: ${details.roles.join(", ")})` : ""),
  );

  return details;
}
