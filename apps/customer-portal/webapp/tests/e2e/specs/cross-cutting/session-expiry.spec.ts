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
// Cross-cutting: what happens when the session is no longer valid.
//
// ✅ READ-ONLY. Nothing is created or changed; the only side effect is a
// sign-in redirect inside this test's own throwaway browser context.
//
// ⚠️ This spec deliberately does NOT call `withSession`. Every other spec
// replays the captured session so the app boots signed-in — here the whole
// point is to boot with one that has expired, so the bundle is read, tampered
// with, and replayed by hand into a context of this test's own.
//
// WHAT "EXPIRED" HAS TO MEAN
//
// An expired access token on its own is not a dead session: `periodicTokenRefresh`
// is on, so the SDK is expected to refresh silently and the user should notice
// nothing. Asserting a redirect on that alone would be asserting a bug. A
// session is only genuinely gone when the refresh cannot rescue it either, so
// this test takes all three away together:
//
//   1. the access/id tokens are backdated past their own `expires_in`
//   2. the refresh token is replaced with one the IdP will reject
//   3. the IdP cookies are withheld, so the hidden silent-refresh iframe has no
//      server-side session to lean on
//
// Without (3) the IdP could re-authenticate silently and land the user back in
// the app — correct behaviour, but it would make this test assert nothing.
//
// The portal's own expiry path is in `useAuthApiClient`: a token error, one
// retry, then `signIn()` behind a module-scoped single-flight guard so
// concurrent failures cause exactly one redirect.
//

import {
  test,
  expect,
  hasSession,
  sessionOrigin,
} from "../../fixtures/test";
import { CASE_DETAIL, LOGIN } from "../../utils/selectors";
import {
  readSessionBundle,
  replayWithoutCookies,
  type SessionBundle,
} from "../../utils/sessionBundle";

/** A refresh token the IdP cannot honour, so the silent refresh fails rather
 * than quietly rescuing the session. Obviously fake, so a stray log line cannot
 * be mistaken for a real credential. */
const DEAD_REFRESH_TOKEN = "expired-by-e2e-test";

/** How long the SDK's boot, its failed refresh attempt and the redirect take. */
const REDIRECT_TIMEOUT_MS = 60_000;

/**
 * Backdates a captured bundle so its tokens are already expired.
 *
 * Works on the SDK's `session_data` entry, whose key carries an instance and
 * client id — matched by prefix rather than spelled out, so a recaptured
 * bundle from a different client still works.
 *
 * `created_at` is moved back by more than `expires_in`, which is what the SDK
 * compares; the refresh token is then neutered so the failure cannot be
 * recovered from. The id token is left byte-intact: it is a signed JWT whose
 * own `exp` has long passed in a backdated session, and rewriting it would only
 * produce a malformed token, which is a different failure.
 *
 * @param bundle - The captured session.
 * @returns A copy whose session is expired and unrefreshable.
 */
function expireSession(bundle: SessionBundle): SessionBundle {
  const storage = { ...(bundle.sessionStorage ?? {}) };

  const sessionKey = Object.keys(storage).find((key) =>
    key.startsWith("session_data-"),
  );
  if (!sessionKey) {
    throw new Error(
      "The captured session has no session_data entry to expire. Recapture it " +
        "— see tests/e2e/auth/README.md.",
    );
  }

  const data = JSON.parse(storage[sessionKey]) as {
    expires_in?: string | number;
    created_at?: number;
    refresh_token?: string;
  };

  const lifetimeSeconds = Number(data.expires_in ?? 3600);
  // A day beyond the token's own lifetime: far enough that no clock tolerance
  // (`tokenValidation.clockTolerance` is 300s) can make it look current.
  data.created_at = Date.now() - (lifetimeSeconds + 86_400) * 1000;
  data.refresh_token = DEAD_REFRESH_TOKEN;

  storage[sessionKey] = JSON.stringify(data);
  return { ...bundle, sessionStorage: storage };
}

test.describe("Session expiry", () => {
  // A cold boot, a failed token refresh against the real IdP, and a full
  // cross-origin redirect.
  test.describe.configure({ timeout: 180_000 });

  test("expired session redirects to sign-in", async ({ browser, baseURL }) => {
    test.skip(
      !hasSession(),
      `No captured session to expire. See tests/e2e/auth/README.md.`,
    );

    // Same guard withSession applies, and for the same reason: a bundle
    // captured elsewhere restores nothing, so the app would boot signed-out and
    // this test would "pass" without ever exercising expiry.
    const captured = sessionOrigin();
    const target = baseURL ? new URL(baseURL).origin : undefined;
    test.skip(
      !!captured && !!target && captured !== target,
      `Session was captured against ${captured} but this run targets ${target}.`,
    );

    const bundle = readSessionBundle();

    // Its own context, never the shared one: this leaves expired tokens and a
    // signed-out IdP behind it, which would strand any spec that followed.
    const context = await browser.newContext();
    try {
      await replayWithoutCookies(context, expireSession(bundle));

      const page = await context.newPage();

      // The project hub — behind AuthGuard, and needs no project id, so this
      // cannot skip for a reason unrelated to auth.
      await page.goto("/");

      // Left the portal entirely. Asserted as "not on the app origin" rather
      // than as a specific IdP URL: the flow crosses more than one Asgardeo
      // host, and pinning the first would make this brittle for no gain.
      await page.waitForURL((url) => url.origin !== target, {
        timeout: REDIRECT_TIMEOUT_MS,
      });

      // And arrived at a sign-in form — the thing the user is actually asked
      // for. A bare redirect could just as well be an error page.
      await expect(
        page.locator(LOGIN.usernameInput),
        "the expired session should land on the sign-in form",
      ).toBeVisible({ timeout: REDIRECT_TIMEOUT_MS });

      // The portal is not rendering underneath. Without this, a sign-in shown
      // in a dialog over a still-live page would pass.
      await expect(page.getByTestId(CASE_DETAIL.mainTestId)).toHaveCount(0);

      console.log(
        `Session expiry: expired session redirected to sign-in at ` +
          `${new URL(page.url()).origin}`,
      );
    } finally {
      await context.close();
    }
  });
});
