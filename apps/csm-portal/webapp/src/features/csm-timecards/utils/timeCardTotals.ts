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

import {
  ACTIVITY_KEYS,
  type ActivityBreakdown,
} from "@features/csm-timecards/types/timeCards";

/** A fresh breakdown with every activity at zero hours. */
export function emptyBreakdown(): ActivityBreakdown {
  return {
    analysisDebugging: 0,
    reproduce: 0,
    settingUp: 0,
    providingSolution: 0,
    answering: 0,
  };
}

/** Round to 2 decimals to avoid float drift in summed hours (e.g. 0.1 + 0.2). */
export function roundHours(value: number): number {
  return Math.round(value * 100) / 100;
}

/** Total hours across all activity buckets. */
export function totalHours(breakdown: ActivityBreakdown): number {
  return roundHours(
    ACTIVITY_KEYS.reduce((sum, key) => sum + (breakdown[key] || 0), 0),
  );
}

/** True when at least one activity bucket has positive hours. */
export function hasLoggedHours(breakdown: ActivityBreakdown): boolean {
  return ACTIVITY_KEYS.some((key) => (breakdown[key] || 0) > 0);
}

/** Field-level validation errors for the log dialog, keyed by field name. */
export interface TimeCardDraftErrors {
  date?: string;
  hours?: string;
  workLogComment?: string;
  approver?: string;
}

export interface TimeCardDraft {
  date: string;
  breakdown: ActivityBreakdown;
  workLogComment: string;
  approverId?: string;
}

/**
 * Validate a log-time draft. Returns an errors object; an empty object means the
 * draft is submittable. Mirrors the ServiceNow required fields (Date, Task —
 * preset from the case, Work Log Comment, Approver) plus a "log some time" rule.
 */
export function timeCardDraftErrors(draft: TimeCardDraft): TimeCardDraftErrors {
  const errors: TimeCardDraftErrors = {};
  if (!draft.date) errors.date = "Pick a date.";
  if (!hasLoggedHours(draft.breakdown)) {
    errors.hours = "Log time against at least one activity.";
  }
  if (!draft.workLogComment.trim()) {
    errors.workLogComment = "Add a work log comment.";
  }
  if (!draft.approverId) errors.approver = "Choose an approver.";
  return errors;
}
