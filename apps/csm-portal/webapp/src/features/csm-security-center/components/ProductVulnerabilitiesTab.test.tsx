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

import { fireEvent, render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// QueryErrorState imports BackendApiError from @api/backend/client, which
// reads window.config at module load -- not present under Vitest (same
// convention as every other test that transitively reaches this module; see
// this app's own CLAUDE.md "Testing" section).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: vi.fn(), post: vi.fn() }),
  BackendApiError: class BackendApiError extends Error {},
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

import ProductVulnerabilitiesTab from "./ProductVulnerabilitiesTab";

const searchResult = {
  data: { productVulnerabilities: [], total: 0, limit: 20, offset: 0 },
  isLoading: false,
  isError: false,
  error: null,
  isFetching: false,
  refetch: vi.fn(),
  dataUpdatedAt: 0,
};

const searchMock = vi.fn<(payload: unknown) => typeof searchResult>(() => searchResult);
vi.mock(
  "@features/csm-security-center/api/useSearchProductVulnerabilities",
  () => ({
    useSearchProductVulnerabilities: (payload: unknown) => searchMock(payload),
  }),
);

const metaMock = vi.fn();
vi.mock("@features/csm-security-center/api/useGetVulnerabilityMeta", () => ({
  useGetVulnerabilityMeta: () => metaMock(),
}));

function renderTab(): ReturnType<typeof render> {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ProductVulnerabilitiesTab />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ProductVulnerabilitiesTab — Priority filter", () => {
  it("renders the fetched severity labels, not the static display map, when meta has loaded", () => {
    metaMock.mockReturnValue({
      data: {
        severities: [
          { id: "4", label: "Catastrophic" },
          { id: "3", label: "High" },
        ],
      },
      isLoading: false,
    });
    renderTab();

    fireEvent.mouseDown(screen.getByLabelText(/filter by priority/i));
    const listbox = screen.getByRole("listbox");
    expect(within(listbox).getByText("Catastrophic")).toBeInTheDocument();
    expect(within(listbox).getByText("High")).toBeInTheDocument();
    // Medium/Low/Info/Unknown were never returned by meta, so they must not render.
    expect(within(listbox).queryByText("Medium")).not.toBeInTheDocument();
  });

  it("falls back to the static priority list while meta is loading", () => {
    metaMock.mockReturnValue({ data: undefined, isLoading: true });
    renderTab();

    fireEvent.mouseDown(screen.getByLabelText(/filter by priority/i));
    const listbox = screen.getByRole("listbox");
    expect(within(listbox).getByText("Critical")).toBeInTheDocument();
    expect(within(listbox).getByText("Unknown")).toBeInTheDocument();
  });
});

describe("ProductVulnerabilitiesTab — Product and Version filters", () => {
  const meta = {
    data: {
      severities: [{ id: "4", label: "Critical" }],
      products: [
        { name: "alpha", versions: ["2.0.0", "1.0.0"] },
        { name: "beta", versions: [] },
      ],
    },
    isLoading: false,
  };

  const lastPayload = (): { filters: Record<string, unknown>; pagination: Record<string, number> } =>
    searchMock.mock.calls[searchMock.mock.calls.length - 1][0] as never;

  const choose = (ariaLabel: RegExp, option: string): void => {
    fireEvent.mouseDown(screen.getByLabelText(ariaLabel));
    fireEvent.click(within(screen.getByRole("listbox")).getByText(option));
  };

  beforeEach(() => {
    searchMock.mockClear();
    searchResult.data = { productVulnerabilities: [], total: 0, limit: 20, offset: 0 };
    searchResult.isError = false;
    metaMock.mockReturnValue(meta);
  });

  it("sends productName and resets to page 0 when a product is selected", () => {
    renderTab();
    expect(lastPayload().filters).not.toHaveProperty("productName");

    choose(/filter by product$/i, "alpha");
    expect(lastPayload().filters).toEqual({ productName: "alpha" });
    expect(lastPayload().pagination.offset).toBe(0);
  });

  it("offers the selected product's versions and clears the version on product change", () => {
    metaMock.mockReturnValue({
      ...meta,
      data: {
        ...meta.data,
        products: [
          { name: "alpha", versions: ["2.0.0", "1.0.0"] },
          { name: "gamma", versions: ["9.9.9"] },
        ],
      },
    });
    renderTab();
    expect(screen.getByLabelText(/filter by product version/i)).toHaveAttribute("aria-disabled", "true");

    choose(/filter by product$/i, "alpha");
    fireEvent.mouseDown(screen.getByLabelText(/filter by product version/i));
    const listbox = screen.getByRole("listbox");
    expect(within(listbox).getByText("All versions")).toBeInTheDocument();
    expect(within(listbox).getByText("2.0.0")).toBeInTheDocument();
    expect(within(listbox).queryByText("9.9.9")).not.toBeInTheDocument();
    fireEvent.click(within(listbox).getByText("1.0.0"));
    expect(lastPayload().filters).toEqual({ productName: "alpha", productVersion: "1.0.0" });

    choose(/filter by product$/i, "gamma");
    expect(lastPayload().filters).toEqual({ productName: "gamma" });
  });

  it("keeps the Version control disabled, with helper text, for a product without versions", () => {
    renderTab();
    choose(/filter by product$/i, "beta");
    expect(screen.getByLabelText(/filter by product version/i)).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText("No versions available")).toBeInTheDocument();
    expect(lastPayload().filters).toEqual({ productName: "beta" });
  });

  it("sends productVersion when a version is selected", () => {
    renderTab();
    choose(/filter by product$/i, "alpha");
    choose(/filter by product version/i, "2.0.0");
    expect(lastPayload().filters).toEqual({ productName: "alpha", productVersion: "2.0.0" });
    expect(lastPayload().pagination.offset).toBe(0);
  });

  it("still renders table rows, with the new dropdowns disabled, when meta fails", () => {
    metaMock.mockReturnValue({ data: undefined, isLoading: false, isError: true });
    searchResult.data = {
      productVulnerabilities: [
        { id: "v1", cveId: "CVE-0000-0001", productName: "alpha", productVersion: "1.0.0" },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    } as never;
    renderTab();

    expect(screen.getByText("CVE-0000-0001")).toBeInTheDocument();
    expect(screen.getByLabelText(/filter by product$/i)).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByLabelText(/filter by product version/i)).toHaveAttribute("aria-disabled", "true");
  });
});
