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
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CommentBubble from "@case-details-activity/CommentBubble";
import type { CaseComment } from "@features/support/types/cases";

const previewHook = vi.fn();
vi.mock("@api/useAttachmentPreview", () => ({
  useAttachmentPreview: (...args: unknown[]) => previewHook(...args),
}));

const attached: CaseComment = {
  id: "att-1",
  content: "",
  type: "attachment",
  fileName: "screenshot.png",
  contentType: "image/png",
  sizeBytes: 1000,
  createdOn: "2026-02-12 11:15:42",
  createdBy: "someone@example.com",
  isEscalated: false,
};

function renderBubble(comment: CaseComment = attached) {
  const queryClient = new QueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <CommentBubble
          comment={comment}
          isCurrentUser={false}
          primaryBg="rgba(250,123,63,0.1)"
        />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

describe("CommentBubble image attachment preview", () => {
  beforeEach(() => {
    previewHook.mockReset();
    previewHook.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
  });

  it("does not fetch the image until Preview is clicked", () => {
    renderBubble();
    expect(previewHook).toHaveBeenLastCalledWith(null);
    expect(screen.getByRole("button", { name: /preview/i })).toBeInTheDocument();
  });

  it("fetches the image once Preview is clicked and shows it", () => {
    renderBubble();
    previewHook.mockReturnValue({
      data: "data:image/png;base64,AAAA",
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    expect(previewHook).toHaveBeenLastCalledWith("att-1");
    expect(screen.getByAltText("screenshot.png")).toHaveAttribute(
      "src",
      "data:image/png;base64,AAAA",
    );
    expect(screen.queryByRole("button", { name: /^preview$/i })).not.toBeInTheDocument();
  });

  it("offers Try again when the preview could not be loaded", () => {
    const refetch = vi.fn();
    renderBubble();
    previewHook.mockReturnValue({ data: undefined, isLoading: false, isError: true, refetch });
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    expect(screen.getByText(/could not be loaded/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("shows no Preview button for a file that is not a previewable image", () => {
    renderBubble({ ...attached, fileName: "report.pdf", contentType: "application/pdf" });
    expect(screen.queryByRole("button", { name: /preview/i })).not.toBeInTheDocument();
  });
});
