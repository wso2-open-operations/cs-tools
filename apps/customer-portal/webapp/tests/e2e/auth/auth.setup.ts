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
// Signs in once and writes the session bundle every spec replays.
//
// Runs as a Playwright setup project — `playwright.config.ts` makes the test
// projects depend on it, so a run always starts from a session minted moments
// earlier. That is the point of doing this at all: a hand-captured session
// expires about an hour after capture, and a suite that outlives it fails
// halfway through on redirects to the sign-in page, which looks like a
// application bug and is not.
//
// It writes the SAME bundle shape the capture flow produced — origin plus
// localStorage, sessionStorage and cookies — because Playwright's own
// `storageState` does not carry sessionStorage, and that is exactly where the
// Asgardeo SDK keeps its tokens. The replay in fixtures/test.ts is unchanged.
//
// ⚠️ Skips rather than fails when credentials are absent, so a fresh clone can
// still run against a hand-captured session without being blocked on secrets.
//

import { test as setup, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { LoginPage, RECAPTCHA_GUIDANCE } from "./LoginPage";
import {
  describeCredentials,
  hasCredentials,
  loginIdentity,
  readCredentials,
} from "./credentials";
import {
  DEFAULT_SESSION,
  hasSession,
  sessionIdentity,
  sessionPath,
} from "../fixtures/test";


/**
 * How much life is left in the session bundle already on disk.
 *
 * Used to decide whether a failed sign-in is fatal: if a usable bundle exists,
 * blocking the whole run on the sign-in would be worse than proceeding with it.
 *
 * @returns Minutes remaining, or null when there is no readable bundle.
 */
function existingSessionMinutesLeft(): number | null {
  if (!hasSession(DEFAULT_SESSION)) return null;
  try {
    const raw = fs.readFileSync(sessionPath(DEFAULT_SESSION), "utf8");
    let latest = 0;
    for (const token of raw.match(/eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]+\./g) ?? []) {
      const payload = token.split(".")[1];
      const claims = JSON.parse(
        Buffer.from(payload, "base64url").toString("utf8"),
      ) as { exp?: number };
      if (claims.exp && claims.exp > latest) latest = claims.exp;
    }
    if (!latest) return null;
    return Math.round((latest * 1000 - Date.now()) / 60_000);
  } catch {
    return null;
  }
}

