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
import AllConversationsPage from "@features/support/pages/AllConversationsPage";

const mockNavigate = vi.fn();
const mockShowLoader = vi.fn();
const mockHideLoader = vi.fn();

const mockUseParams = vi.fn();
const mockUseLocation = vi.fn();
const mockUseSearchParams = vi.fn();
const mockUseSessionState = vi.fn();
const mockUseGetProjectFilters = vi.fn();
const mockUseSearchConversations = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useParams: () => mockUseParams(),
    useLocation: () => mockUseLocation(),
    useSearchParams: () => mockUseSearchParams(),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => mockNavigate,
}));

vi.mock("@hooks/useSessionState", () => ({
  useSessionState: (...args: unknown[]) => mockUseSessionState(...args),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: mockShowLoader, hideLoader: mockHideLoader }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => mockUseGetProjectFilters(),
}));

vi.mock("@features/support/api/useSearchConversations", () => ({
  useSearchConversations: () => mockUseSearchConversations(),
}));

vi.mock("@components/list-view/ListPageHeader", () => ({
  default: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

vi.mock("@components/list-view/ListSearchBar", () => ({
  default: () => <div data-testid="search-bar">SearchBar</div>,
}));

vi.mock("@features/support/components/all-conversations/AllConversationsList", () => ({
  default: ({
    onConversationClick,
  }: {
    onConversationClick: (conv: {
      id: string;
      number: string;
      initialMessage: string;
      createdOn: string;
      messageCount: number;
      state: { label: string };
      createdBy: string;
    }) => void;
  }) => (
    <button
      onClick={() =>
        onConversationClick({
          id: "conv-1",
          number: "CHAT-1",
          initialMessage: "Need help",
          createdOn: "2026-05-01T00:00:00Z",
          messageCount: 2,
          state: { label: "Open" },
          createdBy: "dev@wso2.com",
        })
      }
    >
      Open Conversation
    </button>
  ),
}));

vi.mock("@components/list-view/ListResultsBar", () => ({
  default: () => <div>ResultsBar</div>,
}));

vi.mock("@components/list-view/ListPagination", () => ({
  default: () => <div>Pagination</div>,
}));

describe("AllConversationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({ state: { returnTo: "/projects/project-1/support" } });
    mockUseSearchParams.mockReturnValue([new URLSearchParams(), vi.fn()]);
    mockUseGetProjectFilters.mockReturnValue({ data: { conversationStates: [] } });
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [], totalRecords: 0 },
      isLoading: false,
      isError: false,
    });

    mockUseSessionState.mockImplementation(
      (_key: string, initialValue: unknown) => [initialValue, vi.fn()],
    );
  });

  it("should hide search bar in status-filter mode", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("statusFilter=active"),
      vi.fn(),
    ]);

    render(<AllConversationsPage />);

    expect(screen.queryByTestId("search-bar")).not.toBeInTheDocument();
    expect(screen.getByText("Active Chats")).toBeInTheDocument();
  });

  it("should navigate with composed conversation summary", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [{ id: "conv-1" }], totalRecords: 1 },
      isLoading: false,
      isError: false,
    });

    render(<AllConversationsPage />);
    fireEvent.click(screen.getByText("Open Conversation"));

    expect(mockNavigate).toHaveBeenCalledWith(
      "/projects/project-1/support/conversations/conv-1",
      {
        state: {
          conversationSummary: expect.objectContaining({
            chatId: "conv-1",
            chatNumber: "CHAT-1",
            messages: 2,
          }),
        },
      },
    );
  });
});

