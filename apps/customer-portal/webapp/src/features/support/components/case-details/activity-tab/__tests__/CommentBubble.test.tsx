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

  it("should render formatted date", () => {
    renderBubble();
    expect(screen.getByText(/Feb 12, 2026/)).toBeInTheDocument();
  });
});
