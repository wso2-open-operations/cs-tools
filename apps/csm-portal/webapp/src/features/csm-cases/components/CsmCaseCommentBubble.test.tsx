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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
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

// The bubble's edit/delete affordances gate on the signed-in user via
// `useCurrentUser` — mocked per the repo's own convention (see
// `WidgetEditorDialog.test.tsx`) rather than rendering a real
// `CurrentUserProvider`, which would itself hit the real backend client.
const mockCurrentUser = vi.fn<() => { email?: string; roles?: string[] } | undefined>(
  () => undefined,
);
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: mockCurrentUser(),
    isLoading: false,
    isError: false,
    error: null,
  }),
}));

// The inline editor (rendered on Edit) mounts the real rich-text `Editor`,
// which logs via `useLogger` and its toolbar reads `useErrorBanner` — both
// require a provider this test doesn't otherwise set up. Same pattern as
// `Editor.pasteFormatPrompt.test.tsx`.
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    warn: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
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
  beforeEach(() => {
    mockCurrentUser.mockReturnValue(undefined);
  });

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

  it("marks an internal work note with a persistent 'Internal note' chip", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorRole: "wso2_engineer",
          bodyHtml: "<p>Only the team should see this.</p>",
          internal: true,
        })}
      />,
    );
    expect(screen.getByText("Internal note")).toBeInTheDocument();
  });

  it("does not show the 'Internal note' chip on a public comment", () => {
    renderWithProviders(
      <CsmCaseCommentBubble
        comment={makeComment({
          authorRole: "wso2_engineer",
          bodyHtml: "<p>Visible to the customer.</p>",
          internal: false,
        })}
      />,
    );
    expect(screen.queryByText("Internal note")).not.toBeInTheDocument();
  });

  describe("edit/delete affordances", () => {
    it("shows the comment-actions menu for the comment's own author", () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "Jane.Doe@example.com" })}
          onEditComment={vi.fn()}
          onDeleteComment={vi.fn()}
        />,
      );
      expect(
        screen.getByRole("button", { name: "Comment actions" }),
      ).toBeInTheDocument();
    });

    it("shows the comment-actions menu for an admin who isn't the author", () => {
      mockCurrentUser.mockReturnValue({
        email: "admin@example.com",
        roles: ["admin"],
      });
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "jane.doe@example.com" })}
          onEditComment={vi.fn()}
          onDeleteComment={vi.fn()}
        />,
      );
      expect(
        screen.getByRole("button", { name: "Comment actions" }),
      ).toBeInTheDocument();
    });

    it("hides the comment-actions menu for a non-author, non-admin caller", () => {
      mockCurrentUser.mockReturnValue({
        email: "someone.else@example.com",
        roles: ["cs_engineer"],
      });
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "jane.doe@example.com" })}
          onEditComment={vi.fn()}
          onDeleteComment={vi.fn()}
        />,
      );
      expect(
        screen.queryByRole("button", { name: "Comment actions" }),
      ).not.toBeInTheDocument();
    });

    it("hides the comment-actions menu when no signed-in user is available", () => {
      mockCurrentUser.mockReturnValue(undefined);
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "jane.doe@example.com" })}
          onEditComment={vi.fn()}
          onDeleteComment={vi.fn()}
        />,
      );
      expect(
        screen.queryByRole("button", { name: "Comment actions" }),
      ).not.toBeInTheDocument();
    });

    it("hides the comment-actions menu when the caller doesn't pass edit/delete callbacks, even for the author", () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "jane.doe@example.com" })}
        />,
      );
      expect(
        screen.queryByRole("button", { name: "Comment actions" }),
      ).not.toBeInTheDocument();
    });

    it("opens an inline editor and calls onEditComment with the edited content on Save", async () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      const onEditComment = vi.fn().mockResolvedValue(undefined);
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({
            authorEmail: "jane.doe@example.com",
            bodyHtml: "<p>Original text</p>",
          })}
          onEditComment={onEditComment}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Comment actions" }));
      fireEvent.click(screen.getByRole("menuitem", { name: /Edit/ }));

      // The rich-text editor renders its own contenteditable host rather than a
      // plain <textarea> — switch to the HTML-source mode (already used
      // elsewhere in this codebase, e.g. CsmCaseCommentInput) to edit via a
      // real form control the test can drive directly.
      fireEvent.click(screen.getByText("HTML source"));
      const sourceField = screen.getByPlaceholderText("<p>Type HTML here…</p>");
      fireEvent.change(sourceField, { target: { value: "<p>Edited text</p>" } });

      fireEvent.click(screen.getByRole("button", { name: "Save" }));

      await waitFor(() =>
        expect(onEditComment).toHaveBeenCalledWith("<p>Edited text</p>"),
      );
    });

    it("reverts to the unedited view on Cancel without calling onEditComment", () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      const onEditComment = vi.fn();
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({
            authorEmail: "jane.doe@example.com",
            bodyHtml: "<p>Original text</p>",
          })}
          onEditComment={onEditComment}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Comment actions" }));
      fireEvent.click(screen.getByRole("menuitem", { name: /Edit/ }));
      expect(screen.getByText("Save")).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

      expect(screen.queryByText("Save")).not.toBeInTheDocument();
      expect(screen.getByText("Original text")).toBeInTheDocument();
      expect(onEditComment).not.toHaveBeenCalled();
    });

    it("shows a confirmation dialog before calling onDeleteComment", async () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      const onDeleteComment = vi.fn().mockResolvedValue(undefined);
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "jane.doe@example.com" })}
          onDeleteComment={onDeleteComment}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Comment actions" }));
      fireEvent.click(screen.getByRole("menuitem", { name: /Delete/ }));

      // Not called yet — the confirm dialog is up first.
      expect(onDeleteComment).not.toHaveBeenCalled();
      expect(screen.getByText("Delete comment?")).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Delete" }));

      await waitFor(() => expect(onDeleteComment).toHaveBeenCalledTimes(1));
    });

    it("does not call onDeleteComment when the confirmation dialog is canceled", async () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      const onDeleteComment = vi.fn();
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ authorEmail: "jane.doe@example.com" })}
          onDeleteComment={onDeleteComment}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Comment actions" }));
      fireEvent.click(screen.getByRole("menuitem", { name: /Delete/ }));
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

      // MUI's Dialog exit transition keeps the node mounted briefly.
      await waitFor(() =>
        expect(screen.queryByText("Delete comment?")).not.toBeInTheDocument(),
      );
      expect(onDeleteComment).not.toHaveBeenCalled();
    });

    it('renders the "(edited)" marker when the comment has been edited', () => {
      renderWithProviders(
        <CsmCaseCommentBubble comment={makeComment({ isEdited: true })} />,
      );
      expect(screen.getByText("(edited)")).toBeInTheDocument();
    });

    it('does not render the "(edited)" marker for a never-edited comment', () => {
      renderWithProviders(<CsmCaseCommentBubble comment={makeComment({})} />);
      expect(screen.queryByText("(edited)")).not.toBeInTheDocument();
    });

    it("renders a deleted-comment visual treatment when isDeleted is true", () => {
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({ isDeleted: true, bodyHtml: "[deleted]" })}
        />,
      );
      expect(screen.getByText("This comment was deleted.")).toBeInTheDocument();
      expect(screen.getByText("Deleted")).toBeInTheDocument();
      // Whatever `content` the backend returned still renders — no client-side
      // redaction/branching on the text.
      expect(screen.getByText("[deleted]")).toBeInTheDocument();
    });

    it("renders an admin's real content on a deleted comment as-is, with no special-casing", () => {
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({
            isDeleted: true,
            bodyHtml: "<p>The real original message</p>",
          })}
        />,
      );
      expect(screen.getByText("The real original message")).toBeInTheDocument();
      expect(screen.getByText("Deleted")).toBeInTheDocument();
    });

    it("never shows edit/delete affordances on a deleted comment even for its own author", () => {
      mockCurrentUser.mockReturnValue({ email: "jane.doe@example.com" });
      renderWithProviders(
        <CsmCaseCommentBubble
          comment={makeComment({
            authorEmail: "jane.doe@example.com",
            isDeleted: true,
          })}
          onEditComment={vi.fn()}
          onDeleteComment={vi.fn()}
        />,
      );
      expect(
        screen.queryByRole("button", { name: "Comment actions" }),
      ).not.toBeInTheDocument();
    });
  });
});
