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

import { describe, expect, it } from "vitest";
import {
  cabCommentsLabel,
  cabDecisionText,
  cabHistoryHeadline,
  cabHistoryOf,
  cabHistorySummary,
  cabRejectionsLabel,
  deviationLabel,
  hasCabPushback,
  suggestionDeviationCount,
} from "@features/csm-operations/utils/cabHistory";
import type { BeStandardChangeCabHistory } from "@api/backend/types";

const cr = { id: "cr-1", number: "CHG0000001", subject: "Deploy", state: "canceled" };

describe("cabHistory", () => {
  it("labels the counts, and none as nothing", () => {
    expect(cabRejectionsLabel(1)).toBe("1 CAB rejection");
    expect(cabRejectionsLabel(2)).toBe("2 CAB rejections");
    expect(cabCommentsLabel(3)).toBe("3 CAB comments");
    expect(deviationLabel(1)).toBe("1 deviated from plan");
    expect(cabRejectionsLabel(0)).toBeNull();
    expect(cabCommentsLabel(undefined)).toBeNull();
    expect(deviationLabel(0)).toBeNull();
  });

  it("reads an absent or null history as none, and never counts fewer than it lists", () => {
    expect(cabHistoryOf(undefined)).toEqual({ rejections: [], rejectionCount: 0, comments: [], commentCount: 0 });
    expect(cabHistoryOf({ cabRejections: null, cabComments: null })).toEqual({ rejections: [], rejectionCount: 0, comments: [], commentCount: 0 });
    const listed = cabHistoryOf({ cabRejections: [{ changeRequest: cr, member: false, rejectedOn: "", rejectedBy: "", comment: "" }] });
    expect(listed.rejectionCount).toBe(1);
    expect(hasCabPushback(undefined)).toBe(false);
    expect(hasCabPushback({ cabCommentCount: 1 })).toBe(true);
  });

  it("counts deviations from the count sent, else from the listed members", () => {
    const members = [{ hasDeviation: true }, { hasDeviation: false }] as Parameters<typeof suggestionDeviationCount>[0]["members"];
    expect(suggestionDeviationCount({ deviationCount: 4, members })).toBe(4);
    expect(suggestionDeviationCount({ members })).toBe(1);
  });

  it("headlines and sums up what the CAB said", () => {
    const h: BeStandardChangeCabHistory = {
      cabRejectionCount: 2,
      cabRejections: [
        { changeRequest: cr, member: false, rejectedOn: "2026-10-01T00:00:00Z", rejectedBy: "Alice", comment: "" },
        { changeRequest: cr, member: true, rejectedOn: "2026-09-01T00:00:00Z", rejectedBy: "Bob", comment: "No rollback plan" },
      ],
      cabCommentCount: 1,
      cabComments: [{ changeRequest: cr, member: true, approverName: "Bob", state: "REJECTED", comment: "No rollback plan", decidedOn: "2026-09-01T00:00:00Z" }],
    };
    expect(cabHistoryHeadline(h)).toBe("The CAB rejected 2 change requests like this one");
    expect(cabHistorySummary(h)).toBe('The CAB rejected 2 change requests like this one and left 1 comment. Latest: "No rollback plan"');
    const commentsOnly = { ...h, cabRejectionCount: 0, cabRejections: [] };
    expect(cabHistoryHeadline(commentsOnly)).toBe("The CAB left 1 comment on change requests like this one");
    expect(cabHistorySummary(commentsOnly)).toBe('The CAB left 1 comment on change requests like this one. Latest: "No rollback plan"');
    expect(cabHistoryHeadline(null)).toBe("No CAB rejections or comments");
    expect(cabHistorySummary(undefined)).toBe("No CAB rejections or comments.");
  });

  it("quotes the newest comment as the latest, even when it is newer than the rejection", () => {
    const h: BeStandardChangeCabHistory = {
      cabRejectionCount: 1,
      cabRejections: [{ changeRequest: cr, member: true, rejectedOn: "2026-01-01T00:00:00Z", rejectedBy: "Bob", comment: "Plan unclear" }],
      cabCommentCount: 2,
      cabComments: [
        { changeRequest: cr, member: true, approverName: "Alice", state: "APPROVED", comment: "Add the verification steps", decidedOn: "2026-03-01T00:00:00Z" },
        { changeRequest: cr, member: true, approverName: "Bob", state: "REJECTED", comment: "Plan unclear", decidedOn: "2026-01-01T00:00:00Z" },
      ],
    };
    expect(cabHistorySummary(h)).toBe(
      'The CAB rejected 1 change request like this one and left 2 comments. Latest: "Add the verification steps"',
    );
    // With no comment listed, the rejection's own comment is the only word.
    expect(cabHistorySummary({ ...h, cabCommentCount: 0, cabComments: [] })).toBe(
      'The CAB rejected 1 change request like this one. Latest: "Plan unclear"',
    );
  });

  it("says a decision in words", () => {
    expect(cabDecisionText("APPROVED")).toBe("approved");
    expect(cabDecisionText("REJECTED")).toBe("rejected");
    expect(cabDecisionText("REQUESTED")).toBe("asked");
    expect(cabDecisionText("NO_CONSENSUS")).toBe("no consensus");
  });
});
