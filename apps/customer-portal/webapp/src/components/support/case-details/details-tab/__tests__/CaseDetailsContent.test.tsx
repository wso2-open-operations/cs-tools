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

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CaseDetailsContent from "@case-details-details/CaseDetailsContent";

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
  parentCase: null,
  conversation: null,
  issueType: null,
  severity: { id: "60", label: "S0" },
  status: { id: "1", label: "Open" },
  closedOn: null,
  closedBy: null,
  closeNotes: null,
  hasAutoClosed: null,
  engineerEmail: null,
  findingsResolved: null,
  findingsTotal: null,
};

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: { caseStates: [{ id: "3", label: "Closed" }] },
  }),
}));

vi.mock("@api/usePatchCase", () => ({
  usePatchCase: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@api/useGetCaseAttachments", () => ({
  useGetCaseAttachments: vi.fn(() => ({
    data: {
      pages: [{ totalRecords: 3, attachments: [], limit: 10, offset: 0 }],
      pageParams: [0],
    },
    isLoading: false,
    isError: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    fetchNextPage: vi.fn(),
  })),
}));

vi.mock("@api/useGetCallRequests", () => ({
  useGetCallRequests: vi.fn(() => ({
    data: { pages: [{ totalRecords: 2, callRequests: [] }] },
  })),
}));

vi.mock("@case-details/CaseDetailsTabPanels", () => ({
  __esModule: true,
  default: () => <div data-testid="tab-panels">Tab panels</div>,
}));

function renderContent(
  props: {
    data?: typeof mockCaseDetails;
    isLoading?: boolean;
    isError?: boolean;
    caseId?: string;
    onBack?: () => void;
  } = {},
) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsContent
        data={props.data ?? mockCaseDetails}
        isLoading={props.isLoading ?? false}
        isError={props.isError ?? false}
        caseId={props.caseId ?? "case-001"}
        onBack={props.onBack ?? vi.fn()}
      />
    </ThemeProvider>,
  );
}

describe("CaseDetailsContent", () => {
  it("should show loading skeleton when isLoading is true", () => {
    renderContent({ data: undefined, isLoading: true });
    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
    const skeletons = document.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render header and action row when not in focus mode", () => {
    renderContent();
    expect(screen.getByText("CS0001001")).toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
  });

  it("should hide header and action row when focus mode is on", () => {
    renderContent();
    const focusButton = screen.getByRole("button", { name: /focus mode/i });
    fireEvent.click(focusButton);
    expect(screen.queryByText("Manage case status")).not.toBeInTheDocument();
  });

  it("should show attachment count in Attachments tab label when available", () => {
    renderContent();
    expect(screen.getByText("Attachments (3)")).toBeInTheDocument();
  });

  it("should render tab panels", () => {
    renderContent();
    expect(screen.getByTestId("tab-panels")).toBeInTheDocument();
  });
});
