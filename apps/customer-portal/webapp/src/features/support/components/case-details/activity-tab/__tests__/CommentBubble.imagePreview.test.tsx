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

const downloadHook = vi.fn();
vi.mock("@api/useStoredAttachments", () => ({
  useStoredAttachmentDownload: () => downloadHook(),
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

function renderBubble(comment: CaseComment = attached, onImageClick?: (src: string) => void) {
  const queryClient = new QueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <CommentBubble
          comment={comment}
          isCurrentUser={false}
          primaryBg="rgba(250,123,63,0.1)"
          onImageClick={onImageClick}
        />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

describe("CommentBubble image attachment preview", () => {
  beforeEach(() => {
    downloadHook.mockReset();
    downloadHook.mockReturnValue({
      downloadStoredAttachment: vi.fn(),
      isDownloadingId: () => false,
      downloadProgressOf: () => null,
      cancelDownload: vi.fn(),
    });
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

  it("opens the full image from the keyboard as well as the mouse", () => {
    const onImageClick = vi.fn();
    renderBubble(attached, onImageClick);
    previewHook.mockReturnValue({
      data: "data:image/png;base64,AAAA",
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    });
    fireEvent.click(screen.getByRole("button", { name: /preview/i }));
    const thumb = screen.getByRole("button", { name: /open screenshot.png full size/i });
    expect(thumb).toHaveAttribute("tabindex", "0");
    fireEvent.keyDown(thumb, { key: "Enter" });
    fireEvent.keyDown(thumb, { key: " " });
    fireEvent.keyDown(thumb, { key: "a" });
    fireEvent.click(thumb);
    expect(onImageClick).toHaveBeenCalledTimes(3);
    expect(onImageClick).toHaveBeenCalledWith("data:image/png;base64,AAAA");
  });

  it("shows the download's progress and disables the button while it runs", () => {
    downloadHook.mockReturnValue({
      downloadStoredAttachment: vi.fn(),
      isDownloadingId: (id: string) => id === "att-1",
      downloadProgressOf: () => 37,
      cancelDownload: vi.fn(),
    });
    renderBubble();
    expect(screen.getByRole("progressbar", { name: "Downloading" })).toHaveAttribute("aria-valuenow", "37");
    expect(screen.getByText("37%")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Download attachment" })).toBeDisabled();
  });

  it("offers Cancel while it downloads, and cancels that attachment's download", () => {
    const cancelDownload = vi.fn();
    downloadHook.mockReturnValue({
      downloadStoredAttachment: vi.fn(),
      isDownloadingId: (id: string) => id === "att-1",
      downloadProgressOf: () => 12,
      cancelDownload,
    });
    renderBubble();
    fireEvent.click(screen.getByRole("button", { name: "Cancel download of screenshot.png" }));
    expect(cancelDownload).toHaveBeenCalledWith("att-1");
  });

  it("shows no Cancel button when nothing downloads", () => {
    renderBubble();
    expect(screen.queryByRole("button", { name: /cancel download/i })).not.toBeInTheDocument();
  });

  it("shows the plain download icon when nothing downloads", () => {
    renderBubble();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Download attachment" })).toBeEnabled();
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
