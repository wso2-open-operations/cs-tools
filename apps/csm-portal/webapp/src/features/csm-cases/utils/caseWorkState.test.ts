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
  caseAcceptsPublicComments,
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
  it("returns null when public comments are allowed", () => {
    expect(publicCommentGateReason("work_in_progress", "ongoing")).toBeNull();
  });

  it("gives a resume hint for a paused case", () => {
    expect(publicCommentGateReason("work_in_progress", "paused")).toMatch(
      /paused/i,
    );
  });

  it("gives an in-progress hint for other states", () => {
    expect(publicCommentGateReason("open", null)).toMatch(/in progress/i);
    expect(publicCommentGateReason("closed", null)).toMatch(/in progress/i);
  });

  it("does not promise a work-note fallback (pending the backend exemption)", () => {
    expect(publicCommentGateReason("open", null)).not.toMatch(/work note/i);
    expect(publicCommentGateReason("work_in_progress", "paused")).not.toMatch(
      /work note/i,
    );
  });
});
