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


import type { ProjectContact } from "@features/settings/types/users";

const REGISTERED_STATUS = "REGISTERED";

/**
 * Whether a project contact has completed registration. Invited, re-invited and
 * deactivated contacts are not registered, and the backend rejects a case with
 * a watcher who is not a registered contact of the project.
 *
 * @param {ProjectContact} contact - The project contact.
 * @returns {boolean} True when the contact's membership status is REGISTERED.
 */
export function isRegisteredContact(
  contact: Pick<ProjectContact, "membershipStatus">,
): boolean {
  return (contact.membershipStatus ?? "").trim().toUpperCase() === REGISTERED_STATUS;
}

/**
 * Whether a project contact may be offered (and pre-selected) as a case watcher.
 *
 * The backend rejects a case-creation request when any watcher is not a
 * registered contact of the project, so invited, re-invited and deactivated
 * contacts must never appear here. The role rule (admin, integration user,
 * portal user, or any non-security contact) is kept as is.
 *
 * The membership status is compared trimmed and case-insensitively because the
 * wire value is an upstream state string (e.g. "REGISTERED", "INVITED",
 * "RE-INVITED", "DEACTIVATED"); a missing status is treated as not registered.
 *
 * @param {ProjectContact} contact - The project contact.
 * @returns {boolean} True when the contact is eligible as a watcher.
 */
export function isEligibleWatcher(
  contact: Pick<
    ProjectContact,
    | "membershipStatus"
    | "isCsAdmin"
    | "isCsIntegrationUser"
    | "isPortalUser"
    | "isSecurityContact"
  >,
): boolean {
  const isRegistered = isRegisteredContact(contact);
  const hasEligibleRole =
    contact.isCsAdmin ||
    contact.isCsIntegrationUser ||
    !!contact.isPortalUser ||
    !contact.isSecurityContact;
  return isRegistered && hasEligibleRole;
}

/** The watchers a user added and removed while editing, as emails. */
export interface WatchListChange {
  /** In the edited list but not in the one the edit started from. */
  addWatchers: string[];
  /** In the list the edit started from but not in the edited one. */
  removeWatchers: string[];
}

/**
 * What the user actually changed between the watch list an edit started from
 * and the one they ended with.
 *
 * The case's watch list is saved as this change, never as the edited list: the
 * backend applies a change against the list stored at that moment, so a watcher
 * a colleague added while this page was open stays on the case. Sending the
 * whole edited list instead replaces whatever is stored and drops them.
 *
 * Emails are compared trimmed and case-insensitively; the first spelling seen
 * is the one returned. Someone left alone appears in neither list.
 *
 * @param {string[]} baseline - The watch list the edit started from.
 * @param {string[]} edited - The watch list as the user left it.
 * @returns {WatchListChange} The additions and removals.
 */
export function diffWatchList(baseline: string[], edited: string[]): WatchListChange {
  const key = (email: string): string => email.trim().toLowerCase();
  const unique = (emails: string[]): Map<string, string> => {
    const byKey = new Map<string, string>();
    for (const email of emails) {
      const k = key(email);
      if (k && !byKey.has(k)) byKey.set(k, email.trim());
    }
    return byKey;
  };
  const before = unique(baseline);
  const after = unique(edited);
  return {
    addWatchers: [...after].filter(([k]) => !before.has(k)).map(([, email]) => email),
    removeWatchers: [...before].filter(([k]) => !after.has(k)).map(([, email]) => email),
  };
}
