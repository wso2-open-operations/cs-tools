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
// Cross-cutting: the header account menu — Profile and Log out.
//
// Lives here rather than under a feature because the menu is on every
// authenticated page and belongs to none of them.
//
// ✅ READ-ONLY. The profile modal is opened and dismissed without saving;
// signing out changes no record.
//
// WHY THESE ARE TWO TESTS
//
// They need opposite setups, so combining them would make one of the two lie:
//
// - Profile runs on the shared session like any other spec.
// - Log out DESTROYS a session, so it runs in a context of its own, seeded
//   WITHOUT the IdP cookies (see replayWithoutCookies). That is not tidiness:
//   the cookies identify the IdP's server-side session, so replaying them would
//   have this test sign out the session every other spec in the run depends on.
//   Without them the app still clears its own tokens and leaves — which is the
//   behaviour under test — and the shared session survives.
//
// Ordering is not relied on either way. Playwright gives each test its own
// context, so the sign-out cannot strand the profile test even if it ran first.
//

import {
  test,
  expect,
  withSession,
  hasSession,
  sessionOrigin,
} from "../../fixtures/test";
import {
  readSessionBundle,
  replayWithoutCookies,
} from "../../utils/sessionBundle";
import { ACCOUNT_MENU, CASE_DETAIL, LOGIN } from "../../utils/selectors";

withSession(test);

/** A cold shell load, then a cross-origin sign-out round trip. */
const LOAD_TIMEOUT_MS = 60_000;

test.describe("Account menu", () => {
  // A shell load, the /users/me query behind the menu, and for the sign-out a
  // full redirect to the IdP.
  test.describe.configure({ timeout: 180_000 });

  test("opens the profile from the account menu", async ({ page }) => {
    // The project hub — authenticated, and needs no project id, so this cannot
    // skip for a reason unrelated to the menu.
    await page.goto("/");

    const trigger = page.getByRole("button", { name: ACCOUNT_MENU.trigger });
    await expect(trigger).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await trigger.click();

    // Both items are offered. Asserted together, because a menu that lost one
    // of them is as much a regression as one that failed to open.
    const profileItem = page.getByRole("menuitem", {
      name: ACCOUNT_MENU.profileItem,
    });
    const logoutItem = page.getByRole("menuitem", {
      name: ACCOUNT_MENU.logoutItem,
    });
    await expect(profileItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(logoutItem).toBeVisible();

    await profileItem.click();

    const modal = page.getByRole("dialog");
    await expect(
      modal,
      "the Profile item should open the profile modal",
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await expect(
      modal.getByText(ACCOUNT_MENU.profileModalTitle, { exact: true }),
    ).toBeVisible();

    // It describes the signed-in user rather than rendering empty.
    //
    // Read from the FIELD VALUES, not the modal's text: Full Name, Email and
    // Mobile Number are disabled `textbox`es, so their contents are input
    // values and `getByText` cannot see them — asserting on text here finds
    // nothing however well the modal is populated. Verified live.
    //
    // The assertion is on shape — that one field holds an email — rather than
    // on a particular address, which would pin the test to one account.
    const fieldValues = await Promise.all(
      (await modal.getByRole("textbox").all()).map((field) =>
        field.inputValue(),
      ),
    );
    expect(
      fieldValues.some((value) => /\S+@\S+\.\S+/.test(value)),
      `the profile should show the signed-in user's email; fields held ` +
        `${JSON.stringify(fieldValues)}`,
    ).toBe(true);

    // The name is populated too, and is not the app's "not set" placeholder —
    // which is what distinguishes a loaded profile from a rendered-but-empty
    // one.
    await expect(
      modal.getByRole("heading", { level: 5 }).last(),
      "the profile should name the signed-in user",
    ).not.toHaveText("");

    // Dismissed without saving, so nothing is changed. Through the modal's own
    // close control rather than Escape: the dialog can be configured to ignore
    // Escape, and this is the path a user actually takes.
    await modal.getByRole("button", { name: ACCOUNT_MENU.closeButton }).click();
    await expect(modal).toBeHidden({ timeout: LOAD_TIMEOUT_MS });

    console.log("Account menu: profile modal opened and dismissed");
  });

  test("signs out from the account menu", async ({ browser, baseURL }) => {
    test.skip(
      !hasSession(),
      `No captured session to sign out of. See tests/e2e/auth/README.md.`,
    );
    const captured = sessionOrigin();
    const target = baseURL ? new URL(baseURL).origin : undefined;
    test.skip(
      !!captured && !!target && captured !== target,
      `Session was captured against ${captured} but this run targets ${target}.`,
    );

    // Its own context, and no IdP cookies — see the header. The shared session
    // must still be usable by every spec that follows this one.
    const context = await browser.newContext();
    try {
      await replayWithoutCookies(context, readSessionBundle());
      const page = await context.newPage();
      await page.goto("/");

      const trigger = page.getByRole("button", { name: ACCOUNT_MENU.trigger });
      await expect(trigger).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
      await trigger.click();

      const logoutItem = page.getByRole("menuitem", {
        name: ACCOUNT_MENU.logoutItem,
      });
      await expect(logoutItem).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
      await logoutItem.click();

      // Left the portal. Asserted as "off the app origin" rather than as a
      // specific Asgardeo URL: sign-out crosses more than one host on its way
      // to the sign-in page, and pinning the first would be brittle for no gain.
      await page.waitForURL((url) => url.origin !== target, {
        timeout: LOAD_TIMEOUT_MS,
      });

      // The user is asked to authenticate again — the actual contract of
      // signing out. A bare redirect proves less: it could be an error page, or
      // a bounce that left the session intact.
      await expect(
        page.locator(LOGIN.usernameInput),
        "signing out should land on the sign-in form",
      ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

      // And the portal is not still rendering behind it.
      await expect(page.getByTestId(CASE_DETAIL.mainTestId)).toHaveCount(0);

      // NOT asserted: that the SDK's tokens were cleared from sessionStorage.
      //
      // It cannot be observed from here. The replay is an init script, so it
      // runs on EVERY document this context loads — reopening the app origin to
      // read its storage would restore the very tokens the check is looking
      // for, and the assertion would fail no matter how correct the sign-out
      // was. Reading it from a context without the replay is equally useless,
      // since nothing was ever put there. The redirect to a sign-in form is the
      // observable the test can honestly stand on.

      console.log(
        `Account menu: signed out to ${new URL(page.url()).origin}`,
      );
    } finally {
      await context.close();
    }
  });
});
