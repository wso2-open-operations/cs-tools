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
// Sessions for the LOCAL docker-compose stack (webapp on :3000, mock OIDC on
// :9100) — the customers the local seed registers on its projects.
//
// Nothing here talks to Asgardeo or a real tenant. The local identity provider
// signs in ANY email with no credential check, so a session is minted by driving
// the app's own sign-in once (auth/local-session.setup.ts) and the bundle is then
// replayed exactly like a hand-captured one (`withSession` in fixtures/test.ts).
//
// What a bundle is NOT: it is not reusable across runs forever. The local IdP's
// tokens live one hour and it issues no refresh token, so every helper here
// treats an expired bundle like a missing one — the test is SKIPPED with the
// command that mints a new one, rather than failing on a redirect to the
// sign-in page that reads as an application bug.
//

import fs from "node:fs";
import type { Browser, BrowserContext, BrowserContextOptions } from "@playwright/test";
import {
  hasSession,
  openContextAs,
  sessionOrigin,
  sessionPath,
  withSession,
  type test,
} from "../fixtures/test";

/**
 * The customer personas of the local seed (scripts/csm-compose/seed-entity-service.sql,
 * "Local seed personas" in entity-service/CLAUDE.md).
 *
 * All four are role `customer` (EXTERNAL) and registered `PORTAL_USER` contacts of
 * exactly one project — which is what scopes them: the customer backend asks
 * entity-service for the projects the signed-in email is a registered contact of,
 * and nothing else.
 */
export const LOCAL_PERSONAS = {
  dave: {
    email: "dave.mendis@example.com",
    /** A contact of "Example Corp Production" (project 401), with erin. */
    project: "Example Corp Production",
  },
  erin: {
    email: "erin.jayawardena@example.com",
    project: "Example Corp Production",
  },
  mira: {
    email: "mira.santos@lumenworks.example",
    /** The seed-generator names this project at random per database; the seed
     * finds it by this name. Its id is NOT fixed — look it up by name. */
    project: "Lumen Works Platform",
  },
  noel: {
    email: "noel.prasad@lumenworks.example",
    project: "Lumen Works Platform",
  },
} as const;

export type LocalPersona = keyof typeof LOCAL_PERSONAS;

/** The seeded Example Corp project (dave, erin) — fixed id, unlike Lumen's. */
export const EXAMPLE_CORP_PROJECT_ID = "00000000-0000-0000-0000-000000000401";

/** Whether a name is one of the persona keys. */
export function isLocalPersona(name: string): name is LocalPersona {
  return Object.prototype.hasOwnProperty.call(LOCAL_PERSONAS, name);
}

/**
 * Name of the session bundle for a persona: `storageState/<name>.json`.
 *
 * Prefixed `local-` so a local bundle can never be mistaken for, or overwrite, the
 * staging `session.json` the rest of the suite replays.
 */
export function localSessionName(persona: LocalPersona): string {
  return `local-${persona}`;
}

/** Matches the JWTs inside a bundle's sessionStorage (the Asgardeo SDK keeps its
 * tokens there as JSON strings). */
const JWT_PATTERN = /eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*/g;

/**
 * Minutes of life left in a bundle's tokens, judged by the LATEST expiry among the
 * JWTs it holds (the same rule auth.setup.ts uses for the staging bundle).
 *
 * @param name - Bundle name, as in `storageState/<name>.json`.
 * @returns Whole minutes (negative once expired), or null when the bundle is absent,
 * unreadable or holds no token.
 */
export function sessionMinutesLeft(name: string): number | null {
  if (!hasSession(name)) return null;
  try {
    const raw = fs.readFileSync(sessionPath(name), "utf8");
    let latest = 0;
    for (const token of raw.match(JWT_PATTERN) ?? []) {
      const claims = JSON.parse(
        Buffer.from(token.split(".")[1], "base64url").toString("utf8"),
      ) as { exp?: number };
      if (claims.exp && claims.exp > latest) latest = claims.exp;
    }
    if (!latest) return null;
    return Math.floor((latest * 1000 - Date.now()) / 60_000);
  } catch {
    return null;
  }
}

