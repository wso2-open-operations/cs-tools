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

import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

/**
 * One-line summary of a card's approval decision, or `null` when it hasn't been
 * decided yet.
 *
 * The two decisions are deliberately asymmetric, because the upstream data is:
 * ServiceNow records **who approved** a card (`approvedBy`) but records nothing
 * about who rejected one — no rejecter identity, no timestamp. All a rejection
 * leaves behind is the approver's comment (`rejectionReason`). So an approval
 * shows the name, and a rejection shows the reason instead of a name. (Showing
 * "Decided by …" for both was the old behaviour, and it silently rendered
 * nothing on rejected cards, since `approvedBy` is null for them.)
 *
 * A rejected card with no reason still reports "Rejected" rather than falling
 * through to `null` — the state is a decision even when the comment is missing.
 */
export function decisionSummary(card: CsmTimeCard): string | null {
  if (card.state === "approved") {
    return card.approvedByName ? `Approved by ${card.approvedByName}` : "Approved";
  }
  if (card.state === "rejected") {
    return card.rejectionReason ? `Rejected: ${card.rejectionReason}` : "Rejected";
  }
  return null;
}
