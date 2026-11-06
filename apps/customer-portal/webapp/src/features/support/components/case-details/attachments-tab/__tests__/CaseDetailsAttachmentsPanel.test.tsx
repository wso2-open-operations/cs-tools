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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CaseDetailsAttachmentsPanel from "@case-details-attachments/CaseDetailsAttachmentsPanel";
import type { CaseAttachment } from "@features/support/types/cases";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import { SuccessBannerProvider } from "@context/success-banner/SuccessBannerContext";
import LoggerProvider from "@context/logger/LoggerProvider";

const mockCaseAttachments: CaseAttachment[] = [
  {
    id: "a1",
    name: "screenshot-error.png",
    type: "image/png",
    size: 240 * 1024,
    downloadUrl: "/a1",
    createdOn: "2026-02-01",
    createdBy: "admin@test.com",
  },
  {
    id: "a2",
    name: "logs-debug.txt",
    type: "text/plain",
    size: 1024,
    downloadUrl: "/a2",
    createdOn: "2026-02-01",
    createdBy: "admin@test.com",
  },
  {
    id: "a3",
    name: "config-backup.zip",
    type: "application/zip",
    size: 512,
    downloadUrl: "/a3",
    createdOn: "2026-02-01",
    createdBy: "admin@test.com",
  },
];

vi.mock("../UploadAttachmentModal", () => ({
  __esModule: true,
  default: ({ onSuccess }: { onSuccess?: () => void }) => (
    <button type="button" onClick={() => onSuccess?.()}>
      simulate upload success
    </button>
  ),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

vi.mock("@features/support/api/useGetCaseAttachments", () => ({
  useGetCaseAttachments: vi.fn(() => ({
    data: {
      pages: [
        {
          attachments: mockCaseAttachments,
          totalRecords: mockCaseAttachments.length,
          limit: 10,
          offset: 0,
        },
      ],
      pageParams: [0],
    },
    isLoading: false,
    isError: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchNextPageError: false,
  })),
  flattenCaseAttachments: (data: any) =>
    data?.pages?.flatMap((p: any) => p.attachments ?? []) ?? [],
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  __esModule: true,
  default: () => ({
    data: { email: "admin@test.com" },
    isLoading: false,
    isError: false,
  }),
}));

function renderPanel(caseId = "case-1") {
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <LoggerProvider config={{ level: "ERROR", prefix: "Test" }}>
          <ErrorBannerProvider>
            <SuccessBannerProvider>
              <CaseDetailsAttachmentsPanel caseId={caseId} />
            </SuccessBannerProvider>
          </ErrorBannerProvider>
        </LoggerProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

// The panel shows a skeleton for at least this long even when data is ready
// (SKELETON_MIN_DISPLAY_MS in the component).
const SKELETON_MIN_DISPLAY_MS = 4000;

/** Renders the panel and lets the minimum skeleton display time elapse. */
function renderPanelPastSkeleton(caseId = "case-1") {
  vi.useFakeTimers();
  const result = renderPanel(caseId);
  act(() => {
    vi.advanceTimersByTime(SKELETON_MIN_DISPLAY_MS);
  });
  return result;
}

describe("CaseDetailsAttachmentsPanel", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("should not reload the page after a successful upload", () => {
    const reload = vi.fn();
    vi.stubGlobal("location", { ...window.location, reload });
    renderPanel();
    fireEvent.click(
      screen.getByRole("button", { name: /simulate upload success/i }),
    );
    expect(reload).not.toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: /upload attachment/i }),
    ).toBeInTheDocument();
  });

  it("should render Upload Attachment button", () => {
    renderPanel();
    expect(
      screen.getByRole("button", { name: /upload attachment/i }),
    ).toBeInTheDocument();
  });

  it("should render attachment list from API with names and download buttons", () => {
    renderPanelPastSkeleton();
    expect(screen.getByText("screenshot-error.png")).toBeInTheDocument();
    expect(screen.getByText("logs-debug.txt")).toBeInTheDocument();
    expect(screen.getByText("config-backup.zip")).toBeInTheDocument();
    const downloadButtons = screen.getAllByRole("button", {
      name: /download/i,
    });
    expect(downloadButtons).toHaveLength(mockCaseAttachments.length);
  });

  it("should show uploaded-by and size for attachments", () => {
    renderPanelPastSkeleton();
    expect(screen.getByText(/240 KB/)).toBeInTheDocument();
    expect(
      screen.getAllByText(/Uploaded by admin@test.com/).length,
    ).toBeGreaterThan(0);
  });

  it("should show no case selected when caseId is empty", () => {
    renderPanel("");
    expect(screen.getByText(/no case selected/i)).toBeInTheDocument();
  });

  it("should show EmptyIcon and no attachments found when attachments list is empty", async () => {
    const { useGetCaseAttachments } =
      await import("@features/support/api/useGetCaseAttachments");
    // mockReturnValue (not Once): the panel re-renders when the skeleton timer
    // fires. This is the last test in the file, so the override can persist.
    vi.mocked(useGetCaseAttachments).mockReturnValue({
      data: {
        pages: [
          {
            attachments: [],
            totalRecords: 0,
            limit: 10,
            offset: 0,
          },
        ],
        pageParams: [0],
      },
      isLoading: false,
      isError: false,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchNextPageError: false,
    } as any);
    renderPanelPastSkeleton("case-1");
    expect(screen.getByText("No attachments found.")).toBeInTheDocument();
    expect(
      document.querySelector("svg[aria-hidden='true']"),
    ).toBeInTheDocument();
  });
});
