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
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ConversationPreviewContent from "@features/csm-projects/components/ConversationPreviewContent";
import type { BeConversationView } from "@api/backend/types";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

const mockUseGetCsmConversationMessages = vi.fn();

vi.mock("@features/csm-cases/api/useCsmConversationMessages", () => ({
  useGetCsmConversationMessages: (...args: unknown[]) =>
    mockUseGetCsmConversationMessages(...args),
}));

vi.mock("@features/csm-cases/components/CsmCaseCommentBubble", () => ({
  default: ({ comment }: { comment: CsmCaseComment }) => <div>Message: {comment.bodyHtml}</div>,
}));

vi.mock("@features/csm-users/api/useResolvedUserId", () => ({
  useResolvedUserId: () => undefined,
}));

function conversation(overrides: Partial<BeConversationView> = {}): BeConversationView {
  return {
    id: "conv-1",
    number: "CONV0000001",
    initialMessage: "Hi, need help",
    messageCount: 1,
    project: { id: "proj-1", name: "Acme" },
    case: null,
    state: "ACTIVE",
    createdOn: "2026-07-01T10:00:00Z",
    createdBy: { id: null, email: "jane@example.com", name: "Jane Doe" },
    ...overrides,
  };
}

function message(overrides: Partial<CsmCaseComment> = {}): CsmCaseComment {
  return {
    id: "msg-1",
    caseId: "",
    authorName: "Jane Doe",
    authorRole: "customer",
    bodyHtml: "Hi, need help",
    createdAt: "2026-07-01T10:00:00Z",
    ...overrides,
  };
}

describe("ConversationPreviewContent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the last few messages and the conversation summary", () => {
    mockUseGetCsmConversationMessages.mockReturnValue({
      data: [message()],
      isLoading: false,
      isError: false,
    });

    render(
      <MemoryRouter>
        <ConversationPreviewContent conversation={conversation()} onClose={vi.fn()} />
      </MemoryRouter>,
    );

    expect(screen.getByText("Message: Hi, need help")).toBeInTheDocument();
    expect(screen.getByText("CONV0000001")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("shows a 'View case' button when the conversation became a case", () => {
    mockUseGetCsmConversationMessages.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    });

    render(
      <MemoryRouter>
        <ConversationPreviewContent
          conversation={conversation({ case: { id: "case-1", name: "CS0000001" } })}
          onClose={vi.fn()}
        />
      </MemoryRouter>,
    );

    const link = screen.getByText("View case CS0000001").closest("a");
    expect(link).toHaveAttribute("href", "/cases/case-1");
  });

  it("links 'View full details' to the conversation's dedicated page", () => {
    mockUseGetCsmConversationMessages.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    });

    render(
      <MemoryRouter>
        <ConversationPreviewContent conversation={conversation()} onClose={vi.fn()} />
      </MemoryRouter>,
    );

    const link = screen.getByText("View full details").closest("a");
    expect(link).toHaveAttribute("href", "/conversations/conv-1");
  });

  it("calls onClose when the close button is clicked", () => {
    const onClose = vi.fn();
    mockUseGetCsmConversationMessages.mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    });

    render(
      <MemoryRouter>
        <ConversationPreviewContent conversation={conversation()} onClose={onClose} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByLabelText("Close preview"));

    expect(onClose).toHaveBeenCalled();
  });
});