setup("sign in and capture the session", async ({ page, baseURL }) => {
  const identity = loginIdentity();

  setup.skip(
    !hasCredentials(identity),
    `No credentials for the "${identity}" identity — falling back to whatever ` +
      "session bundle is already on disk. See tests/e2e/auth/README.md.",
  );
  setup.setTimeout(180_000);

  const origin = new URL(baseURL!).origin;

  // Logged every run, because which identity signed in changes what the UI
  // offers: as a Portal user the admin surfaces are absent by design, and a
  // spec expecting them fails looking like missing UI rather than wrong
  // identity. Defaults to admin; switch with E2E_LOGIN_IDENTITY=portal.
  console.log(`Signing in as ${describeCredentials(identity)} at ${origin}`);

  const login = new LoginPage(page);
  await page.goto("/");

  try {
    await login.signIn(readCredentials(identity), origin);
  } catch (error) {
    // Every test project depends on this one, so failing here fails the entire
    // run. When a usable session is already on disk that trade is a bad one:
    // warn loudly and let the suite proceed on the existing bundle. Only fail
    // when there is genuinely no way to authenticate.
    const minutesLeft = existingSessionMinutesLeft();
    const reason = (error as Error).message;

    // Only fall back to a bundle belonging to THIS identity. Reusing another
    // account's session would quietly run the whole suite as the wrong user,
    // which is a worse outcome than stopping here.
    const onDisk = sessionIdentity(DEFAULT_SESSION);
    const identityMatches = !onDisk || onDisk === identity;

    if (!identityMatches) {
      throw new Error(
        `Sign-in failed, and the session on disk belongs to "${onDisk}" rather ` +
          `than "${identity}" — falling back to it would run the suite as the ` +
          `wrong user.\n${reason}`,
      );
    }

    if (minutesLeft !== null && minutesLeft > 1) {
      console.warn(
        `\n⚠️  Automated sign-in did not complete: ${reason}\n` +
          `    Falling back to the session bundle on disk, which has about ` +
          `${minutesLeft} minute(s) left. Specs running past that will fail on ` +
          `redirects to the sign-in page.\n`,
      );
      setup.skip(true, `Sign-in unavailable; using the existing session bundle.`);
      return;
    }

    // Surface the cause rather than a raw timeout. reCAPTCHA is the expected
    // blocker here and is a tenant setting, not something to work around.
    throw new Error(
      `Sign-in failed and there is no usable session on disk ` +
        `(${minutesLeft === null ? "no bundle" : `expired ${-minutesLeft}m ago`}).\n` +
        `${reason}\n\n${RECAPTCHA_GUIDANCE}`,
    );
  }

  // Authentication succeeded — but that is not the same as being allowed in.
  // The portal shows "Portal Access Required" to an account that signs in
  // correctly yet has no entitlement, and the distinction matters: one is a
  // broken login, the other is a provisioning gap, and they need entirely
  // different people to fix.
  const accessDenied = await page
    .getByRole("heading", { name: /portal access required/i })
    .first()
    .waitFor({ state: "visible", timeout: 15_000 })
    .then(() => true)
    .catch(() => false);

  if (accessDenied) {
    throw new Error(
      `Signed in successfully as "${identity}" ` +
        `(${process.env[identity === "admin" ? "E2E_USERNAME" : "E2E_PORTAL_USERNAME"]}) ` +
        "— username, password and TOTP were all accepted — but the portal " +
        "answered \"Portal Access Required\": the account has no access to the " +
        "customer portal.\n" +
        "This is an entitlement problem, not a login one. The account needs " +
        "portal access on this environment, or use one that already has it. " +
        "Note the page also offers \"System Mode\", which suggests the account " +
        "may be provisioned as an integration/system user — those are mutually " +
        "exclusive with the ordinary portal roles.",
    );
  }

  // Signed in, allowed in, and the app has actually booted — the token lands in
  // sessionStorage during boot, so snapshotting before this captures nothing.
  await expect(
    page.getByRole("button", { name: "Get Help", exact: true }),
  ).toBeVisible({ timeout: 60_000 });

  const storage = await page.evaluate(() => {
    const dump = (store: Storage): Record<string, string> => {
      const out: Record<string, string> = {};
      for (let i = 0; i < store.length; i++) {
        const key = store.key(i);
        if (key) out[key] = store.getItem(key) ?? "";
      }
      return out;
    };
    return {
      localStorage: dump(window.localStorage),
      sessionStorage: dump(window.sessionStorage),
    };
  });

  expect(
    Object.keys(storage.sessionStorage).length,
    "signed in but sessionStorage is empty — the SDK stores its tokens there, " +
      "so a bundle without it replays as signed-out",
  ).toBeGreaterThan(0);

  // IdP-domain cookies too: they let the SDK's silent refresh succeed mid-run
  // when the short-lived access token expires.
  const cookies = await page.context().cookies();

  // `identity` is persisted alongside the origin for the same reason the origin
  // is: a bundle that does not say whose session it holds can be replayed under
  // a different E2E_LOGIN_IDENTITY, and the whole suite then runs as the wrong
  // user — as a Portal user the admin surfaces are simply absent, so it fails
  // looking like missing UI rather than a wrong identity.
  const bundle = { origin, identity, ...storage, cookies };
  const target = sessionPath(DEFAULT_SESSION);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, `${JSON.stringify(bundle, null, 2)}\n`);

  console.log(
    `Captured "${identity}" session → ${path.relative(process.cwd(), target)} ` +
      `(${Object.keys(storage.sessionStorage).length} sessionStorage keys, ` +
      `${cookies.length} cookies)`,
  );
});
