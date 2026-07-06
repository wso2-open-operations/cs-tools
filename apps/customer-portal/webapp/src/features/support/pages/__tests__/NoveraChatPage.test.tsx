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

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import NoveraChatPage from "@features/support/pages/NoveraChatPage";

const mockNavigate = vi.fn();
const mockUseLocation = vi.fn();
const mockUseParams = vi.fn();
const mockClassifyCase = vi.fn();

vi.mock("react-router", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => mockUseLocation(),
  useParams: () => mockUseParams(),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchAll: () => ({ data: [], isLoading: false }),
}));

vi.mock("@features/support/api/useGetConversationMessages", () => ({
  useGetConversationMessages: () => ({
    data: undefined,
    isLoading: false,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isFetchingNextPage: false,
  }),
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => ({ data: { email: "dev@wso2.com" } }),
}));

vi.mock("@features/support/api/usePostCaseClassifications", () => ({
  usePostCaseClassifications: () => ({ mutateAsync: mockClassifyCase }),
}));

vi.mock("@features/support/hooks/useAllDeploymentProducts", () => ({
  useAllDeploymentProducts: () => ({ productsByDeploymentId: {}, isLoading: false }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { account: { id: "account-1" }, type: { id: "type-1", label: "Enterprise" } } }),
}));

vi.mock("@features/support/api/useChatWebSocket", () => ({
  useChatWebSocket: () => ({
    connect: vi.fn(),
    sendUserMessage: vi.fn(),
  }),
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useQueryClient: () => ({
      invalidateQueries: vi.fn(),
      getQueriesData: vi.fn(() => []),
    }),
  };
});

vi.mock("@features/support/components/novera-ai-assistant/novera-chat-page/ChatHeader", () => ({
  default: ({
    onBack,
    onCreateCase,
  }: {
    onBack: () => void;
    onCreateCase: () => void;
  }) => (
    <>
      <button onClick={onBack}>Back Chat</button>
      <button onClick={onCreateCase}>Create Case</button>
    </>
  ),
}));

vi.mock("@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageList", () => ({
  default: () => <div>MessageList</div>,
}));

vi.mock("@features/support/components/novera-ai-assistant/novera-chat-page/ChatInput", () => ({
  default: () => <div>ChatInput</div>,
}));

vi.mock("@features/support/components/novera-ai-assistant/novera-chat-page/ChatSkeleton", () => ({
  default: () => <div>ChatSkeleton</div>,
}));

describe("NoveraChatPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockClassifyCase.mockResolvedValue({});
    mockUseParams.mockReturnValue({ projectId: "project-1", conversationId: undefined });
    mockUseLocation.mockReturnValue({
      state: {
        initialUserMessage: "Need help with 504",
        messages: [],
      },
    });
  });

  it("should render chat composition shell", () => {
    render(<NoveraChatPage />);
    expect(screen.getByText("MessageList")).toBeInTheDocument();
    expect(screen.getByText("ChatInput")).toBeInTheDocument();
  });

  it("should navigate back and create-case branches", async () => {
    render(<NoveraChatPage />);
    fireEvent.click(screen.getByText("Back Chat"));
    fireEvent.click(screen.getByText("Create Case"));

    expect(mockNavigate).toHaveBeenCalledWith(-1);
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        "/projects/project-1/support/chat/create-case",
        expect.objectContaining({
          state: expect.objectContaining({ messages: expect.any(Array) }),
        }),
      );
    });
  });
});

