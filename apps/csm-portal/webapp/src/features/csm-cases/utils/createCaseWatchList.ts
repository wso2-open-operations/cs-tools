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

import type { BeProjectContact, BeUser } from "@api/backend/types";
import { joinStaffName } from "@utils/staffNameMarker";

const REGISTERED_STATE = "REGISTERED";

/**
 * One person on the create-case watch list. Keyed by `email`: the create
 * request takes emails, which also covers a contact with no linked user.
 */
export interface WatcherCandidate {
  email: string;
  name: string;
  /** Where the candidate came from; drives only the picker's secondary text. */
  kind: "contact" | "internal";
}

/** Case-insensitive, trimmed key used to de-duplicate and compare watchers. */
export function watcherKey(email: string): string {
  return email.trim().toLowerCase();
}

/**
 * Whether a project contact has completed registration. The wire value is an
 * upstream state string ("registered", "invited", ...), so it is compared
 * trimmed and case-insensitively; a missing state is not registered.
 */
export function isRegisteredContact(contact: Pick<BeProjectContact, "registrationState">): boolean {
  return (contact.registrationState ?? "").trim().toUpperCase() === REGISTERED_STATE;
}

/**
 * Whether a project contact may be offered (and pre-selected) as a watcher:
 * registered, with an address to send, and passing the same role rule the
 * customer portal applies (admin, integration user, portal user, or any
 * contact who is not a security contact).
 */
export function isEligibleWatcher(
  contact: Pick<BeProjectContact, "registrationState" | "email" | "roles" | "isCsIntegrationUser">,
): boolean {
  if (!isRegisteredContact(contact) || watcherKey(contact.email ?? "") === "") return false;
  const roles = (contact.roles ?? []).map((r) => r.trim().toUpperCase());
  const hasRole = (code: string): boolean => roles.includes(code);
  return (
    hasRole("ADMIN") ||
    !!contact.isCsIntegrationUser ||
    hasRole("PORTAL_USER") ||
    !hasRole("SECURITY_CONTACT")
  );
}

/** Display name for a candidate, falling back to the address. */
export function watcherLabel(candidate: Pick<WatcherCandidate, "name" | "email">): string {
  return candidate.name.trim() ? `${candidate.name.trim()} (${candidate.email})` : candidate.email;
}

/** Eligible, de-duplicated watcher candidates for a project's contacts, sorted by name. */
export function eligibleContactWatchers(
  contacts: readonly BeProjectContact[] | undefined,
): WatcherCandidate[] {
  const seen = new Set<string>();
  const out: WatcherCandidate[] = [];
  for (const c of contacts ?? []) {
    if (!isEligibleWatcher(c)) continue;
    const email = (c.email ?? "").trim();
    const key = watcherKey(email);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ email, name: (c.name ?? "").trim(), kind: "contact" });
  }
  return out.sort((a, b) => watcherLabel(a).localeCompare(watcherLabel(b)));
}

/** An internal user as a candidate, or null when they have no address to key on. */
export function internalUserWatcher(user: BeUser): WatcherCandidate | null {
  const email = (user.email ?? "").trim();
  if (!email) return null;
  const name = joinStaffName(user.firstName, user.lastName);
  return { email, name, kind: "internal" };
}

/** The user's picks relative to the contact pre-fill: removals and additions. */
export interface WatchListEdits {
  /** {@link watcherKey}s of pre-filled contacts the engineer removed. */
  removed: string[];
  /** People the engineer added (contacts or internal users). */
  added: WatcherCandidate[];
}

export const NO_WATCH_LIST_EDITS: WatchListEdits = { removed: [], added: [] };

/**
 * The effective watch list: the pre-filled contacts minus what was removed,
 * plus what was added. Computed rather than stored so contacts that load after
 * the engineer started editing still pre-fill, while earlier edits are never
 * overwritten.
 */
export function resolveWatchList(
  prefill: readonly WatcherCandidate[],
  edits: WatchListEdits,
): WatcherCandidate[] {
  const removed = new Set(edits.removed);
  const seen = new Set<string>();
  const out: WatcherCandidate[] = [];
  for (const c of [...prefill.filter((p) => !removed.has(watcherKey(p.email))), ...edits.added]) {
    const key = watcherKey(c.email);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(c);
  }
  return out;
}

export function addWatcherEdit(edits: WatchListEdits, who: WatcherCandidate): WatchListEdits {
  const key = watcherKey(who.email);
  return {
    removed: edits.removed.filter((k) => k !== key),
    added: edits.added.some((a) => watcherKey(a.email) === key) ? edits.added : [...edits.added, who],
  };
}

export function removeWatcherEdit(edits: WatchListEdits, email: string): WatchListEdits {
  const key = watcherKey(email);
  return {
    removed: edits.removed.includes(key) ? edits.removed : [...edits.removed, key],
    added: edits.added.filter((a) => watcherKey(a.email) !== key),
  };
}
