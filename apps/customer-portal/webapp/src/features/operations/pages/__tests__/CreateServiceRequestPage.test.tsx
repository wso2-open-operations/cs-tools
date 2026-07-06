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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import CreateServiceRequestPage from "@features/operations/pages/CreateServiceRequestPage";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
    useSearchParams: () => [new URLSearchParams(), vi.fn()],
    useLocation: () => ({ pathname: "/projects/proj-1/operations/service-requests/create", state: null }),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({ data: { name: "Demo", account: { name: "Acct" } }, isLoading: false }),
}));

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: { deployments: [{ id: "d1", label: "Prod" }], products: [] },
    isLoading: false,
  }),
}));

vi.mock("@features/operations/api/useSearchCatalogs", () => ({
  useSearchCatalogs: () => ({ data: { catalogs: [] }, isLoading: false }),
}));

vi.mock("@features/operations/api/useGetCatalogItemVariables", () => ({
  useGetCatalogItemVariables: () => ({ data: undefined, isLoading: false }),
}));

vi.mock("@features/operations/api/usePostCase", () => ({
  usePostCase: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ error: vi.fn(), debug: vi.fn() }),
}));

vi.mock("@api/useGetProjectFeatures", () => ({
  default: () => ({
    data: { hasServiceRequestReadAccess: true, acceptedSeverityValues: [] },
    isLoading: false,
  }),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: { pages: [] },
    isLoading: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  usePostDeploymentProductsSearchInfinite: () => ({
    data: { pages: [] },
    isLoading: false,
    isError: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
  extractDeploymentProducts: () => [],
}));

vi.mock("@features/settings/api/useGetUserDetails", () => ({
  default: () => ({ data: { email: "user@test.dev" }, isLoading: false }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => vi.fn(),
}));

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useQueryClient: () => ({ invalidateQueries: vi.fn() }),
  };
});

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

describe("CreateServiceRequestPage", () => {
  it("renders create service request heading", () => {
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CreateServiceRequestPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByText(/Create Service Request/i)).toBeInTheDocument();
  });
});
