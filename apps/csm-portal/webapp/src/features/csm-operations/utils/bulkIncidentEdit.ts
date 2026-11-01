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

import { isIncidentTerminal } from "@features/csm-operations/utils/incidents";
import type { BeUpdateIncidentPayload } from "@api/backend/types";

/** How a bulk edit changes each incident's watch list. The API only replaces the whole list. */
export type BulkWatchListMode = "add" | "remove" | "replace";

export const BULK_WATCH_LIST_MODES: ReadonlyArray<{ value: BulkWatchListMode; label: string }> = [
  { value: "add", label: "Add these people" },
  { value: "remove", label: "Remove these people" },
  { value: "replace", label: "Replace the watch list with these people" },
];

/**
 * The one change a bulk edit applies to every selected incident: `patch` holds
 * only the fields set in the dialog (each incident keeps its own value for the
 * rest); the watch list is described apart, because adding or removing people
 * needs each incident's own list.
 */
export interface BulkIncidentEdit {
  patch: BeUpdateIncidentPayload;
  watchList?: { mode: BulkWatchListMode; userIds: string[] };
}

/** What is sent to one incident, or why nothing is. */
export type BulkIncidentPlan = { patch: BeUpdateIncidentPayload } | { skip: string };

/** Whether a watch list change needs each incident's current watchers. */
export function bulkWatchListNeedsCurrent(edit: BulkIncidentEdit): boolean {
  return !!edit.watchList && edit.watchList.mode !== "replace";
}

/** The watch list after `mode` is applied to `current`, in a stable order. */
export function nextWatchList(current: string[], mode: BulkWatchListMode, userIds: string[]): string[] {
  if (mode === "replace") return [...new Set(userIds)];
  if (mode === "add") return [...new Set([...current, ...userIds])];
  const removed = new Set(userIds);
  return current.filter((id) => !removed.has(id));
}

const sameMembers = (a: string[], b: string[]): boolean =>
  a.length === b.length && a.every((id) => b.includes(id));

/**
 * The PATCH for one incident in a bulk edit. A closed or cancelled incident is
 * never edited; an incident is never made its own parent; adding or removing
 * watchers starts from the incident's own list (`currentWatchers`, `undefined`
 * when it could not be read: the watch list is then left alone and said so),
 * and a list that would not change is not sent. The backend stays the
 * authority for everything else (e.g. who may close an incident): what it
 * refuses for one incident is reported for that one.
 */
export function bulkIncidentPlan(
  incident: { id?: string | null; state?: string | null },
  edit: BulkIncidentEdit,
  currentWatchers?: string[],
): BulkIncidentPlan {
  if (isIncidentTerminal(incident.state)) return { skip: "Closed or cancelled: it can't be edited." };
  if (edit.patch.parentIncidentId && edit.patch.parentIncidentId === incident.id) {
    return { skip: "An incident can't be its own parent incident." };
  }
  const patch: BeUpdateIncidentPayload = { ...edit.patch };
  if (edit.watchList) {
    if (edit.watchList.mode !== "replace" && currentWatchers === undefined) {
      return { skip: "Couldn't read its watch list, so it was not changed. Try again." };
    }
    const current = currentWatchers ?? [];
    const next = nextWatchList(current, edit.watchList.mode, edit.watchList.userIds);
    if (edit.watchList.mode === "replace" || !sameMembers(current, next)) patch.watchList = next;
  }
  if (Object.keys(patch).length === 0) return { skip: "Nothing to change: its watch list already is as asked." };
  return { patch };
}

/** "Assign to me" for one incident: the signed-in user as its assignee. */
export function assignIncidentToMePlan(
  incident: { state?: string | null; assignedTo?: { id?: string | null } | null },
  userId: string,
): BulkIncidentPlan {
  if (isIncidentTerminal(incident.state)) return { skip: "Closed or cancelled: it can't be edited." };
  if (incident.assignedTo?.id === userId) return { skip: "Already assigned to you." };
  return { patch: { assignedEngineerId: userId } };
}
