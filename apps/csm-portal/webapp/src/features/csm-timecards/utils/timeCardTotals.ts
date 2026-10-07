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
import {
  MAX_MINUTES_PER_TICKET_PER_DAY,
  WORK_LOG_MAX,
} from "@features/csm-timecards/constants/timeCardConstants";
import { isBlankHtml } from "@utils/sanitizeHtml";

/** A fresh breakdown with every activity at zero minutes. */
export function emptyBreakdown(): ActivityBreakdown {
  return {
    analysisDebugging: 0,
    reproduce: 0,
    settingUp: 0,
    providingSolution: 0,
    answering: 0,
  };
}

/** Total whole minutes across all activity buckets. */
export function totalMinutes(breakdown: ActivityBreakdown): number {
  return ACTIVITY_KEYS.reduce((sum, key) => sum + (breakdown[key] || 0), 0);
}

/** True when at least one activity bucket has logged time. */
export function hasLoggedTime(breakdown: ActivityBreakdown): boolean {
  return ACTIVITY_KEYS.some((key) => (breakdown[key] || 0) > 0);
}

/** Field-level validation errors for the log dialog, keyed by field name. */
export interface TimeCardDraftErrors {
  date?: string;
  minutes?: string;
  workLogComment?: string;
  approver?: string;
}

export interface TimeCardDraft {
  date: string;
  breakdown: ActivityBreakdown;
  workLogComment: string;
  approverId?: string;
  /**
   * Whether an approver must be chosen. True (the default) when creating a
   * card. False when editing an existing one: the approver is fixed at
   * submission time, shown read-only, and never sent on an edit — and a card
   * may come back with no approver at all, which must not block the edit.
   */
  requireApprover?: boolean;
}

/**
 * Validate a log-time draft. Returns an errors object; an empty object means the
 * draft is submittable. Mirrors the required fields of the backing work-log
 * record (Date, Task — preset from the case, Work Log Comment, and Approver on
 * create) plus a "log some time" rule.
 */
export function timeCardDraftErrors(draft: TimeCardDraft): TimeCardDraftErrors {
  const errors: TimeCardDraftErrors = {};
  if (!draft.date) errors.date = "Pick a date.";
  if (!hasLoggedTime(draft.breakdown)) {
    errors.minutes = "Log time against at least one activity.";
  } else if (totalMinutes(draft.breakdown) > MAX_MINUTES_PER_TICKET_PER_DAY) {
    errors.minutes = `Total logged time cannot exceed ${MAX_MINUTES_PER_TICKET_PER_DAY / 60} hours (${MAX_MINUTES_PER_TICKET_PER_DAY} minutes) per ticket per day.`;
  }
  // workLogComment is rich-text HTML from Editor, not a plain string — an
  // untouched editor still outputs non-empty-looking HTML (e.g.
  // "<p><br></p>"), so `.trim()` truthiness would treat an empty comment as
  // filled in. isBlankHtml is the same check used wherever else the app
  // decides whether HTML content is really empty.
  if (isBlankHtml(draft.workLogComment)) {
    errors.workLogComment = "Add a work log comment.";
  } else if (draft.workLogComment.length > WORK_LOG_MAX) {
    errors.workLogComment = `Comment must be ${WORK_LOG_MAX} characters or fewer.`;
  }
  if (draft.requireApprover !== false && !draft.approverId) {
    errors.approver = "Choose an approver.";
  }
  return errors;
}
