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
import ServiceRequestsPage from "@features/operations/pages/ServiceRequestsPage";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
    useLocation: () => ({ pathname: "/projects/proj-1/operations/service-requests", state: null }),
    useSearchParams: () => [new URLSearchParams(), vi.fn()],
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), debug: vi.fn() }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: { pages: [] },
    isLoading: false,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
  }),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { name: "Demo" }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({ data: { severities: [], statuses: [] }, isLoading: false }),
}));

vi.mock("@api/useGetProjectCases", () => ({
  default: () => ({
    data: { pages: [{ cases: [], totalRecords: 0 }] },
    isLoading: false,
    isError: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
  }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: {
      hasServiceRequestReadAccess: true,
      acceptedSeverityValues: [],
    },
    isLoading: false,
  }),
}));

vi.mock("@hooks/useSessionState", () => ({
  useSessionState: (key: string, initial: unknown) => {
    if (key.includes("search")) return ["", vi.fn()];
    if (key.includes("page")) return [1, vi.fn()];
    if (key.includes("rowsPerPage")) return [10, vi.fn()];
    if (key.includes("sortField")) return ["updatedOn", vi.fn()];
    if (key.includes("sortOrder")) return ["desc", vi.fn()];
    if (key.includes("filters")) return [{}, vi.fn()];
    return [initial, vi.fn()];
  },
}));

describe("ServiceRequestsPage", () => {
  it("renders service requests page header", () => {
    render(
      <MemoryRouter>
        <ServiceRequestsPage />
      </MemoryRouter>,
    );
    expect(screen.getByText("All Service Requests")).toBeInTheDocument();
  });
});
