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
// An opt-in gate for specs that create records which cannot be deleted.
//
// Some flows here have no undo: `POST /cases` has no delete counterpart, so a
// case — and a security report, which is a case — is permanent once raised. The
// specs covering those flows deliberately create unconditionally, because a
// create-only-if-missing guard means the write path stops being exercised the
// moment the record exists. The cost is that every run leaves real,
// customer-visible data in whatever environment E2E_BASE_URL names.
//
// That is a reasonable cost when someone runs the spec deliberately, and an
// unreasonable one when a full-suite run or a scheduled job does it silently.
// This gate draws that line: the write happens only when a human says so.
//
// Not a substitute for cleanup — there is no cleanup to be had. It only ensures
// the accumulation is chosen rather than incidental.
//

/** The environment variable that opts a run in. */
export const PERMANENT_WRITES_VAR = "E2E_ALLOW_PERMANENT_WRITES";

/**
 * Whether this run is allowed to create records that cannot be removed.
 *
 * Accepts `1`, `true` or `yes`, so it reads naturally from a shell and from CI
 * variable editors that only offer string values.
 *
 * @returns True when the run has opted in.
 */
export function permanentWritesAllowed(): boolean {
  const raw = process.env[PERMANENT_WRITES_VAR]?.trim().toLowerCase();
  return raw === "1" || raw === "true" || raw === "yes";
}

/**
 * The skip reason shown when a run has not opted in.
 *
 * States what would be created and how to allow it, because a bare "skipped"
 * on a test whose name says "create" invites someone to assume it is broken.
 *
 * @param creates - What the spec would create, e.g. "up to 12 support cases".
 * @returns A reason string for `test.skip`.
 */
export function permanentWriteSkipReason(creates: string): string {
  return (
    `Creates ${creates} that cannot be deleted. Set ` +
    `${PERMANENT_WRITES_VAR}=1 to run it — deliberately, and preferably not ` +
    `against a shared environment.`
  );
}
