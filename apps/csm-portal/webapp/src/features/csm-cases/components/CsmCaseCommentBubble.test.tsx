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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
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

describe("CsmCaseCommentBubble", () => {
  it("renders comment body HTML", () => {
    render(<CsmCaseCommentBubble comment={makeComment({})} />);
    expect(screen.getByText("Hello there")).toBeInTheDocument();
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });

  it("returns null for a comment with no displayable content", () => {
    const { container } = render(
      <CsmCaseCommentBubble comment={makeComment({ bodyHtml: "<p></p>" })} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("strips a single [code]...[/code] wrapper before rendering", () => {
    render(
      <CsmCaseCommentBubble
        comment={makeComment({ bodyHtml: "[code]<b>raw</b>[/code]" })}
      />,
    );
    expect(screen.getByText("raw")).toBeInTheDocument();
  });

  it("linkifies a bare URL and opens it in a new tab safely", () => {
    render(
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
    render(
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
    );
  });

  it("renders a chatbot comment's markdown body as HTML", () => {
    render(
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

  it("renders a system comment as a compact inline row", () => {
    render(
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
});