/** The command that mints a persona's session, for skip messages. */
export function mintCommand(persona: LocalPersona): string {
  return (
    `E2E_LOCAL_PERSONA=${persona} node_modules/.bin/playwright test ` +
    `--config=playwright.local-auth.config.ts`
  );
}

/** Reachability of the app under test, probed once per worker. */
const reachable = new Map<string, Promise<boolean>>();

/**
 * Whether something answers at `url`.
 *
 * A running stack that is merely slow is not "down", so the probe is generous
 * (5 s) and any HTTP answer, whatever its status, counts as up.
 *
 * @param url - Origin of the app under test.
 */
export function isReachable(url: string): Promise<boolean> {
  let probe = reachable.get(url);
  if (!probe) {
    probe = fetch(url, { signal: AbortSignal.timeout(5_000) })
      .then(() => true)
      .catch(() => false);
    reachable.set(url, probe);
  }
  return probe;
}

/**
 * Configures a spec file to run signed in as a local persona, SKIPPING each test
 * cleanly when it cannot: no bundle minted yet, a bundle minted against another
 * origin than the run targets, an expired bundle, or no stack answering at the
 * base URL.
 *
 * Builds on `withSession`, so the missing-bundle and wrong-origin skips are the
 * ones the rest of the suite already has; this adds expiry and reachability, which
 * only a local stack has to worry about.
 *
 * Usage at the top of a spec:
 *   withLocalSession(test, "dave");
 *
 * @param t - The `test` object of the spec file.
 * @param persona - Which seeded customer to sign in as.
 */
export function withLocalSession(t: typeof test, persona: LocalPersona): void {
  const name = localSessionName(persona);
  withSession(t, name);

  t.beforeEach(async ({ baseURL }) => {
    const left = sessionMinutesLeft(name);
    t.skip(
      left !== null && left < 2,
      `The local '${name}' session ${left !== null && left < 0 ? "expired" : "is about to expire"} ` +
        `(the local identity provider's tokens live one hour). Mint a new one: ${mintCommand(persona)}`,
    );
    t.skip(
      baseURL !== undefined && !(await isReachable(baseURL)),
      `Nothing answers at ${baseURL}. Start the local stack (docker-compose up -d), or point ` +
        "E2E_BASE_URL at the customer webapp it publishes.",
    );
  });
}

/**
 * Opens a SECOND signed-in browser context as another local persona, for a spec
 * in which two customers act (dave approves, erin looks). Skips the test, with
 * the reason, exactly as {@link withLocalSession} does for the first persona: no
 * bundle minted, expired, or minted for another origin than the run targets.
 *
 * The caller closes the returned context.
 *
 * @param t - The `test` object of the spec file.
 * @param browser - The `browser` fixture.
 * @param persona - Which seeded customer to sign in as.
 * @param options - Context options; pass the run's `baseURL` (a hand-made context
 * does not inherit it) and any `timezoneId` the spec pins.
 */
export async function openLocalContext(
  t: typeof test,
  browser: Browser,
  persona: LocalPersona,
  options?: BrowserContextOptions,
): Promise<BrowserContext> {
  const name = localSessionName(persona);
  t.skip(
    !hasSession(name),
    `No '${name}' session. Mint one: ${mintCommand(persona)}`,
  );
  const left = sessionMinutesLeft(name);
  t.skip(
    left !== null && left < 2,
    `The local '${name}' session ${left !== null && left < 0 ? "expired" : "is about to expire"}. ` +
      `Mint a new one: ${mintCommand(persona)}`,
  );
  const captured = sessionOrigin(name);
  const target = options?.baseURL ? new URL(options.baseURL).origin : undefined;
  t.skip(
    !!captured && !!target && captured !== target,
    `Session '${name}' was minted for ${captured} but this run targets ${target}.`,
  );
  return openContextAs(browser, name, options);
}
