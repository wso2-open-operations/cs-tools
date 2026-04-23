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
});
