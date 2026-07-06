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
import DashboardItemsPage from "@features/dashboard/pages/DashboardItemsPage";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
    useNavigate: () => vi.fn(),
    useLocation: () => ({ pathname: "/projects/proj-1/dashboard/items", state: null }),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { type: { label: "Enterprise" } }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: {
      hasServiceRequestReadAccess: false,
      hasChangeRequestReadAccess: false,
      hasSraReadAccess: false,
      acceptedSeverityValues: [],
    },
    isLoading: false,
  }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: { caseStates: [], severities: [] },
    isLoading: false,
  }),
}));

vi.mock("@api/useGetProjectCasesPage", () => ({
  useGetProjectCasesPage: () => ({
    data: { cases: [], totalRecords: 0 },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/operations/api/useGetChangeRequests", () => ({
  default: () => ({
    data: { changeRequests: [], totalRecords: 0 },
    isLoading: false,
    isError: false,
  }),
}));

describe("DashboardItemsPage", () => {
  it("renders action required items heading", () => {
    render(
      <MemoryRouter>
        <DashboardItemsPage mode="action-required" />
      </MemoryRouter>,
    );
    expect(screen.getByText("Action Required Items")).toBeInTheDocument();
  });

  it("renders outstanding items heading", () => {
    render(
      <MemoryRouter>
        <DashboardItemsPage mode="outstanding-interactions" />
      </MemoryRouter>,
    );
    expect(screen.getByText("Outstanding Items")).toBeInTheDocument();
  });
});
