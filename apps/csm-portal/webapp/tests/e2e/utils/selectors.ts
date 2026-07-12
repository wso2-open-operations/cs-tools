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
// The time-card feature is wired to the real csm-portal-backend — there is no
// seeded mock data, and no delete endpoint, so anything a spec creates via
// `POST /time-cards` becomes a permanent record in staging. Every card an E2E
// spec creates MUST carry E2E_TAG in its work-log comment so it's easy to
// find/identify later, and specs must never assume any pre-existing card.
//

/** Prefix every E2E-created work-log comment with this, followed by a
 * timestamp, so entries are identifiable and never collide across runs. */
export const E2E_TAG = "[E2E]";

export function e2eWorkLogComment(label: string): string {
  return `${E2E_TAG} ${label} — ${new Date().toISOString()}`;
}

/** Page-level accessible names used across the time-cards UI. */
export const TIMECARDS = {
  path: "/time-cards",
  heading: "Time cards",
  tabs: { mine: "My time sheets", approvals: "Approvals" },
} as const;

//
// Change requests are wired to the real csm-portal-backend too — POST
// /change-requests has no delete endpoint, so anything a spec creates
// becomes a permanent ServiceNow record. Same tagging rule as time cards.
//

/** Same E2E_TAG + label + timestamp format as {@link e2eWorkLogComment} —
 * kept as its own named export since it tags a different kind of record,
 * but delegates to avoid duplicating the format itself. */
export function e2eChangeRequestSubject(label: string): string {
  return e2eWorkLogComment(label);
}

export const CHANGE_REQUEST_CREATE = {
  path: "/operations/change-requests/new",
  heading: "New change request",
} as const;
