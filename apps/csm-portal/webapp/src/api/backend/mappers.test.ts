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
import type { BeComment } from "./types";
import {
  beStateFromUi,
  commentTypeFromInternal,
  priorityFromSeverity,
  severityFromBe,
  uiCommentFromBe,
  uiStateFromBe,
} from "./mappers";

describe("severityFromBe", () => {
  it("maps legacy English names onto the S0-S4 scale", () => {
    expect(severityFromBe("catastrophic")).toBe("S0");
    expect(severityFromBe("critical")).toBe("S1");
    expect(severityFromBe("high")).toBe("S2");
    expect(severityFromBe("medium")).toBe("S3");
    expect(severityFromBe("low")).toBe("S4");
  });

  it("maps the backend display-string format 'Label (Px)' onto S0-S4", () => {
    expect(severityFromBe("Catastrophic (P0)")).toBe("S0");
    expect(severityFromBe("Critical (P1)")).toBe("S1");
    expect(severityFromBe("High (P2)")).toBe("S2");
    expect(severityFromBe("Medium (P3)")).toBe("S3");
    expect(severityFromBe("Low (P4)")).toBe("S4");
  });

  it("maps a falsy/unrecognized severity to the distinct 'unset' state, never to S3", () => {
    // A case with no severity value is NOT the same fact as "the severity
    // really is Medium" — it must never collapse into a real severity.
    expect(severityFromBe(undefined)).toBe("unset");
    expect(severityFromBe("")).toBe("unset");
    expect(severityFromBe("unknown_value")).toBe("unset");
  });
});

describe("priorityFromSeverity", () => {
  it("is the inverse of severityFromBe for the known set", () => {
    expect(priorityFromSeverity("S0")).toBe("catastrophic");
    expect(priorityFromSeverity("S1")).toBe("critical");
    expect(priorityFromSeverity("S2")).toBe("high");
    expect(priorityFromSeverity("S3")).toBe("medium");
    expect(priorityFromSeverity("S4")).toBe("low");
  });
});

describe("uiStateFromBe / beStateFromUi", () => {
  it("passes through shared states unchanged", () => {
    for (const s of [
      "open",
      "work_in_progress",
      "waiting_on_wso2",
      "awaiting_info",
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

  it("normalizes the raw ServiceNow label form to the enum", () => {
    // The SN case-search view sends the human label instead of the enum; the
    // mapper lowercases + collapses whitespace so SN cases render with the
    // curated label/colour and `state === "work_in_progress"` checks match.
    expect(uiStateFromBe("Work In Progress")).toBe("work_in_progress");
    expect(uiStateFromBe("Waiting On WSO2")).toBe("waiting_on_wso2");
    expect(uiStateFromBe("Solution Proposed")).toBe("solution_proposed");
  });
});

describe("commentTypeFromInternal", () => {
  it("maps the internal flag to the backend comment type", () => {
    expect(commentTypeFromInternal(true)).toBe("work_note");
    expect(commentTypeFromInternal(false)).toBe("comment");
  });
});

describe("uiCommentFromBe", () => {
  const base: BeComment = {
    id: "c1",
    referenceId: "case1",
    type: "comment",
    content: "<p>hello</p>",
    createdBy: {
      id: null,
      email: "user@wso2.com",
      name: "Ada Lovelace ⓦ",
    },
    createdOn: "2026-06-01T10:00:00Z",
  };

  it("wraps a public comment as a wso2_engineer, non-internal bubble", () => {
    const ui = uiCommentFromBe(base);
    expect(ui.authorRole).toBe("wso2_engineer");
    expect(ui.internal).toBe(false);
    expect(ui.authorName).toBe("Ada Lovelace ⓦ");
    expect(ui.bodyHtml).toBe("<p>hello</p>");
    expect(ui.createdAt).toBe("2026-06-01T10:00:00Z");
  });

  it("marks a work_note as internal", () => {
    const ui = uiCommentFromBe({ ...base, type: "work_note" });
    expect(ui.authorRole).toBe("wso2_engineer");
    expect(ui.internal).toBe(true);
  });

  it("renders an activity entry as a system author", () => {
    const ui = uiCommentFromBe({ ...base, type: "activity" });
    expect(ui.authorRole).toBe("system");
    expect(ui.internal).toBe(false);
  });

  it("passes through the HTML content untouched (sanitised at render)", () => {
    const ui = uiCommentFromBe({
      ...base,
      content: '<p>a &amp; b</p><img src=x onerror="alert(1)">',
    });
    expect(ui.bodyHtml).toBe('<p>a &amp; b</p><img src=x onerror="alert(1)">');
  });

  it("falls back from name to email, then Unknown when createdBy is null", () => {
    expect(
      uiCommentFromBe({
        ...base,
        createdBy: { id: null, email: "x@wso2.com", name: "" },
      }).authorName,
    ).toBe("x@wso2.com");
    expect(
      uiCommentFromBe({ ...base, createdBy: null }).authorName,
    ).toBe("Unknown");
  });
});

describe("uiCommentFromBe — /comments/search shape and chat", () => {
  // The confirmed shape backing both case comments and chat messages: a
  // canonical `createdBy` UserReference, `referenceId` (not `caseId`), and a
  // normalized singular `type`. (createdOn tie-break etc. is covered in
  // caseActivityFeed.test.ts.)
  const msg: BeComment = {
    id: "m1",
    referenceId: "conv1",
    content: "the EOL for AWS RDS MySQL 8.0.42 is July 31st, 2026",
    type: "comment",
    createdOn: "2026-07-01T00:51:54Z",
    createdBy: {
      id: null,
      email: "sree@abc.com",
      name: "Sree Kumar",
    },
  };

  it("maps a customer chat message to a customer bubble", () => {
    const ui = uiCommentFromBe(msg, { context: "conversation" });
    expect(ui.authorRole).toBe("customer");
    expect(ui.internal).toBe(false);
    expect(ui.authorName).toBe("Sree Kumar");
    expect(ui.caseId).toBe("conv1"); // referenceId, not caseId
  });

  it("detects Novera as a chatbot via the nested createdBy.name", () => {
    const ui = uiCommentFromBe(
      { ...msg, createdBy: { id: null, email: "novera@bot", name: "Novera" } },
      { context: "conversation" },
    );
    expect(ui.authorRole).toBe("chatbot");
  });

  it("drops Novera's <thinking> reasoning but never edits a person's message", () => {
    const withThinking = "<thinking>internal notes</thinking>\n\nWhich environment?";
    const bot = uiCommentFromBe(
      {
        ...msg,
        content: withThinking,
        createdBy: { id: null, email: "novera@bot", name: "Novera" },
      },
      { context: "conversation" },
    );
    expect(bot.bodyHtml).toBe("Which environment?");

    const person = uiCommentFromBe(
      { ...msg, content: withThinking },
      { context: "conversation" },
    );
    expect(person.bodyHtml).toBe(withThinking);
  });

  it("marks a work_note as internal", () => {
    const ui = uiCommentFromBe({ ...msg, type: "work_note" });
    expect(ui.internal).toBe(true);
  });

  it("defaults a non-bot case comment to wso2_engineer", () => {
    const ui = uiCommentFromBe(msg, { context: "case" });
    expect(ui.authorRole).toBe("wso2_engineer");
  });

  it("falls back to Unknown when createdBy is null", () => {
    const ui = uiCommentFromBe(
      { ...msg, createdBy: null },
      { context: "case" },
    );
    expect(ui.authorName).toBe("Unknown");
  });
});
