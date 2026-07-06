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
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";
import { compareFeedEntries, type FeedEntry } from "./caseActivityFeed";

function commentEntry(
  id: string,
  at: string,
  role: CsmCaseComment["authorRole"],
): FeedEntry {
  return {
    kind: "comment",
    at,
    comment: {
      id,
      caseId: "c",
      authorName: role === "chatbot" ? "Novera" : "Someone",
      authorRole: role,
      bodyHtml: "",
      createdAt: at,
    },
  };
}

describe("compareFeedEntries", () => {
  it("orders by timestamp ascending", () => {
    const older = commentEntry("a", "2026-07-01T00:00:00Z", "customer");
    const newer = commentEntry("b", "2026-07-01T00:05:00Z", "chatbot");
    expect(compareFeedEntries(older, newer)).toBeLessThan(0);
    expect(compareFeedEntries(newer, older)).toBeGreaterThan(0);
  });

  it("puts the human question before the bot answer on a timestamp tie", () => {
    // Both imported at the same second — the real Novera Q&A case.
    const ts = "2026-07-01T00:51:54Z";
    const question = commentEntry("q", ts, "customer");
    const answer = commentEntry("a", ts, "chatbot");
    // answer sorts AFTER question regardless of input order
    expect(compareFeedEntries(question, answer)).toBeLessThan(0);
    expect(compareFeedEntries(answer, question)).toBeGreaterThan(0);
  });

  it("newest-first (negated) keeps the bot answer above the question", () => {
    const ts = "2026-07-01T00:51:54Z";
    const question = commentEntry("q", ts, "customer");
    const answer = commentEntry("a", ts, "chatbot");
    const feed = [question, answer].sort((x, y) => -compareFeedEntries(x, y));
    // Newest-first view: the later-in-thread bot answer shows above the question.
    expect(feed.map((e) => (e.kind === "comment" ? e.comment.id : ""))).toEqual([
      "a",
      "q",
    ]);
  });

  it("is deterministic for two non-bot entries at the same time (by id)", () => {
    const ts = "2026-07-01T00:51:54Z";
    const a = commentEntry("a", ts, "customer");
    const b = commentEntry("b", ts, "customer");
    expect(compareFeedEntries(a, b)).toBeLessThan(0);
    expect(compareFeedEntries(b, a)).toBeGreaterThan(0);
  });
});
