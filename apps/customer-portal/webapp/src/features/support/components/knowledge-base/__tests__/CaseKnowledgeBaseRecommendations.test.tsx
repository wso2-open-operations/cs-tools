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

import { fireEvent, render, screen } from "@testing-library/react";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it, vi } from "vitest";
import CaseKnowledgeBaseRecommendations from "../CaseKnowledgeBaseRecommendations";

vi.mock("@features/support/api/useGetAIChatHistory", () => ({
  default: () => ({ comments: [], isLoading: false }),
}));
vi.mock("@features/support/api/useConversationRecommendationsSearch", () => ({
  useConversationRecommendationsSearch: () => ({
    data: { recommendations: [{ articleId: "a-1", title: "Doc", score: 0.8 }] },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("@features/support/utils/recommendations", () => ({
  buildRecommendationRequestFromCase: () => ({ prompt: "x" }),
  recommendationScoreToPercent: () => 80,
}));

describe("CaseKnowledgeBaseRecommendations", () => {
  it("renders recommendation and opens article", () => {
    const openSpy = vi.spyOn(window, "open").mockImplementation(() => null);
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseKnowledgeBaseRecommendations caseId="c1" projectId="p1" data={{} as never} />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: /view article/i }));
    expect(openSpy).toHaveBeenCalledWith(
      expect.stringContaining("a-1"),
      "_blank",
      expect.stringContaining("noopener"),
    );
  });
});
