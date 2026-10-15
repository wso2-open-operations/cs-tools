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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AnnouncementAttachmentsPanel from "@features/announcements/components/AnnouncementAttachmentsPanel";
import type { CaseAttachment } from "@features/support/types/cases";

const useGetCaseAttachmentsMock = vi.fn();
const downloadAttachmentMock = vi.fn();
const showErrorMock = vi.fn();
const refetchMock = vi.fn();
const fetchNextPageMock = vi.fn();

vi.mock("@features/support/api/useGetCaseAttachments", async () => {
  const actual = await vi.importActual<typeof import("@features/support/api/useGetCaseAttachments")>(
    "@features/support/api/useGetCaseAttachments",
  );
  return { ...actual, useGetCaseAttachments: (caseId: string) => useGetCaseAttachmentsMock(caseId) };
});
vi.mock("@api/useGetAttachmentContent", () => ({
  useGetAttachmentContent: () => ({
    downloadAttachment: downloadAttachmentMock,
    isDownloading: false,
    downloadingId: null,
  }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: showErrorMock }),
}));

function attachment(name: string, overrides: Partial<CaseAttachment> = {}): CaseAttachment {
  return {
    id: `att-${name}`,
    name,
    type: "application/pdf",
    size: 2048,
    sizeBytes: "2048",
    createdBy: "wso2.agent@example.com",
    createdOn: "2026-02-13 10:00:00",
    downloadUrl: null,
    ...overrides,
  } as CaseAttachment;
}

/** The infinite query's result for the given first page of files. */
function loaded(files: CaseAttachment[], extra: Record<string, unknown> = {}) {
  return {
    data: { pages: [{ attachments: files, offset: 0, limit: 10, totalRecords: files.length }] },
    isLoading: false,
    isError: false,
    refetch: refetchMock,
    hasNextPage: false,
    fetchNextPage: fetchNextPageMock,
    isFetchingNextPage: false,
    ...extra,
  };
}

function renderPanel(caseId = "case-1") {
  // The real list item previews images through a query, so it needs a client.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <AnnouncementAttachmentsPanel caseId={caseId} />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useGetCaseAttachmentsMock.mockReset();
  downloadAttachmentMock.mockReset();
  downloadAttachmentMock.mockResolvedValue(undefined);
  showErrorMock.mockReset();
  refetchMock.mockReset();
  fetchNextPageMock.mockReset();
  useGetCaseAttachmentsMock.mockReturnValue(loaded([]));
});

describe("AnnouncementAttachmentsPanel", () => {
  it("reads the attachments of the announcement's own case", () => {
    renderPanel("case-9");
    expect(useGetCaseAttachmentsMock).toHaveBeenCalledWith("case-9");
  });

  it("lists the announcement's files under an Attachments heading", () => {
    useGetCaseAttachmentsMock.mockReturnValue(loaded([attachment("advisory.pdf"), attachment("notes.pdf")]));
    renderPanel();

    expect(screen.getByText("Attachments")).toBeInTheDocument();
    expect(screen.getByText("advisory.pdf")).toBeInTheDocument();
    expect(screen.getByText("notes.pdf")).toBeInTheDocument();
  });

  it("downloads a file through the authenticated download", () => {
    const advisory = attachment("advisory.pdf", { downloadUrl: "https://example.test/a" });
    useGetCaseAttachmentsMock.mockReturnValue(loaded([advisory]));
    renderPanel();

    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    expect(downloadAttachmentMock).toHaveBeenCalledWith({
      id: "att-advisory.pdf",
      name: "advisory.pdf",
      type: "application/pdf",
      downloadUrl: "https://example.test/a",
    });
  });

  it("shows why a download failed", async () => {
    downloadAttachmentMock.mockRejectedValue(new Error("Download is not available"));
    useGetCaseAttachmentsMock.mockReturnValue(loaded([attachment("advisory.pdf")]));
    renderPanel();

    fireEvent.click(screen.getByRole("button", { name: /download/i }));

    await vi.waitFor(() => expect(showErrorMock).toHaveBeenCalledWith("Download is not available"));
  });

  it("offers nothing to change the files: customers cannot upload to or delete from an announcement", () => {
    useGetCaseAttachmentsMock.mockReturnValue(loaded([attachment("advisory.pdf")]));
    renderPanel();

    expect(screen.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /edit/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /upload/i })).not.toBeInTheDocument();
  });

  it("renders nothing for an announcement with no files, so it looks as it did before", () => {
    const { container } = renderPanel();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing while the files are loading", () => {
    useGetCaseAttachmentsMock.mockReturnValue({ ...loaded([]), data: undefined, isLoading: true });
    const { container } = renderPanel();
    expect(container).toBeEmptyDOMElement();
  });

  it("says the files could not be loaded, instead of reading as if there were none, and can retry", () => {
    useGetCaseAttachmentsMock.mockReturnValue({ ...loaded([]), data: undefined, isError: true });
    renderPanel();

    expect(screen.getByText("Attachments")).toBeInTheDocument();
    expect(screen.getByText("Could not load this announcement's attachments.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetchMock).toHaveBeenCalled();
  });

  it("offers to load more only when the server has more", () => {
    useGetCaseAttachmentsMock.mockReturnValue(loaded([attachment("a.pdf")]));
    const { unmount } = renderPanel();
    expect(screen.queryByRole("button", { name: "Show more" })).not.toBeInTheDocument();
    unmount();

    useGetCaseAttachmentsMock.mockReturnValue(loaded([attachment("a.pdf")], { hasNextPage: true }));
    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: "Show more" }));
    expect(fetchNextPageMock).toHaveBeenCalled();
  });
});
