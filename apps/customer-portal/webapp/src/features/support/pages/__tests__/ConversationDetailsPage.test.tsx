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

import { render, screen, fireEvent } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ConversationDetailsPage from "@features/support/pages/ConversationDetailsPage";

const mockNavigate = vi.fn();
const mockUseLocation = vi.fn();
const mockUseParams = vi.fn();
const mockUseGetConversationMessages = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => mockUseLocation(),
  useParams: () => mockUseParams(),
}));

vi.mock("@features/support/api/useGetConversationMessages", () => ({
  useGetConversationMessages: () => mockUseGetConversationMessages(),
}));

vi.mock("@features/support/api/useChatWebSocket", () => ({
  useChatWebSocket: () => ({
    connect: vi.fn(),
    sendUserMessage: vi.fn(),
  }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { account: { id: "account-1" } } }),
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => ({ data: { email: "dev@wso2.com" } }),
}));

vi.mock(
  "@features/support/components/knowledge-base/ConversationKnowledgeRecommendations",
  () => ({
    default: () => <div>Knowledge</div>,
  }),
);

describe("ConversationDetailsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({
      projectId: "project-1",
      conversationId: "conv-1",
    });
    mockUseLocation.mockReturnValue({
      state: {
        returnTo: "/projects/project-1/support/conversations",
        conversationSummary: {
          chatId: "conv-1",
          status: "Open",
          startedTime: "2026-05-01T00:00:00Z",
          messages: 2,
        },
      },
    });
    mockUseGetConversationMessages.mockReturnValue({
      data: { pages: [{ comments: [] }] },
      isLoading: false,
      isError: false,
      error: undefined,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    });
  });

  it("should render loading branch for conversation messages", () => {
    mockUseGetConversationMessages.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: undefined,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isFetchingNextPage: false,
    });

    render(<ConversationDetailsPage />);

    expect(screen.getByText("Chat Session")).toBeInTheDocument();
    expect(screen.getByText("Conversation")).toBeInTheDocument();
  });

  it("should navigate to returnTo when back clicked", () => {
    render(<ConversationDetailsPage />);

    fireEvent.click(screen.getByText("Back"));

    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/conversations",
      { state: { fromBack: true } },
    );
  });
});

