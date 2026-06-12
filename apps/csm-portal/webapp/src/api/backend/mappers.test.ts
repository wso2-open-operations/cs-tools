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
import type { BeCaseComment } from "./types";
import {
  beStateFromUi,
  commentTypeFromInternal,
  priorityFromSeverity,
  severityFromPriority,
  uiCommentFromBe,
  uiStateFromBe,
} from "./mappers";

describe("severityFromPriority", () => {
  it("maps each backend priority onto the S0-S4 scale", () => {
    expect(severityFromPriority("catastrophic")).toBe("S0");
    expect(severityFromPriority("critical")).toBe("S1");
    expect(severityFromPriority("high")).toBe("S2");
    expect(severityFromPriority("medium")).toBe("S3");
    expect(severityFromPriority("low")).toBe("S4");
  });

  it("falls back to S3 for an unknown/undefined priority", () => {
    expect(severityFromPriority(undefined)).toBe("S3");
  });
});

describe("priorityFromSeverity", () => {
  it("is the inverse of severityFromPriority for the known set", () => {
    expect(priorityFromSeverity("S0")).toBe("catastrophic");
    expect(priorityFromSeverity("S1")).toBe("critical");
    expect(priorityFromSeverity("S2")).toBe("high");
    expect(priorityFromSeverity("S3")).toBe("medium");
    expect(priorityFromSeverity("S4")).toBe("low");
  });
});

describe("uiStateFromBe / beStateFromUi", () => {
  it("passes reopened through unchanged (UI and backend now share the spelling)", () => {
    expect(uiStateFromBe("reopened")).toBe("reopened");
    expect(beStateFromUi("reopened")).toBe("reopened");
  });

  it("passes through shared states unchanged", () => {
    for (const s of [
      "open",
      "work_in_progress",
      "waiting_on_wso2",
      "awaiting_info",
      "reopened",
      "solution_proposed",
      "closed",
    ] as const) {
      expect(uiStateFromBe(s)).toBe(s);
      expect(beStateFromUi(s)).toBe(s);
    }
  });

  it("defaults an absent backend state to open", () => {
    expect(uiStateFromBe(undefined)).toBe("open");
  });

  it("passes an unknown backend state through so the UI can render it", () => {
    // A state the frontend has not been taught about must still reach the UI
    // (it renders with a humanized label) rather than being collapsed to a
    // known state — that is what lets the backend add a state with no FE change.
    expect(uiStateFromBe("pending_review")).toBe("pending_review");
  });
});

describe("commentTypeFromInternal", () => {
  it("maps the internal flag to the backend comment type", () => {
    expect(commentTypeFromInternal(true)).toBe("work_note");
    expect(commentTypeFromInternal(false)).toBe("comment");
  });
});

describe("uiCommentFromBe", () => {
  const base: BeCaseComment = {
    id: "c1",
    caseId: "case1",
    commentType: "comment",
    body: "hello",
    createdBy: "user-123",
    createdAt: "2026-06-01T10:00:00Z",
  };

  it("wraps a public comment as a wso2_engineer, non-internal bubble", () => {
    const ui = uiCommentFromBe(base);
    expect(ui.authorRole).toBe("wso2_engineer");
    expect(ui.internal).toBe(false);
    expect(ui.authorName).toBe("user-123");
    expect(ui.bodyHtml).toBe("<p>hello</p>");
  });

  it("marks a work_note as internal", () => {
    const ui = uiCommentFromBe({ ...base, commentType: "work_note" });
    expect(ui.authorRole).toBe("wso2_engineer");
    expect(ui.internal).toBe(true);
  });

  it("renders an activity entry as a system author", () => {
    const ui = uiCommentFromBe({ ...base, commentType: "activity" });
    expect(ui.authorRole).toBe("system");
    expect(ui.internal).toBe(false);
  });

  it("escapes HTML-special characters and converts newlines (htmlFromPlain)", () => {
    const ui = uiCommentFromBe({
      ...base,
      body: 'a & b < c > d\nsecond <script>',
    });
    expect(ui.bodyHtml).toBe(
      "<p>a &amp; b &lt; c &gt; d<br/>second &lt;script&gt;</p>",
    );
    // No raw angle brackets survive the escape.
    expect(ui.bodyHtml).not.toContain("<script>");
  });

  it("normalises CRLF/CR line endings to a single break (no \\r artifacts)", () => {
    const ui = uiCommentFromBe({
      ...base,
      body: "line1\r\nline2\rline3\nline4",
    });
    expect(ui.bodyHtml).toBe("<p>line1<br/>line2<br/>line3<br/>line4</p>");
    expect(ui.bodyHtml).not.toContain("\r");
  });
});
