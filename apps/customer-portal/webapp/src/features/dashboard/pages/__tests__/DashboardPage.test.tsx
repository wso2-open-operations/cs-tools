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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import DashboardPage from "@features/dashboard/pages/DashboardPage";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
    useLocation: () => ({ pathname: "/projects/proj-1/dashboard" }),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@hooks/useResponsiveLayout", () => ({
  useIsMidSizeTouchViewport: () => false,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isLoading: false, isSignedIn: true }),
}));

vi.mock("@api/useGetProjects", () => ({
  default: () => ({
    data: { pages: [{ projects: [{ id: "proj-1", type: { label: "Enterprise" } }] }] },
  }),
  flattenProjectPages: (data: { pages: { projects: unknown[] }[] } | undefined) =>
    data?.pages.flatMap((p) => p.projects) ?? [],
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { type: { label: "Enterprise" } }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: {
      hasServiceRequestReadAccess: true,
      hasChangeRequestReadAccess: true,
      hasEngagementsReadAccess: true,
      acceptedSeverityValues: [],
    },
    isLoading: false,
  }),
}));

vi.mock("@features/dashboard/api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: () => ({
    data: {
      totalCases: 10,
      openCases: 5,
      resolvedCases: 3,
      avgResponseTime: "2h",
      casesBySeverity: [],
      casesByType: [],
    },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/dashboard/api/useGetProjectChangeRequestsStats", () => ({
  useGetProjectChangeRequestsStats: () => ({
    data: { totalCount: 2, stateCount: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@components/stat-grid/SupportStatGrid", () => ({
  default: () => <div data-testid="support-stat-grid" />,
}));

vi.mock("@features/dashboard/components/charts/ChartLayout", () => ({
  default: () => <div data-testid="chart-layout" />,
}));

vi.mock("@features/dashboard/components/cases-table/CasesTable", () => ({
  default: () => <div data-testid="cases-table" />,
}));

describe("DashboardPage", () => {
  it("renders dashboard stat grid, charts, and cases table", () => {
    render(
      <MemoryRouter>
        <DashboardPage />
      </MemoryRouter>,
    );
    expect(screen.getByTestId("support-stat-grid")).toBeInTheDocument();
    expect(screen.getByTestId("chart-layout")).toBeInTheDocument();
    expect(screen.getByTestId("cases-table")).toBeInTheDocument();
  });
});
