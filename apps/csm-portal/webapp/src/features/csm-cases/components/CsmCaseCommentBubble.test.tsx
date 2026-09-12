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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router";
import CsmCaseCommentBubble from "@features/csm-cases/components/CsmCaseCommentBubble";
import type { CsmCaseComment } from "@features/csm-cases/types/csmCases";

vi.mock("@features/csm-cases/api/useResolvedInlineImageHtml", () => ({
  // Pass the sanitized HTML straight through — no attachment resolution in
  // these tests, which don't exercise the react-query/backend-client path.
  useResolvedInlineImageHtml: vi.fn((html: string) => ({
    resolvedHtml: html,
    isLoading: false,
  })),
}));

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useQuickCaseSearch.test.tsx). The comment
// author name renders through `UserRefLink`, which resolves an unknown id
// through `useResolvedUserId`, which calls this client.
const searchUsersByEmail = vi.fn().mockResolvedValue({ users: [] });
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: searchUsersByEmail }),
}));

function makeComment(overrides: Partial<CsmCaseComment>): CsmCaseComment {
  return {
    id: "c-1",
    caseId: "case-1",
    authorName: "Jane Doe",
    authorRole: "customer",
    bodyHtml: "<p>Hello there</p>",
    createdAt: "2026-07-01T00:00:00Z",
    ...overrides,
  };
}

