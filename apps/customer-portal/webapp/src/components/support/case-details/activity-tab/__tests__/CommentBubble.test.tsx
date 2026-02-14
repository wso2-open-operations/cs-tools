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
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CommentBubble from "@case-details-activity/CommentBubble";
import type { CaseComment } from "@models/responses";

vi.mock("dompurify", () => ({
  default: {
    sanitize: (html: string) => html,
  },
}));

const mockComment: CaseComment = {
  id: "comment-1",
  content: "[code]<p>Thanks for the detailed recommendations.</p>[/code]",
  type: "comments",
  createdOn: "2026-02-12 11:15:42",
  createdBy: "support-engineer@wso2.com",
  isEscalated: false,
};

function renderBubble(props: {
  comment?: CaseComment;
  isCurrentUser?: boolean;
  primaryBg?: string;
  userDetails?: { email?: string; firstName?: string; lastName?: string } | null;
} = {}) {
  const defaults = {
    comment: mockComment,
    isCurrentUser: false,
    primaryBg: "rgba(250,123,63,0.1)",
  };
  return render(
    <ThemeProvider theme={createTheme()}>
      <CommentBubble {...defaults} {...props} />
    </ThemeProvider>,
  );
}

describe("CommentBubble", () => {
  it("should render comment content", () => {
    renderBubble();
    expect(screen.getByText(/Thanks for the detailed recommendations/)).toBeInTheDocument();
  });

  it("should show display name for non-current-user comment", () => {
    renderBubble({ isCurrentUser: false });
    expect(screen.getByText("support-engineer@wso2.com")).toBeInTheDocument();
  });

  it("should show Support Engineer chip for non-current-user", () => {
    renderBubble({ isCurrentUser: false });
    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
  });

  it("should not show Support Engineer chip for current user", () => {
    renderBubble({ isCurrentUser: true });
    expect(screen.queryByText("Support Engineer")).not.toBeInTheDocument();
  });

  it("should render formatted date", () => {
    renderBubble();
    expect(screen.getByText(/Feb 12, 2026/)).toBeInTheDocument();
  });
});
