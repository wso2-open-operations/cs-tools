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
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ConversationsTab from "@features/csm-projects/components/ConversationsTab";
import type { BeConversationView } from "@api/backend/types";

// QueryErrorState imports BackendApiError from @api/backend/client, whose
// module reads window.config (via @config/apiConfig) at load time —
// unavailable under vitest. Mock the config so that module evaluates cleanly;
// useBackendApi itself is never called here (useSearchConversations is mocked
// below), so it doesn't need its own mock.
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

const mockUseSearchConversations = vi.fn();

vi.mock("@features/csm-projects/api/useSearchConversations", () => ({
  useSearchConversations: (...args: unknown[]) => mockUseSearchConversations(...args),
}));

// Avoids resolving a user id for UserRefLink's underlying email lookup —
// keeps these tests focused on the tab's own rendering/navigation.
vi.mock("@features/csm-users/api/useResolvedUserId", () => ({
  useResolvedUserId: () => undefined,
}));

// The eye-icon preview drawer fetches messages via a real hook — irrelevant
// here (its own coverage lives in ConversationPreviewContent tests).
vi.mock("@features/csm-cases/api/useCsmConversationMessages", () => ({
  useGetCsmConversationMessages: () => ({ data: [], isLoading: false, isError: false }),
}));

function conversation(overrides: Partial<BeConversationView> = {}): BeConversationView {
  return {
    id: "conv-1",
    number: "CONV0000001",
    initialMessage: "Hi, need help",
    messageCount: 4,
    project: { id: "proj-1", name: "Acme" },
    case: null,
    state: "ACTIVE",
    createdOn: "2026-07-01T10:00:00Z",
    createdBy: { id: null, email: "jane@example.com", name: "Jane Doe" },
    ...overrides,
  };
}

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-probe">{location.pathname}</div>;
}

function renderTab(projectId = "proj-1") {
  return render(
    <MemoryRouter initialEntries={["/customers/projects/proj-1"]}>
      <Routes>
        <Route path="/customers/projects/proj-1" element={<ConversationsTab projectId={projectId} />} />
        <Route path="/conversations/:id" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ConversationsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a loading skeleton while the search is in flight", () => {
    mockUseSearchConversations.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    });

    renderTab();

    expect(screen.queryByText("No chat sessions found for this project.")).not.toBeInTheDocument();
  });

  it("shows an error state when the search fails", () => {
    mockUseSearchConversations.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("boom"),
    });

    renderTab();

    expect(screen.getByText("boom")).toBeInTheDocument();
  });

  it("shows an empty state when the project has no conversations", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [], total: 0 },
      isLoading: false,
      isError: false,
      error: null,
    });

    renderTab();

    expect(screen.getByText("No chat sessions found for this project.")).toBeInTheDocument();
  });

  it("lists conversations with the new columns and navigates to the dedicated page on row click", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [conversation()], total: 1 },
      isLoading: false,
      isError: false,
      error: null,
    });

    renderTab();

    expect(screen.getByText("CONV0000001")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Jane Doe"));

    expect(screen.getByTestId("location-probe")).toHaveTextContent("/conversations/conv-1");
  });

  it("groups RESOLVED/ABANDONED/CLOSED into one 'Closed' chip and CONVERTED into its own chip", () => {
    mockUseSearchConversations.mockReturnValue({
      data: {
        conversations: [
          conversation({ id: "conv-2", number: "CONV0000002", state: "RESOLVED" }),
          conversation({ id: "conv-3", number: "CONV0000003", state: "CONVERTED" }),
          conversation({ id: "conv-4", number: "CONV0000004", state: "ABANDONED" }),
        ],
        total: 3,
      },
      isLoading: false,
      isError: false,
      error: null,
    });

    renderTab();

    expect(screen.getAllByText("Closed")).toHaveLength(2);
    expect(screen.getByText("Converted")).toBeInTheDocument();
  });

  it("renders a dash for a conversation with no resolved state", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [conversation({ state: null })], total: 1 },
      isLoading: false,
      isError: false,
      error: null,
    });

    renderTab();

    expect(screen.queryByText("Active")).not.toBeInTheDocument();
    expect(screen.queryByText("Closed")).not.toBeInTheDocument();
  });

  it("opens the preview drawer from the eye icon without navigating", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [conversation()], total: 1 },
      isLoading: false,
      isError: false,
      error: null,
    });

    renderTab();

    fireEvent.click(screen.getByRole("button", { name: /Quick preview/i }));

    expect(screen.queryByTestId("location-probe")).not.toBeInTheDocument();
    expect(screen.getByText("Chat session")).toBeInTheDocument();
  });

  it("resets pagination and filters when projectId changes", () => {
    mockUseSearchConversations.mockReturnValue({
      data: { conversations: [conversation()], total: 100 },
      isLoading: false,
      isError: false,
      error: null,
    });

    const { rerender } = render(
      <MemoryRouter>
        <ConversationsTab projectId="proj-1" />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    expect(mockUseSearchConversations).toHaveBeenLastCalledWith(
      "proj-1",
      expect.objectContaining({ page: 1 }),
      expect.anything(),
    );

    rerender(
      <MemoryRouter>
        <ConversationsTab projectId="proj-2" />
      </MemoryRouter>,
    );

    expect(mockUseSearchConversations).toHaveBeenLastCalledWith(
      "proj-2",
      expect.objectContaining({ page: 0 }),
      expect.anything(),
    );
  });
});
