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
// Sign-in credentials, read from the environment.
//
// Never hardcoded, and never in a tracked file: these are a real staging
// password and a TOTP seed, and a seed in git is a second factor that has
// stopped being a second factor. They live in `.env.e2e.local`, which
// .gitignore excludes via `.env.*.local` and which playwright.config.ts loads
// ahead of the tracked `.env.e2e`. In CI, set the same names as masked secrets.
//
// Nothing here is logged. If you add diagnostics, print `describeCredentials()`
// rather than the values — a password in a CI log is a password that has leaked.
//

/** The credential set a UI sign-in needs. */
export interface Credentials {
  username: string;
  password: string;
  /** Base32 TOTP seed for the second factor. */
  totpSecret: string;
}

/**
 * The accounts this suite can sign in as.
 *
 * These are not interchangeable, and picking the wrong one produces failures
 * that look like missing UI rather than a wrong identity:
 *
 * - `admin` holds the ServiceNow `customer_admin` role, so it sees the admin
 *   surfaces — user management, the AI Assistant toggle, and anything else
 *   gated on it.
 * - `portal` is an ordinary Portal user. Admin-only controls are simply absent
 *   for it, by design.
 *
 * See the capability table in apps/customer-portal/CLAUDE.md: admin comes from a
 * ServiceNow user role, while Lead/Portal/Security come from project membership
 * — two independent sources, and the usual cause of confusion in this area.
 */
export type IdentityName = "admin" | "portal";

/** Env-var prefix per identity. `admin` keeps the original unprefixed names so
 * existing setups and CI secrets keep working unchanged. */
const PREFIXES: Record<IdentityName, string> = {
  admin: "E2E",
  portal: "E2E_PORTAL",
};

/**
 * Which identity a sign-in uses, from `E2E_LOGIN_IDENTITY`.
 *
 * Defaults to `admin` (customer-portal-sub-admin-stg@wso2support.com), because
 * most of this suite exercises admin surfaces — user management, the AI
 * Assistant toggle — which are simply absent for a Portal user and would fail
 * looking like missing UI rather than a permissions boundary.
 *
 * Override with `E2E_LOGIN_IDENTITY=portal` to cover what a non-admin sees.
 */
export function loginIdentity(): IdentityName {
  const raw = process.env.E2E_LOGIN_IDENTITY?.trim().toLowerCase();
  if (raw === "admin" || raw === "portal") return raw;
  if (raw) {
    throw new Error(
      `E2E_LOGIN_IDENTITY="${raw}" is not a known identity. ` +
        `Use one of: ${Object.keys(PREFIXES).join(", ")}.`,
    );
  }
  return "admin";
}

const varsFor = (identity: IdentityName) => ({
  USERNAME_VAR: `${PREFIXES[identity]}_USERNAME`,
  PASSWORD_VAR: `${PREFIXES[identity]}_PASSWORD`,
  TOTP_VAR: `${PREFIXES[identity]}_TOTP_SECRET`,
});

/**
 * Reads the credentials, failing with an actionable message when incomplete.
 *
 * Names the missing variables and where to set them — a bare "undefined" here
 * sends people looking in the wrong place, because the file that supplies these
 * is git-ignored and so invisible in a fresh clone.
 *
 * @returns The credential set.
 */
export function readCredentials(
  identity: IdentityName = loginIdentity(),
): Credentials {
  const { USERNAME_VAR, PASSWORD_VAR, TOTP_VAR } = varsFor(identity);
  const username = process.env[USERNAME_VAR];
  const password = process.env[PASSWORD_VAR];
  const totpSecret = process.env[TOTP_VAR];

  const missing = [
    username ? null : USERNAME_VAR,
    password ? null : PASSWORD_VAR,
    totpSecret ? null : TOTP_VAR,
  ].filter(Boolean);

  if (missing.length > 0) {
    throw new Error(
      `Missing sign-in credentials: ${missing.join(", ")}. ` +
        "Set them in webapp/.env.e2e.local (git-ignored) or as environment " +
        "variables. See tests/e2e/auth/README.md.",
    );
  }

  return {
    username: username!,
    password: password!,
    totpSecret: totpSecret!,
  };
}

/** Whether a full credential set is available, for skipping rather than failing. */
export function hasCredentials(
  identity: IdentityName = loginIdentity(),
): boolean {
  const { USERNAME_VAR, PASSWORD_VAR, TOTP_VAR } = varsFor(identity);
  return Boolean(
    process.env[USERNAME_VAR] &&
      process.env[PASSWORD_VAR] &&
      process.env[TOTP_VAR],
  );
}

/**
 * A safe one-line description of the configured credentials, for logs.
 *
 * @returns The username and whether the other two are set — never their values.
 */
export function describeCredentials(
  identity: IdentityName = loginIdentity(),
): string {
  const { USERNAME_VAR, PASSWORD_VAR, TOTP_VAR } = varsFor(identity);
  return (
    `${identity}: ${process.env[USERNAME_VAR] ?? "(no username)"} ` +
    `[password ${process.env[PASSWORD_VAR] ? "set" : "MISSING"}, ` +
    `totp ${process.env[TOTP_VAR] ? "set" : "MISSING"}]`
  );
}
