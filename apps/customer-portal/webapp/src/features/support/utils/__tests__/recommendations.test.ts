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
  buildRecommendationRequestFromCase,
  buildRecommendationRequestFromConversationMessages,
  recommendationScoreToPercent,
} from "@features/support/utils/recommendations";
import type { CaseDetails } from "@features/support/types/cases";

describe("recommendationScoreToPercent", () => {
  it("should map 0–1 scores to a percentage", () => {
    expect(recommendationScoreToPercent(0)).toBe(0);
    expect(recommendationScoreToPercent(0.94)).toBe(94);
    expect(recommendationScoreToPercent(1)).toBe(100);
  });

  it("should treat values above 1 as already being percent-like", () => {
    expect(recommendationScoreToPercent(94)).toBe(94);
  });
});

describe("buildRecommendationRequestFromConversationMessages", () => {
  it("sends the answer, not Novera's <thinking> reasoning, but never edits a user message", () => {
    const base = {
      createdOn: "2026-01-01T00:00:00Z",
      isEscalated: false,
      hasInlineAttachments: false,
      inlineAttachments: [],
    };
    const req = buildRecommendationRequestFromConversationMessages([
      { ...base, id: "1", type: "user", content: "<thinking>mine</thinking>hello" },
      {
        ...base,
        id: "2",
        type: "bot",
        content: "<thinking>internal notes</thinking>\n\nWhich gateway is this?",
      },
    ]);
    expect(req?.chatHistory.map((m) => m.content)).toEqual([
      "<thinking>mine</thinking>hello",
      "Which gateway is this?",
    ]);
  });

  it("drops a bot message that was only reasoning", () => {
    const req = buildRecommendationRequestFromConversationMessages([
      {
        id: "1",
        type: "bot",
        content: "<thinking>only reasoning</thinking>",
        createdOn: "2026-01-01T00:00:00Z",
        isEscalated: false,
        hasInlineAttachments: false,
        inlineAttachments: [],
      },
    ]);
    expect(req).toBeNull();
  });

  it("returns null for empty messages", () => {
    expect(buildRecommendationRequestFromConversationMessages([])).toBeNull();
  });

  const makeMsg = (content: string) => ({
    id: "1",
    content,
    type: "user",
    createdOn: "2026-01-01T00:00:00Z",
    isEscalated: false,
    hasInlineAttachments: false,
    inlineAttachments: [],
  });

  it("truncates message content to last 150 chars", () => {
    const req = buildRecommendationRequestFromConversationMessages([
      makeMsg("x".repeat(200)),
    ]);
    expect(req?.chatHistory[0].content).toBe("x".repeat(150));
  });

  it("does not truncate messages under 150 chars", () => {
    const req = buildRecommendationRequestFromConversationMessages([
      makeMsg("short message"),
    ]);
    expect(req?.chatHistory[0].content).toBe("short message");
  });
});

describe("buildRecommendationRequestFromCase", () => {
  it("should return null when case data is undefined", () => {
    expect(buildRecommendationRequestFromCase(undefined, [])).toBeNull();
  });

  it("should build chat history from title and comments", () => {
    const data = {
      title: "Slow API",
      description: "Latency spikes",
      createdOn: "2026-01-01T00:00:00Z",
      deployment: { id: "d1", label: "Prod" },
      deployedProduct: { id: "p1", label: "APIM", version: "4.2.0" },
    } as CaseDetails;

    const req = buildRecommendationRequestFromCase(data, []);
    expect(req).not.toBeNull();
    expect(req?.chatHistory.length).toBeGreaterThanOrEqual(2);
    expect(req?.conversationData.envProducts).toEqual({
      Prod: ["APIM 4.2.0"],
    });
  });

  it("drops Novera's <thinking> reasoning from case comments but not a person's", () => {
    const data = {
      title: "Slow API",
      description: "",
      createdOn: "2026-01-01T00:00:00Z",
    } as CaseDetails;
    const comment = (id: string, createdBy: string, content: string) => ({
      id,
      createdBy,
      content,
      type: "comments",
      createdOn: `2026-01-02T00:00:0${id}Z`,
      isEscalated: false,
    });

    const req = buildRecommendationRequestFromCase(data, [
      comment("1", "support-engineer@wso2.com", "<thinking>x</thinking>keep me"),
      comment("2", "Novera", "<thinking>internal</thinking>\n\nWhich gateway?"),
    ]);

    const contents = req?.chatHistory.map((m) => m.content) ?? [];
    expect(contents).toContain("<thinking>x</thinking>keep me");
    expect(contents).toContain("Which gateway?");
    expect(contents.join("\n")).not.toContain("internal");
  });
});
