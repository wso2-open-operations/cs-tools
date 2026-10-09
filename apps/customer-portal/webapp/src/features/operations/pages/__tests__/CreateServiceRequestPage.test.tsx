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
import { afterEach, describe, expect, it, vi } from "vitest";
import CreateServiceRequestPage from "@features/operations/pages/CreateServiceRequestPage";

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

// Mutable fixture state read by the mocks below, so a single test can drive
// the page into the "project has a chosen deployment and restricts SR product
// categories" scenario without re-declaring every vi.mock factory per test.
const testState = vi.hoisted(() => ({
  projectTypeLabel: undefined as string | undefined,
  srProductCategories: undefined as string[] | undefined,
  deployments: [] as Record<string, unknown>[],
  searchParams: new URLSearchParams(),
}));

const { productsSearchAllSpy, productsSearchInfiniteSpy } = vi.hoisted(() => ({
  productsSearchAllSpy: vi.fn(),
  productsSearchInfiniteSpy: vi.fn(),
}));

afterEach(() => {
  testState.projectTypeLabel = undefined;
  testState.srProductCategories = undefined;
  testState.deployments = [];
  testState.searchParams = new URLSearchParams();
  productsSearchAllSpy.mockClear();
  productsSearchInfiniteSpy.mockClear();
});

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    useParams: () => ({ projectId: "proj-1" }),
    useSearchParams: () => [testState.searchParams, vi.fn()],
    useLocation: () => ({ pathname: "/projects/proj-1/operations/service-requests/create", state: null }),
  };
});

vi.mock("@hooks/useModifierAwareNavigate", () => ({
  useModifierAwareNavigate: () => vi.fn(),
}));

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: {
      name: "Demo",
      account: { name: "Acct" },
      type: testState.projectTypeLabel
        ? { label: testState.projectTypeLabel }
        : undefined,
    },
    isLoading: false,
  }),
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
  usePostCase: () => ({ mutate: vi.fn(), mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@features/support/api/usePostAttachments", () => ({
  usePostAttachments: () => ({ mutate: vi.fn(), mutateAsync: vi.fn() }),
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
    data: {
      hasServiceRequestReadAccess: true,
      acceptedSeverityValues: [],
      srProductCategories: testState.srProductCategories,
    },
    isLoading: false,
  }),
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchInfinite: () => ({
    data: { pages: [{ deployments: testState.deployments }] },
    isLoading: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  usePostDeploymentProductsSearchInfinite: (
    id: string,
    opts: Record<string, unknown>,
  ) => {
    productsSearchInfiniteSpy(id, opts);
    return {
      data: { pages: [] },
      isLoading: false,
      isError: false,
      isFetchingNextPage: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    };
  },
  usePostDeploymentProductsSearchAll: (
    id: string,
    opts: Record<string, unknown>,
  ) => {
    productsSearchAllSpy(id, opts);
    return {
      data: [],
      isLoading: false,
      isError: false,
    };
  },
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

  it("fetches the full product list exhaustively, instead of lazily paging it, once a project restricts SR product categories", () => {
    // Regression guard: lazy, scroll-triggered pagination can strand eligible
    // products on a later page once the client-side category filter is
    // applied (a fetched page made entirely of NULL-category products
    // filters down to an empty, unscrollable menu while more pages remain).
    // A deployment selected via the ?deploymentId= prefill (no UI
    // interaction needed) must switch the page onto the exhaustive
    // usePostDeploymentProductsSearchAll hook and disable the lazy infinite
    // one, rather than feeding it a filter it can never fully page through.
    testState.projectTypeLabel = "Managed Cloud Subscription";
    testState.srProductCategories = ["ms", "pc"];
    testState.deployments = [
      {
        id: "d1",
        name: "Prod",
        description: null,
        url: null,
        project: { id: "proj-1", label: "Demo" },
        type: { id: "t1", label: "Primary Production" },
      },
    ];
    testState.searchParams = new URLSearchParams({ deploymentId: "d1" });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CreateServiceRequestPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(productsSearchAllSpy).toHaveBeenCalledWith(
      "d1",
      expect.objectContaining({
        enabled: true,
        request: { filters: { productCategories: ["ms", "pc"] } },
      }),
    );
    expect(productsSearchInfiniteSpy).toHaveBeenCalledWith(
      "d1",
      expect.objectContaining({ enabled: false }),
    );
  });

  it("uses the lazy infinite product list when the project has no category restriction", () => {
    testState.projectTypeLabel = "Managed Cloud Subscription";
    testState.srProductCategories = undefined;
    testState.deployments = [
      {
        id: "d1",
        name: "Prod",
        description: null,
        url: null,
        project: { id: "proj-1", label: "Demo" },
        type: { id: "t1", label: "Primary Production" },
      },
    ];
    testState.searchParams = new URLSearchParams({ deploymentId: "d1" });

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <CreateServiceRequestPage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(productsSearchInfiniteSpy).toHaveBeenCalledWith(
      "d1",
      expect.objectContaining({ enabled: true }),
    );
    expect(productsSearchAllSpy).toHaveBeenCalledWith(
      "d1",
      expect.objectContaining({ enabled: false }),
    );
  });
});