// `UserRefLink` (used for the comment author) renders a `react-router` `Link`
// and resolves its id through react-query — needs both a Router and a
// QueryClient context even outside a full app render.
function renderWithProviders(ui: ReactElement): ReturnType<typeof render> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CsmCaseCommentBubble", () => {
  it("renders comment body HTML", () => {
    renderWithProviders(<CsmCaseCommentBubble comment={makeComment({})} />);
    expect(screen.getByText("Hello there")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("returns null for a comment with no displayable content", () => {
    const { container } = renderWithProviders(
      <CsmCaseCommentBubble comment={makeComment({ bodyHtml: "<p></p>" })} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("strips a single [code]...[/code] wrapper before rendering", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({ bodyHtml: "[code]<b>raw</b>[/code]" })}
      />,
    );
    expect(screen.getByText("raw")).toBeInTheDocument();
  });

  it("linkifies a bare URL and opens it in a new tab safely", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({ bodyHtml: "See https://example.com/doc" })}
      />,
    );
    const link = screen.getByRole("link", { name: /example\.com/ });
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("invokes onImageClick when an inline image is clicked", () => {
    // A relative unresolved-attachment-style src (as an unresolved .iix
    // reference would look) — a bare `https://` src would also get rewritten
    // by linkifyBareUrls, which only special-cases `href=`, so it's avoided
    // here to keep this test focused on the click-to-zoom wiring.
    const onImageClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml: '<img src="/abc123.iix" alt="a" />',
        })}
        onImageClick={onImageClick}
      />,
    );
    const img = screen.getByRole("button", { name: "Open image preview" });
    fireEvent.click(img);
    expect(onImageClick).toHaveBeenCalledWith(
      expect.stringContaining("abc123.iix"),
      "a",
    );
  });

  it("does not mark inline images as interactive when onImageClick is not provided", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml: '<img src="/abc123.iix" alt="a" />',
        })}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Open image preview" }),
    ).not.toBeInTheDocument();
  });

  it("renders a chatbot comment's markdown body as HTML", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorRole: "chatbot",
          authorName: "Novera",
          bodyHtml: "**bold answer**",
        })}
      />,
    );
    expect(screen.getByText("bold answer")).toBeInTheDocument();
  });

  it("resolves the author link from the canonical email when there is no legacy email and no id", async () => {
    // Regression for a null canonical id + empty legacy `authorEmail`: the
    // author link must still resolve through `comment.authorUser.email`
    // rather than silently falling back to plain text.
    searchUsersByEmail.mockResolvedValueOnce({
      users: [{ id: "user-42", email: "canonical@example.com" }],
    });
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorName: "Jane Doe",
          authorEmail: undefined,
          authorUser: {
            id: null,
            email: "canonical@example.com",
            name: "Jane Doe",
          },
        })}
      />,
    );
    const link = await screen.findByRole("link", { name: "Jane Doe" });
    expect(link).toHaveAttribute("href", "/people/user-42");
  });

  it("turns a bare ServiceNow call-request URL into an in-app clickable marker", () => {
    const onCallRequestClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "See https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41 for details",
        })}
        onCallRequestClick={onCallRequestClick}
      />,
    );
    // Must not fall through to the raw-URL linkifier (only the unrelated
    // "N ago" permalink anchor from RelativeTime should be present).
    expect(
      screen.queryByRole("link", { name: /example\.com|sn_customerservice/i }),
    ).not.toBeInTheDocument();
    const marker = screen.getByRole("button", { name: "View call request" });
    fireEvent.click(marker);
    expect(onCallRequestClick).toHaveBeenCalledWith(
      "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41",
    );
  });

  it("invokes onCallRequestClick on Enter/Space keydown for the call-request marker", () => {
    const onCallRequestClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41",
        })}
        onCallRequestClick={onCallRequestClick}
      />,
    );
    const marker = screen.getByRole("button", { name: "View call request" });
    fireEvent.keyDown(marker, { key: "Enter" });
    expect(onCallRequestClick).toHaveBeenCalledWith(
      "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41",
    );
  });

  it("does nothing on click when onCallRequestClick is not provided", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41",
        })}
      />,
    );
    const marker = screen.getByRole("button", { name: "View call request" });
    expect(() => fireEvent.click(marker)).not.toThrow();
  });

  it("turns a bare alert URL into an in-app clickable marker and dispatches its type/id", () => {
    const onSnLinkClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "See https://sn-dev.example.com/u_custom_alert.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41 for details",
        })}
        onSnLinkClick={onSnLinkClick}
      />,
    );
    expect(
      screen.queryByRole("link", { name: /example\.com|u_custom_alert/i }),
    ).not.toBeInTheDocument();
    const marker = screen.getByRole("button", { name: "View alert" });
    fireEvent.click(marker);
    expect(onSnLinkClick).toHaveBeenCalledWith(
      "alert",
      "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41",
    );
  });

  it("turns a bare smart-alert URL into an in-app clickable marker and dispatches its type/id", () => {
    const onSnLinkClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "https://sn-dev.example.com/u_smart_alert_buffer.do?sys_id=00000000000000000000000000000001",
        })}
        onSnLinkClick={onSnLinkClick}
      />,
    );
    const marker = screen.getByRole("button", { name: "View smart alert" });
    fireEvent.click(marker);
    expect(onSnLinkClick).toHaveBeenCalledWith(
      "smartAlert",
      "00000000-0000-0000-0000-000000000001",
    );
  });

  it("invokes onSnLinkClick on Enter/Space keydown for an alert marker", () => {
    const onSnLinkClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "https://sn-dev.example.com/u_custom_alert.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41",
        })}
        onSnLinkClick={onSnLinkClick}
      />,
    );
    const marker = screen.getByRole("button", { name: "View alert" });
    fireEvent.keyDown(marker, { key: "Enter" });
    expect(onSnLinkClick).toHaveBeenCalledWith(
      "alert",
      "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41",
    );
  });

  it("does nothing on click when onSnLinkClick is not provided", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            "https://sn-dev.example.com/u_custom_alert.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41",
        })}
      />,
    );
    const marker = screen.getByRole("button", { name: "View alert" });
    expect(() => fireEvent.click(marker)).not.toThrow();
  });

  it("does not invoke onSnLinkClick for an unrecognized data-sn-link-type value", () => {
    const onSnLinkClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            '<span data-sn-link-type="other" data-sn-link-id="7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41" role="button" tabindex="0">Suspicious marker</span>',
        })}
        onSnLinkClick={onSnLinkClick}
      />,
    );
    const marker = screen.getByRole("button", { name: "Suspicious marker" });
    fireEvent.click(marker);
    fireEvent.keyDown(marker, { key: "Enter" });
    expect(onSnLinkClick).not.toHaveBeenCalled();
  });

  it("preserves the original call-request task-number text as the clickable label", () => {
    const onCallRequestClick = vi.fn();
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          bodyHtml:
            'Case Task <a href="https://sn-dev.example.com/sn_customerservice_customer_call.do?sys_id=7a43e2d43b2a4b5091404c6aa5e45a41">CTASK0012345</a> has been created',
        })}
        onCallRequestClick={onCallRequestClick}
      />,
    );
    const marker = screen.getByRole("button", { name: "CTASK0012345" });
    fireEvent.click(marker);
    expect(onCallRequestClick).toHaveBeenCalledWith(
      "7a43e2d4-3b2a-4b50-9140-4c6aa5e45a41",
    );
    expect(screen.queryByText("View call request")).not.toBeInTheDocument();
  });

  it("renders a system comment as a compact inline row", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorRole: "system",
          bodyHtml: "<p>Case reassigned</p>",
        })}
      />,
    );
    expect(screen.getByText("System")).toBeInTheDocument();
    expect(screen.getByText("Case reassigned")).toBeInTheDocument();
  });

  it("renders an avatar and no 'Commented by' prefix by default", () => {
    renderWithProviders(<CsmCaseCommentBubble comment={makeComment({})} />);
    expect(document.querySelector(".MuiAvatar-root")).toBeInTheDocument();
    expect(screen.queryByText(/Commented by/)).not.toBeInTheDocument();
  });

  it("drops the avatar and prefixes the name with 'Commented by' in compact mode", () => {
    renderWithProviders(<CsmCaseCommentBubble comment={makeComment({})} compact />);
    expect(document.querySelector(".MuiAvatar-root")).not.toBeInTheDocument();
    expect(screen.getByText(/Commented by/)).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("suppresses the role chip for a synthetic (client-injected) comment even though authorRole is 'customer'", () => {
    // `synthetic` is used for entries the frontend fabricates itself (e.g.
    // the case description echoed into the activity feed) — the real
    // author's role is unknown in that case, so no chip should claim one,
    // regardless of what placeholder `authorRole` the entry carries.
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({ authorRole: "customer", synthetic: true })}
      />,
    );
    expect(screen.queryByText("Customer")).not.toBeInTheDocument();
  });

  it("still shows the role chip for a non-synthetic customer comment", () => {
    renderWithProviders(
      <CsmCaseCommentBubble comment={makeComment({ authorRole: "customer" })} />,
    );
    expect(screen.getByText("Customer")).toBeInTheDocument();
  });
});
