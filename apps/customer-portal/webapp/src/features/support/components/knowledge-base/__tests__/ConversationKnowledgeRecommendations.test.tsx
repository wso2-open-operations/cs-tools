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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ConversationKnowledgeRecommendations from "@features/support/components/knowledge-base/ConversationKnowledgeRecommendations";
import type { ConversationMessage } from "@features/support/types/conversations";

vi.mock("@features/support/api/useConversationRecommendationsSearch", () => ({
  useConversationRecommendationsSearch: vi.fn(() => ({
    data: {
      query: "Example query",
      recommendations: [
        { title: "Article One", articleId: "KB-1", score: 0.91 },
      ],
    },
    isLoading: false,
    isError: false,
  })),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

const baseMessage = (
  overrides: Partial<ConversationMessage>,
): ConversationMessage => ({
  id: "m1",
  content: "Hello",
  type: "user",
  isEscalated: false,
  hasInlineAttachments: false,
  inlineAttachments: [],
  createdOn: "2026-01-01T00:00:00Z",
  ...overrides,
});

describe("ConversationKnowledgeRecommendations", () => {
  it("should render recommendation titles when API returns data", () => {
    render(
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={createTheme()}>
          <ConversationKnowledgeRecommendations
            messages={[
              baseMessage({ id: "a", content: "User question" }),
              baseMessage({
                id: "b",
                content: "Bot reply",
                type: "bot",
                createdBy: "novera",
              }),
            ]}
          />
        </ThemeProvider>
      </QueryClientProvider>,
    );

    expect(
      screen.getByText("Knowledge Base Articles Suggested"),
    ).toBeInTheDocument();
    expect(screen.getByText("Article One")).toBeInTheDocument();
    expect(screen.getByText("Example query")).toBeInTheDocument();
  });
});
