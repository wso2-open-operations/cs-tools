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
import { beforeEach, describe, expect, it, vi } from "vitest";
import AllCasesPage from "@features/support/pages/AllCasesPage";

const mockNavigate = vi.fn();
const mockSetSearchParams = vi.fn();
const mockShowLoader = vi.fn();
const mockHideLoader = vi.fn();

const mockUseParams = vi.fn();
const mockUseLocation = vi.fn();
const mockUseSearchParams = vi.fn();
const mockUseSessionState = vi.fn();
const mockUseGetProjectDetails = vi.fn();
const mockUseGetProjectFeatures = vi.fn();
const mockUseGetProjectFilters = vi.fn();
const mockUseGetProjectCases = vi.fn();
const mockUsePostProjectDeploymentsSearchInfinite = vi.fn();

vi.mock("react-router", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useParams: () => mockUseParams(),
    useLocation: () => mockUseLocation(),
    useSearchParams: () => mockUseSearchParams(),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => mockNavigate,
}));

vi.mock("@hooks/useSessionState", () => ({
  useSessionState: (...args: unknown[]) => mockUseSessionState(...args),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: mockShowLoader, hideLoader: mockHideLoader }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => mockUseGetProjectDetails(),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => mockUseGetProjectFeatures(),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => mockUseGetProjectFilters(),
}));

vi.mock("@api/useGetProjectCases", () => ({
  default: () => mockUseGetProjectCases(),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () =>
    mockUsePostProjectDeploymentsSearchInfinite(),
}));

vi.mock("@utils/permission", () => ({
  getProjectPermissions: () => ({ hasDeployments: true }),
  getProjectSeverityPolicy: () => ({
    excludeS0: false,
    restrictSeverityToLow: false,
  }),
}));

vi.mock("@components/list-view/ListPageHeader", () => ({
  default: ({
    title,
    onBack,
  }: {
    title: string;
    onBack: () => void;
  }) => (
    <div>
      <h1>{title}</h1>
      <button onClick={onBack}>Back</button>
    </div>
  ),
}));

vi.mock("@components/list-view/ListSearchPanel", () => ({
  default: () => <div data-testid="search-panel">SearchPanel</div>,
}));

vi.mock("@components/list-view/ListItems", () => ({
  default: ({
    onCaseClick,
  }: {
    onCaseClick: (value: { id: string }) => void;
  }) => <button onClick={() => onCaseClick({ id: "case-1" })}>Open Case Row</button>,
}));

vi.mock("@components/list-view/ListResultsBar", () => ({
  default: () => <div>ResultsBar</div>,
}));

vi.mock("@components/list-view/ListPagination", () => ({
  default: () => <div>Pagination</div>,
}));

vi.mock("@features/support/components/all-cases/AllCasesCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));

describe("AllCasesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseParams.mockReturnValue({ projectId: "project-1" });
    mockUseLocation.mockReturnValue({ state: { returnTo: "/projects/project-1/support" } });
    mockUseSearchParams.mockReturnValue([new URLSearchParams(), mockSetSearchParams]);
    mockUseGetProjectDetails.mockReturnValue({
      data: { type: { label: "Enterprise" }, name: "P1" },
      isLoading: false,
    });
    mockUseGetProjectFeatures.mockReturnValue({ data: {} });
    mockUseGetProjectFilters.mockReturnValue({
      data: { caseStates: [] },
    });
    mockUseGetProjectCases.mockReturnValue({
      data: { pages: [{ cases: [{ id: "case-1" }], totalRecords: 1 }] },
      isLoading: false,
      isError: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    });
    mockUsePostProjectDeploymentsSearchInfinite.mockReturnValue({
      data: { pages: [] },
      isFetchingNextPage: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    });

    mockUseSessionState.mockImplementation(
      (_key: string, initialValue: unknown) => [initialValue, vi.fn()],
    );
  });

  it("should hide search panel for status filter branch", () => {
    mockUseSearchParams.mockReturnValue([
      new URLSearchParams("statusFilter=active"),
      mockSetSearchParams,
    ]);

    render(<AllCasesPage />);

    expect(screen.queryByTestId("search-panel")).not.toBeInTheDocument();
    expect(screen.getByText("Outstanding Cases")).toBeInTheDocument();
  });

  it("should navigate to case details and back to returnTo", () => {
    render(<AllCasesPage />);

    fireEvent.click(screen.getByText("Open Case Row"));
    fireEvent.click(screen.getByText("Back"));

    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/support/cases/case-1");
    expect(mockNavigate).toHaveBeenCalledWith("/projects/project-1/support");
  });
});

