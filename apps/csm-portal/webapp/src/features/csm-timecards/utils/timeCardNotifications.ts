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
// Lifecycle notifications for the time reporter (ISSU-009: "Automated
// notifications must be sent to the time reporter regarding lifecycle status
// changes, e.g. when a time log is rejected"). FE-first: there is no push
// channel yet, so we derive in-app notices from the engineer's own cards and
// surface them as a banner. The backend phase replaces this with real
// notifications, keyed off the same lifecycle events.
//

import type { CsmTimeCard, CsmTimeSheet } from "@features/csm-timecards/types/timeCards";

/** Severity drives the banner colour; "error" for rejections, "warning" for recalls. */
export type TimeCardNoticeSeverity = "error" | "warning" | "success";

export interface TimeCardNotice {
  cardId: string;
  severity: TimeCardNoticeSeverity;
  caseNumber: string;
  /** Human-readable line, e.g. "CS0352584 was rejected — Please fix the hours." */
  message: string;
  /** When the deciding event happened (ISO), newest first when sorted. */
  at: string;
}

/**
 * Build the reporter's lifecycle notices from their weekly sheets: a card the
 * reporter must act on (rejected or recalled) yields a notice. Newest first.
 */
export function timeCardNotices(sheets: CsmTimeSheet[]): TimeCardNotice[] {
  const cards: CsmTimeCard[] = sheets.flatMap((s) => s.cards);
  const notices: TimeCardNotice[] = [];
  for (const c of cards) {
    if (c.state === "rejected") {
      notices.push({
        cardId: c.id,
        severity: "error",
        caseNumber: c.caseNumber,
        message: `${c.caseNumber} was rejected${
          c.leadComment ? ` — ${c.leadComment}` : ""
        }. Edit and resubmit.`,
        at: c.decidedAt ?? c.submittedAt,
      });
    } else if (c.state === "recalled") {
      notices.push({
        cardId: c.id,
        severity: "warning",
        caseNumber: c.caseNumber,
        message: `${c.caseNumber} was recalled for adjustment${
          c.decidedBy ? ` by ${c.decidedBy}` : ""
        }. Edit and resubmit.`,
        at: c.decidedAt ?? c.submittedAt,
      });
    }
  }
  return notices.sort((a, b) => b.at.localeCompare(a.at));
}
