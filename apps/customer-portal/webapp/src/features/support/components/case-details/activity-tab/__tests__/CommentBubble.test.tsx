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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CommentBubble from "@case-details-activity/CommentBubble";
import type { CaseComment } from "@features/support/types/cases";

const mockComment: CaseComment = {
  id: "comment-1",
  content: "[code]<p>Thanks for the detailed recommendations.</p>[/code]",
  type: "comments",
  createdOn: "2026-02-12 11:15:42",
  createdBy: "support-engineer@wso2.com",
  isEscalated: false,
};

function renderBubble(
  props: {
    comment?: CaseComment;
    isCurrentUser?: boolean;
    primaryBg?: string;
    userDetails?: {
      email?: string;
      firstName?: string;
      lastName?: string;
    } | null;
  } = {},
) {
  const defaults = {
    comment: mockComment,
    isCurrentUser: false,
    primaryBg: "rgba(250,123,63,0.1)",
  };
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <CommentBubble {...defaults} {...props} />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

describe("CommentBubble", () => {
  it("shows a support engineer's icon in place of the circled W after the name", () => {
    renderBubble({
      comment: { ...mockComment, createdByFullName: "Jane Doe \u24E6" },
    });
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Support engineer" })).toBeInTheDocument();
    expect(screen.queryByText(/\u24E6/u)).not.toBeInTheDocument();
    // The avatar initials come from the name alone.
    expect(screen.getByText("JD")).toBeInTheDocument();
  });

  it("shows no icon for a name without the mark", () => {
    renderBubble({
      comment: { ...mockComment, createdByFullName: "Jane Doe" },
    });
    expect(screen.queryByRole("img", { name: "Support engineer" })).not.toBeInTheDocument();
  });

  it("hides Novera's <thinking> reasoning in the linked chat transcript", () => {
    renderBubble({
      comment: {
        ...mockComment,
        id: "novera-1",
        createdBy: "Novera",
        content:
          "<thinking>The user asks about a product.\nI should ask a follow-up.</thinking>\n\nWhich environment is this?",
      },
    });
    expect(screen.getByText(/Which environment is this\?/)).toBeInTheDocument();
    expect(screen.queryByText(/follow-up/)).not.toBeInTheDocument();
    expect(screen.queryByText(/thinking/i)).not.toBeInTheDocument();
  });

  it("leaves a person's own comment alone, even if it contains the tag", () => {
    renderBubble({
      comment: {
        ...mockComment,
        id: "person-1",
        content: "[code]<p>why does &lt;thinking&gt; show up?</p>[/code]",
      },
    });
    expect(screen.getByText(/why does <thinking> show up\?/)).toBeInTheDocument();
  });

  describe("laid-out HTML source", () => {
    const laidOut =
      "<p>Wrapped by hand\r\n   at a fixed width.</p>\r\n<ul>\r\n  <li>First point\r\n    continues here.</li>\r\n</ul>";

    it("renders no newline inside or between the blocks", () => {
      const { container } = renderBubble({
        comment: { ...mockComment, id: "laid-out-1", content: laidOut },
      });
      expect(container.querySelector("ul")?.textContent).toBe("First point continues here.");
      expect(container.querySelector("ul")?.previousElementSibling?.textContent).toBe(
        "Wrapped by hand at a fixed width.",
      );
      expect(container.querySelector("ul")?.parentElement?.innerHTML).not.toMatch(/[\r\n]/);
    });

    it("renders a body inside a [code] wrapper the same way", () => {
      const { container } = renderBubble({
        comment: { ...mockComment, id: "laid-out-2", content: `[code]${laidOut}[/code]` },
      });
      expect(container.querySelector("ul")?.parentElement?.innerHTML).not.toMatch(/[\r\n]/);
    });

    it("keeps the line breaks inside a <pre> block and an inline <code> snippet", () => {
      const { container } = renderBubble({
        comment: {
          ...mockComment,
          id: "code-1",
          content: "<p>Run:</p>\n<pre>one\n  two</pre>\n<p>or <code>x\ny</code></p>",
        },
      });
      expect(container.querySelector("pre")?.textContent).toBe("one\n  two");
      expect(container.querySelector("code")?.textContent).toBe("x\ny");
    });

    it("leaves Novera's Markdown answer alone", () => {
      renderBubble({
        comment: {
          ...mockComment,
          id: "novera-2",
          createdBy: "Novera",
          content: "<p>kept</p>\n<p>as is</p>",
        },
      });
      expect(screen.getByText(/as is/)).toBeInTheDocument();
    });
  });

  describe("plain-text comment (no HTML markup at all)", () => {
    it("renders a system-generated notice's real line breaks instead of one run-on line", () => {
      const { container } = renderBubble({
        comment: {
          ...mockComment,
          id: "plain-text-1",
          createdBy: "system",
          content:
            "Hello,\n\nPlease let us know whether you need further support for this case. Please note that if you are unable to respond, the case will be closed off after one week.\n\nThanks,\nWSO2 Team",
        },
      });
      expect(container.querySelectorAll("br").length).toBe(5);
      expect(screen.getByText(/Thanks,/)).toBeInTheDocument();
      expect(screen.getByText(/WSO2 Team/)).toBeInTheDocument();
    });
  });

  it("should render comment content", () => {
    renderBubble();
    expect(
      screen.getByText(/Thanks for the detailed recommendations/),
    ).toBeInTheDocument();
  });

  it("should show display name for non-current-user comment", () => {
    renderBubble({ isCurrentUser: false });
    expect(screen.getByText("support-engineer@wso2.com")).toBeInTheDocument();
  });

  // Regression: the real GET /conversations/{id}/messages response sends
  // createdBy: "" for a Novera reply, which used to fall through
  // commentAuthorDisplayName's final `|| "Unknown"` and render the literal
  // word "Unknown" as the author name instead of identifying it as Novera.
  it("shows 'Novera' rather than 'Unknown' for a comment with no createdBy", () => {
    renderBubble({
      comment: { ...mockComment, id: "novera-empty-createdby", createdBy: "" },
    });
    expect(screen.getByText("Novera")).toBeInTheDocument();
    expect(screen.queryByText("Unknown")).not.toBeInTheDocument();
  });

  it("should render formatted date", () => {
    renderBubble();
    expect(screen.getByText(/Feb 12, 2026/)).toBeInTheDocument();
  });
});
