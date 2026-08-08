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
  canResumeToUnlockPublicReply,
  caseAcceptsPublicComments,
  effectiveWorkState,
  publicCommentGateReason,
} from "./caseWorkState";

describe("caseAcceptsPublicComments", () => {
  it("allows public comments only when work_in_progress AND ongoing", () => {
    expect(caseAcceptsPublicComments("work_in_progress", "ongoing")).toBe(true);
  });

  it("blocks when work_in_progress but paused", () => {
    expect(caseAcceptsPublicComments("work_in_progress", "paused")).toBe(false);
  });

  it("blocks when work_in_progress but workState is absent", () => {
    expect(caseAcceptsPublicComments("work_in_progress", null)).toBe(false);
    expect(caseAcceptsPublicComments("work_in_progress", undefined)).toBe(
      false,
    );
  });

  it("blocks in any other state regardless of workState", () => {
    expect(caseAcceptsPublicComments("open", "ongoing")).toBe(false);
    expect(caseAcceptsPublicComments("awaiting_info", "ongoing")).toBe(false);
    expect(caseAcceptsPublicComments("waiting_on_wso2", null)).toBe(false);
    expect(caseAcceptsPublicComments("solution_proposed", null)).toBe(false);
    expect(caseAcceptsPublicComments("closed", null)).toBe(false);
    expect(caseAcceptsPublicComments(undefined, undefined)).toBe(false);
  });
});

describe("publicCommentGateReason", () => {
  it("returns null when public comments are allowed and the caller is the assignee", () => {
    expect(
      publicCommentGateReason("work_in_progress", "ongoing", true),
    ).toBeNull();
  });

  it("gives a resume hint for the assignee's paused case", () => {
    expect(
      publicCommentGateReason("work_in_progress", "paused", true),
    ).toMatch(/paused/i);
  });

  it("gives an in-progress hint for other states when the caller is the assignee", () => {
    expect(publicCommentGateReason("open", null, true)).toMatch(
      /in progress/i,
    );
    expect(publicCommentGateReason("closed", null, true)).toMatch(
      /in progress/i,
    );
  });

  it("does not promise a work-note fallback (pending the backend exemption)", () => {
    expect(publicCommentGateReason("open", null, true)).not.toMatch(
      /work note/i,
    );
    expect(
      publicCommentGateReason("work_in_progress", "paused", true),
    ).not.toMatch(/work note/i);
  });

  it("blocks a non-assignee with the ownership reason, even on an otherwise-open case", () => {
    expect(
      publicCommentGateReason("work_in_progress", "ongoing", false),
    ).toMatch(/assigned engineer/i);
  });

  it("blocks a non-assignee with the ownership reason rather than the state/paused reason", () => {
    const reason = publicCommentGateReason(
      "work_in_progress",
      "paused",
      false,
    );
    expect(reason).toMatch(/assigned engineer/i);
    expect(reason).not.toMatch(/paused/i);
  });

  it("blocks a non-assignee with the ownership reason regardless of case state", () => {
    expect(publicCommentGateReason("open", null, false)).toMatch(
      /assigned engineer/i,
    );
    expect(publicCommentGateReason("closed", null, false)).toMatch(
      /assigned engineer/i,
    );
  });
});

describe("canResumeToUnlockPublicReply", () => {
  it("is true for the engineer's own paused, work_in_progress case", () => {
    expect(
      canResumeToUnlockPublicReply("work_in_progress", "paused", true),
    ).toBe(true);
  });

  it("is false when the case isn't assigned to the signed-in engineer, even if paused", () => {
    expect(
      canResumeToUnlockPublicReply("work_in_progress", "paused", false),
    ).toBe(false);
  });

  it("is false once the case is already ongoing (nothing to resume)", () => {
    expect(
      canResumeToUnlockPublicReply("work_in_progress", "ongoing", true),
    ).toBe(false);
  });

  it("is false when the case hasn't started yet, regardless of assignee", () => {
    expect(canResumeToUnlockPublicReply("open", null, true)).toBe(false);
    expect(canResumeToUnlockPublicReply("open", null, false)).toBe(false);
  });

  // A null/undefined workState on a work_in_progress case is a real, expected
  // state (e.g. data predating the work-state feature), not just "open"'s
  // absent-workState case above — and it's resumable, matching
  // CaseActionBar's own deliberate handling of the same case (see its
  // "anything else (paused OR a null work-state in-progress case) is
  // resumable" comment in buildSecondaryItems). Requiring workState ===
  // "paused" exactly here would hide this quick-fix for a case where the
  // action bar's own "Resume work" item is still shown and functional.
  it("is true for a work_in_progress case with a null or undefined work state", () => {
    expect(canResumeToUnlockPublicReply("work_in_progress", null, true)).toBe(
      true,
    );
    expect(
      canResumeToUnlockPublicReply("work_in_progress", undefined, true),
    ).toBe(true);
  });
});

describe("effectiveWorkState", () => {
  it("passes through an explicit work state as-is", () => {
    expect(effectiveWorkState("ongoing")).toBe("ongoing");
    expect(effectiveWorkState("paused")).toBe("paused");
  });

  it("treats a never-set work state as paused", () => {
    expect(effectiveWorkState(null)).toBe("paused");
    expect(effectiveWorkState(undefined)).toBe("paused");
  });
});
