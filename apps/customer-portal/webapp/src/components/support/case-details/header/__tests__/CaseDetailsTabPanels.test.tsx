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
import CaseDetailsTabPanels from "@case-details/CaseDetailsTabPanels";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import {
  mockCaseAttachments,
  mockCaseComments,
  mockCaseDetails,
  mockUserDetails,
} from "@models/mockData";

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

vi.mock("@api/useGetCaseComments", () => ({
  __esModule: true,
  default: vi.fn(() => ({
    data: {
      comments: mockCaseComments,
      totalRecords: mockCaseComments.length,
      offset: 0,
      limit: 50,
    },
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@api/useGetUserDetails", () => ({
  __esModule: true,
  default: vi.fn(() => ({
    data: mockUserDetails,
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@api/usePostComment", () => ({
  usePostComment: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}));

function renderTabPanels(
  activeTab: number,
  caseId = "case-1",
  options?: { data?: typeof mockCaseDetails; isError?: boolean },
) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsTabPanels
        activeTab={activeTab}
        caseId={caseId}
        data={options?.data ?? mockCaseDetails}
        isError={options?.isError ?? false}
      />
    </ThemeProvider>,
  );
}

describe("CaseDetailsTabPanels", () => {
  it("should show Activity panel with chat messages when activeTab is 0", () => {
    renderTabPanels(0);
    expect(
      screen.getByText(/Thanks for the detailed recommendations/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Show more/)).toBeInTheDocument();
    expect(screen.getAllByText(/Support Engineer/).length).toBeGreaterThan(0);
  });

  it("should show Activity placeholder when projectId is missing", () => {
    renderTabPanels(0, "case-1", {
      data: { ...mockCaseDetails, project: null },
    });
    expect(
      screen.getByText("Activity timeline will appear here."),
    ).toBeInTheDocument();
  });

  it("should show Details panel with case overview and cards when activeTab is 1", () => {
    renderTabPanels(1);
    expect(screen.getByText("CS0001001")).toBeInTheDocument();
    expect(screen.getByText("Product & Environment")).toBeInTheDocument();
    expect(screen.getByText("Customer Information")).toBeInTheDocument();
  });

  it("should show Attachments panel with list and download when activeTab is 2", () => {
    renderTabPanels(2);
    expect(screen.getByRole("button", { name: /upload attachment/i })).toBeInTheDocument();
    expect(screen.getByText("screenshot-error.png")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /download/i }).length).toBeGreaterThan(0);
  });

  it("should show Calls placeholder when activeTab is 3", () => {
    renderTabPanels(3);
    expect(screen.getByText("Calls will appear here.")).toBeInTheDocument();
  });

  it("should show Knowledge Base placeholder when activeTab is 4", () => {
    renderTabPanels(4);
    expect(
      screen.getByText("Knowledge Base articles will appear here."),
    ).toBeInTheDocument();
  });
});
