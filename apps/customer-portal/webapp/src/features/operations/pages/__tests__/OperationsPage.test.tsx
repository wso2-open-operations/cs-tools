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
import OperationsPage from "@features/operations/pages/OperationsPage";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { name: "Demo Project" }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: {
      hasServiceRequestReadAccess: true,
      hasChangeRequestReadAccess: true,
      acceptedSeverityValues: [],
    },
    isLoading: false,
  }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@api/useGetProjectCasesPage", () => ({
  useGetProjectCasesPage: () => ({ data: { cases: [] }, isLoading: false }),
}));

vi.mock("@features/operations/api/useGetChangeRequests", () => ({
  default: () => ({ data: { changeRequests: [] }, isLoading: false }),
  useGetChangeRequestsInfinite: () => ({ data: { pages: [] }, isLoading: false }),
}));

vi.mock("@features/dashboard/api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@features/dashboard/api/useGetProjectChangeRequestsStats", () => ({
  useGetProjectChangeRequestsStats: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@utils/permission", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@utils/permission")>();
  return {
    ...actual,
    isProjectRestricted: () => false,
  };
});

describe("OperationsPage", () => {
  it("renders operations overview stat grid", () => {
    render(
      <MemoryRouter>
        <OperationsPage />
      </MemoryRouter>,
    );
    expect(screen.getByText("Outstanding service requests")).toBeInTheDocument();
  });
});
