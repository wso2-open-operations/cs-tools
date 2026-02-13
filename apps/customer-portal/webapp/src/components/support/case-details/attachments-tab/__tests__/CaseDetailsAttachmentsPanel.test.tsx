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
import CaseDetailsAttachmentsPanel from "@case-details-attachments/CaseDetailsAttachmentsPanel";
import { mockCaseAttachments } from "@models/mockData";
import useGetCaseAttachments from "@api/useGetCaseAttachments";

vi.mock("@case-details-attachments/UploadAttachmentModal", () => ({
  __esModule: true,
  default: () => null,
}));

vi.mock("@api/useGetCaseAttachments", () => ({
  __esModule: true,
  default: vi.fn((_caseId: string, opts?: { enabled?: boolean }) => {
    const enabled = opts?.enabled !== false;
    return {
      data: enabled
        ? {
            attachments: mockCaseAttachments,
            totalRecords: mockCaseAttachments.length,
            limit: 50,
            offset: 0,
          }
        : undefined,
      isLoading: false,
      isError: false,
    };
  }),
}));

function renderPanel(caseId = "case-1") {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsAttachmentsPanel caseId={caseId} />
    </ThemeProvider>,
  );
}

describe("CaseDetailsAttachmentsPanel", () => {
  it("should render Upload Attachment button", () => {
    renderPanel();
    expect(
      screen.getByRole("button", { name: /upload attachment/i }),
    ).toBeInTheDocument();
  });

  it("should render attachment list from API with names and download buttons", () => {
    renderPanel();
    expect(screen.getByText("screenshot-error.png")).toBeInTheDocument();
    expect(screen.getByText("logs-debug.txt")).toBeInTheDocument();
    expect(screen.getByText("config-backup.zip")).toBeInTheDocument();
    const downloadButtons = screen.getAllByRole("button", { name: /download/i });
    expect(downloadButtons).toHaveLength(mockCaseAttachments.length);
  });

  it("should show uploaded-by and size for attachments", () => {
    renderPanel();
    expect(screen.getByText(/240 KB/)).toBeInTheDocument();
    expect(
      screen.getAllByText(/Uploaded by para-admin@wso2.com/).length,
    ).toBeGreaterThan(0);
  });

  it("should show no case selected when caseId is empty", () => {
    renderPanel("");
    expect(screen.getByText(/no case selected/i)).toBeInTheDocument();
  });

  it("should show EmptyIcon and no attachments found when attachments list is empty", () => {
    vi.mocked(useGetCaseAttachments).mockReturnValueOnce({
      data: {
        attachments: [],
        totalRecords: 0,
        limit: 50,
        offset: 0,
      },
      error: null,
      isLoading: false,
      isError: false,
      isPending: false,
      isSuccess: true,
      status: "success",
      fetchStatus: "idle",
      dataUpdatedAt: 0,
      errorUpdatedAt: 0,
      failureCount: 0,
      failureReason: null,
      errorUpdateCount: 0,
      isFetched: true,
      isFetchedAfterMount: true,
      isFetching: false,
      isRefetching: false,
      isLoadingError: false,
      isPaused: false,
      isPlaceholderData: false,
      isRefetchError: false,
      isStale: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useGetCaseAttachments>);
    renderPanel("case-1");
    expect(screen.getByText("No attachments found.")).toBeInTheDocument();
    expect(document.querySelector("svg[aria-hidden='true']")).toBeInTheDocument();
  });
});
