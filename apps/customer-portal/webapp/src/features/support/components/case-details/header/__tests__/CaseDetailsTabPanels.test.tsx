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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import CaseDetailsTabPanels from "@case-details/CaseDetailsTabPanels";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { ErrorBannerProvider } from "@context/error-banner/ErrorBannerContext";
import LoggerProvider from "@context/logger/LoggerProvider";
import type { CaseDetails } from "@features/support/types/cases";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

const mockCaseDetails = {
  id: "case-001",
  internalId: "INT-1",
  number: "CS0001001",
  createdOn: "2026-01-31 10:45:12",
  updatedOn: "2026-02-10 23:47:57",
  title: "Test case",
  description: "Desc",
  slaResponseTime: "129671000",
  product: null,
  account: { type: null, id: "acc-1", label: "Account" },
  csManager: null,
  assignedEngineer: null,
  project: { id: "p1", label: "Project" },
  type: { id: "1", label: "Incident" },
  deployment: { id: "d1", label: "Production" },
  deployedProduct: null,
  relatedCase: null,
  conversation: null,
  issueType: null,
  status: { id: "1", label: "Open" },
  severity: { id: "60", label: "S0" },
  closedOn: null,
  closedBy: null,
  closeNotes: null,
  hasAutoClosed: null,
};

const mockCaseComments = [
  {
    id: "c1",
    content: "Thanks for the detailed recommendations. I'll review.",
    type: "comments",
    createdOn: "2026-02-12T11:15:42",
    createdBy: "user@test.com",
    isEscalated: false,
  },
  {
    id: "c2",
    content: "Show more content here.",
    type: "comments",
    createdOn: "2026-02-12T10:30:15",
    createdBy: "support@wso2.com",
    isEscalated: false,
  },
];

const mockCaseAttachments = [
  {
    id: "a1",
    name: "file.txt",
    type: "text/plain",
    downloadUrl: "/file",
    createdOn: "2026-02-01",
    createdBy: "user@test.com",
  },
];

const mockUserDetails = {
  id: "u1",
  email: "user@test.com",
  lastName: "User",
  firstName: "Test",
  timeZone: "UTC",
};

vi.mock("@case-details-attachments/UploadAttachmentModal", () => ({
  __esModule: true,
  default: () => null,
}));

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

vi.mock("@features/support/api/useGetAIChatHistory", () => ({
  __esModule: true,
  default: vi.fn(() => ({
    comments: mockCaseComments,
    isLoading: false,
    isError: false,
    error: null,
  })),
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  __esModule: true,
  default: vi.fn(() => ({
    data: mockUserDetails,
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@features/support/api/usePostComment", () => ({
  usePostComment: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
  })),
}));

vi.mock("@features/support/api/useConversationRecommendationsSearch", () => ({
  useConversationRecommendationsSearch: vi.fn(() => ({
    data: { query: "", recommendations: [] },
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: vi.fn(() => ({
    isSignedIn: true,
    isLoading: false,
  })),
}));

function renderTabPanels(
  panelIndex: number,
  caseId = "case-1",
  options?: { data?: CaseDetails; isError?: boolean },
) {
  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <LoggerProvider>
          <ErrorBannerProvider>
            <CaseDetailsTabPanels
              panelIndex={panelIndex}
              caseId={caseId}
              data={options?.data ?? (mockCaseDetails as CaseDetails)}
              isError={options?.isError ?? false}
            />
          </ErrorBannerProvider>
        </LoggerProvider>
      </ThemeProvider>
    </QueryClientProvider>,
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
      data: { ...mockCaseDetails, project: null } as CaseDetails,
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
    expect(
      screen.getByRole("button", { name: /upload attachment/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("file.txt")).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: /download/i }).length,
    ).toBeGreaterThan(0);
  });

  it("should show Calls placeholder when activeTab is 3 and project is missing", () => {
    renderTabPanels(3, "case-1", {
      data: { ...mockCaseDetails, project: null } as CaseDetails,
    });
    expect(
      screen.getByText("Call requests will appear here."),
    ).toBeInTheDocument();
  });

  it("should show Knowledge Base recommendations panel when activeTab is 4", () => {
    renderTabPanels(4);
    expect(
      screen.getByText(/No matching knowledge base articles were found for this case/i),
    ).toBeInTheDocument();
  });
});
