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

import { formatUtcDateForDisplay } from "@utils/dateTime";

export type ActionTone = "success" | "warning" | "error" | "default";

export interface ClosureProcessAction {
  /** The raw key, e.g. `actionSendEmailNotification`. */
  key: string;
  /** Plain-language name of the action. */
  title: string;
  /** Humanised result, e.g. "Successful". */
  result: string;
  tone: ActionTone;
}

export interface ClosureProcessTrack {
  /** The raw key, e.g. `based_on_due_invoices`. */
  key: string;
  title: string;
  /** The raw event type when it was a non-empty string, else `null`. */
  eventType: string | null;
  /** Plain-language description of where the track stands. */
  label: string;
  actions: ClosureProcessAction[];
}

export type ClosureProcessRecord =
  /** Nothing recorded (null, empty string, empty object). */
  | { kind: "empty" }
  /** At least one track was understood. `leftover` is the pretty-printed JSON
   *  of anything else in the blob (unknown keys, malformed tracks), or `null`
   *  when everything was understood. */
  | { kind: "tracks"; tracks: ClosureProcessTrack[]; leftover: string | null }
  /** Nothing could be understood; show `raw` verbatim. */
  | { kind: "unrecognised"; raw: string };

const TRACK_TITLES: Record<string, string> = {
  based_on_subscription_end_date: "Subscription end date",
  based_on_due_invoices: "Due invoices",
  based_on_compliance: "Compliance",
};

const ACTION_TITLES: Record<string, string> = {
  actionSendEmailNotification: "Email notification",
  actionServicePortalAnnouncement: "Portal announcement",
  actionSuspendProject: "Project suspension",
};

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function humanize(value: string): string {
  const spaced = value
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_\s]+/g, " ")
    .trim()
    .toLowerCase();
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function pretty(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? String(value);
  } catch {
    return String(value);
  }
}

/**
 * Plain-language label for a track's event type. `suspendedOn` is only used
 * for the `suspend` event. An unknown event type is returned as-is (never
 * hidden); a missing one reads "No event recorded".
 */
export function closureEventLabel(
  eventType: string | null | undefined,
  suspendedOn?: string | null,
): string {
  const raw = typeof eventType === "string" ? eventType.trim() : "";
  if (!raw) return "No event recorded";
  const key = raw.toLowerCase();

  const notice = /^(\d+)_days?_notice$/.exec(key);
  if (notice) return `${notice[1]}-day notice sent`;
  if (key === "suspend_notice") return "Suspension notice sent";
  if (key === "suspend") {
    const date = suspendedOn ? (formatUtcDateForDisplay(suspendedOn) ?? suspendedOn) : null;
    return date ? `Suspended on ${date}` : "Suspended";
  }
  if (key === "open") return "Open (no notice yet)";
  return raw;
}

/** Chip tone for an action result; anything unknown is neutral. */
export function actionResultTone(result: string): ActionTone {
  switch (result.trim().toUpperCase()) {
    case "SUCCESSFUL":
    case "SUCCESS":
    case "COMPLETED":
      return "success";
    case "FAILED":
    case "FAILURE":
    case "ERROR":
      return "error";
    case "PENDING":
    case "IN_PROGRESS":
    case "RETRY":
      return "warning";
    default:
      // IGNORED, SKIPPED, NOT_APPLICABLE and anything unrecognised.
      return "default";
  }
}

function scalarText(value: unknown): string | null {
  if (typeof value === "string") return value.trim() || null;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return null;
}

function parseTrack(key: string, value: unknown): ClosureProcessTrack | null {
  if (!isPlainObject(value)) return null;

  const eventType = scalarText(value.event_type);
  const suspendedOn = scalarText(value.suspended_on);

  const actions: ClosureProcessAction[] = [];
  for (const [field, raw] of Object.entries(value)) {
    if (!/^action[A-Z]/.test(field)) continue;
    const result = scalarText(raw);
    if (result === null) continue;
    actions.push({
      key: field,
      title: ACTION_TITLES[field] ?? humanize(field.replace(/^action/, "")),
      result: humanize(result),
      tone: actionResultTone(result),
    });
  }

  // A section with none of the fields this view understands is not a track.
  if (eventType === null && suspendedOn === null && actions.length === 0) return null;

  return {
    key,
    title: TRACK_TITLES[key] ?? humanize(key.replace(/^based_on_/, "")),
    eventType,
    label: closureEventLabel(eventType, suspendedOn),
    actions,
  };
}

/**
 * Defensive reader for a project's `suspensionProcessState`: never throws,
 * whatever the backend sent (null, "", a JSON string instead of an object,
 * arrays, scalars, unknown keys, missing fields). Anything it cannot present
 * is surfaced verbatim in `leftover`/`raw` rather than dropped.
 */
export function parseClosureProcessState(value: unknown): ClosureProcessRecord {
  let data: unknown = value;

  if (typeof data === "string") {
    const text = data.trim();
    if (!text) return { kind: "empty" };
    try {
      data = JSON.parse(text);
    } catch {
      return { kind: "unrecognised", raw: text };
    }
    if (data === null) return { kind: "empty" };
  }

  if (data === null || data === undefined) return { kind: "empty" };
  if (!isPlainObject(data)) return { kind: "unrecognised", raw: pretty(data) };
  if (Object.keys(data).length === 0) return { kind: "empty" };

  const tracks: ClosureProcessTrack[] = [];
  const leftover: Record<string, unknown> = {};
  for (const [key, section] of Object.entries(data)) {
    const track = parseTrack(key, section);
    if (track) tracks.push(track);
    else leftover[key] = section;
  }

  if (tracks.length === 0) return { kind: "unrecognised", raw: pretty(data) };
  return {
    kind: "tracks",
    tracks,
    leftover: Object.keys(leftover).length > 0 ? pretty(leftover) : null,
  };
}
