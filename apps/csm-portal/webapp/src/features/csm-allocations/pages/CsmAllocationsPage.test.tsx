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

import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const api = { post: vi.fn(), get: vi.fn() };
vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => api,
}));

// The engineer filter must be the internal-only search. Providing only that
// export makes the page fail loudly if it ever reaches for the unscoped one.
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: () => ({
    data: [{ id: "00000000-0000-0000-0000-0000000000b1", firstName: "Jane", lastName: "Doe" }],
    isFetching: false,
    isError: false,
  }),
}));
vi.mock("@features/csm-accounts/api/useSearchAccounts", () => ({
  useInfiniteAccountSearch: () => ({
    accounts: [{ id: "00000000-0000-0000-0000-0000000000c1", name: "Example Corp" }],
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  }),
}));

import CsmAllocationsPage from "./CsmAllocationsPage";
import { ALLOCATION, SECOND_ALLOCATION } from "../test/fixtures";

const SEARCH = "/engagement-allocations/search";

function Where(): ReactNode {
  const l = useLocation();
  return <div data-testid="where">{`${l.pathname}${l.search}`}</div>;
}

function renderPage(initial = "/allocations") {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initial]}>
        <Where />
        <Routes>
          <Route path="/allocations" element={<CsmAllocationsPage />} />
          <Route path="/allocations/:allocationId" element={<div>detail page</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const page = (over: object = {}) => ({
  allocations: [ALLOCATION, SECOND_ALLOCATION],
  total: 2,
  limit: 20,
  offset: 0,
  hasMore: false,
  ...over,
});
const lastPayload = () => api.post.mock.calls.at(-1)?.[1];

beforeEach(() => {
  api.post.mockReset();
  api.get.mockReset();
  api.post.mockResolvedValue(page());
});

describe("CsmAllocationsPage", () => {
  it("renders a row per allocation with the agreed columns", async () => {
    renderPage();
    expect(await screen.findByText("ALLOC0001001")).toBeInTheDocument();
    for (const name of ["Allocation", "Customer", "Engagement", "Engineer", "State", "Start - End", "Utilization", "Latest update"]) {
      expect(screen.getByRole("columnheader", { name })).toBeInTheDocument();
    }
    const row = screen.getByRole("row", { name: "View allocation ALLOC0001001" });
    expect(within(row).getByText("Example Corp")).toBeInTheDocument();
    expect(within(row).getByText("Example Migration")).toBeInTheDocument();
    expect(within(row).getByText("ENG0001001")).toBeInTheDocument();
    expect(within(row).getByText("Jane Doe")).toBeInTheDocument();
    expect(within(row).getByText("Confirmed")).toBeInTheDocument();
    expect(within(row).getByText("50%")).toBeInTheDocument();
    expect(within(row).getByText(/Sep 21, 2026 - Sep 25, 2026/)).toBeInTheDocument();
    const second = screen.getByRole("row", { name: "View allocation ALLOC0001002" });
    expect(within(second).getByText("Tentative")).toBeInTheDocument();
    expect(within(second).getByText("25.5%")).toBeInTheDocument();
  });

  it("requests newest start date first, 20 per page, no filters, by default", async () => {
    renderPage();
    await screen.findByText("ALLOC0001001");
    expect(api.post).toHaveBeenCalledWith(SEARCH, {
      pagination: { limit: 20, offset: 0 },
      filters: {},
      sortBy: { field: "startDate", order: "desc" },
    });
  });

  it("shows the empty state", async () => {
    api.post.mockResolvedValue(page({ allocations: [], total: 0 }));
    renderPage();
    expect(await screen.findByText("No allocations found.")).toBeInTheDocument();
  });

  it("shows the error state with the server's message and retries", async () => {
    api.post.mockRejectedValueOnce(new Error("search is down"));
    renderPage();
    expect(await screen.findByText("search is down")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(await screen.findByText("ALLOC0001001")).toBeInTheDocument();
  });

  it("maps the text search to filters.searchQuery and resets to the first page", async () => {
    renderPage("/allocations?page=1");
    await screen.findByText("ALLOC0001001");
    fireEvent.change(screen.getByRole("textbox", { name: /search allocations/i }), { target: { value: " migration " } });
    await waitFor(() => expect(lastPayload().filters).toEqual({ searchQuery: "migration" }));
    expect(lastPayload().pagination.offset).toBe(0);
    expect(screen.getByTestId("where").textContent).toBe("/allocations?q=migration");
  });

  it("maps customer, engineer, state and date range to the request", async () => {
    renderPage();
    await screen.findByText("ALLOC0001001");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Customer" }));
    fireEvent.click(await screen.findByRole("option", { name: "Example Corp" }));

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Engineer" }));
    fireEvent.click(await screen.findByRole("option", { name: "Jane Doe" }));

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^state/i }));
    fireEvent.click(await screen.findByRole("option", { name: /^Confirmed$/ }));
    fireEvent.click(screen.getByRole("option", { name: /Tentative/ }));
    fireEvent.keyDown(screen.getByRole("listbox"), { key: "Escape" });

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2026-09-01" } });
    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2026-09-30" } });

    await waitFor(() =>
      expect(lastPayload().filters).toEqual({
        accountId: "00000000-0000-0000-0000-0000000000c1",
        engineerId: "00000000-0000-0000-0000-0000000000b1",
        states: ["CONFIRMED", "TENTATIVE"],
        from: "2026-09-01",
        to: "2026-09-30",
      }),
    );
  });

  it("restores filters from the URL on load", async () => {
    renderPage("/allocations?q=abc&engineer=u-1&engineerLabel=Jane+Doe&state=CONFIRMED&sort=utilization&order=asc&page=1&rows=10");
    await screen.findByText("ALLOC0001001");
    expect(api.post).toHaveBeenCalledWith(SEARCH, {
      pagination: { limit: 10, offset: 10 },
      filters: { searchQuery: "abc", engineerId: "u-1", states: ["CONFIRMED"] },
      sortBy: { field: "utilization", order: "asc" },
    });
    expect(screen.getByRole("combobox", { name: "Engineer" })).toHaveValue("Jane Doe");
  });

  it("sorts by a column, flipping the order on a second click", async () => {
    renderPage();
    await screen.findByText("ALLOC0001001");
    fireEvent.click(screen.getByRole("button", { name: "Utilization" }));
    await waitFor(() => expect(lastPayload().sortBy).toEqual({ field: "utilization", order: "asc" }));
    fireEvent.click(screen.getByRole("button", { name: "Utilization" }));
    await waitFor(() => expect(lastPayload().sortBy).toEqual({ field: "utilization", order: "desc" }));
    fireEvent.click(screen.getByRole("button", { name: "Customer" }));
    await waitFor(() => expect(lastPayload().sortBy).toEqual({ field: "accountName", order: "asc" }));
  });

  it("pages: next page asks for the next offset, and rows per page is capped at 50", async () => {
    api.post.mockResolvedValue(page({ total: 45, hasMore: true }));
    renderPage();
    await screen.findByText("ALLOC0001001");
    fireEvent.click(screen.getByRole("button", { name: /next page/i }));
    await waitFor(() => expect(lastPayload().pagination).toEqual({ limit: 20, offset: 20 }));

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /rows per page/i }));
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toEqual(["10", "20", "50"]);
    fireEvent.click(screen.getByRole("option", { name: "50" }));
    await waitFor(() => expect(lastPayload().pagination).toEqual({ limit: 50, offset: 0 }));
  });

  it("does not send a backwards date range and says so", async () => {
    renderPage();
    await screen.findByText("ALLOC0001001");
    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2026-10-01" } });
    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2026-09-01" } });
    expect(await screen.findByText(/must not be before From/i)).toBeInTheDocument();
    // The range is never sent whole: no request carries a `to` after From passes it.
    expect(api.post.mock.calls.some((c) => c[1].filters.from && c[1].filters.to)).toBe(false);
    expect(api.post.mock.calls.some((c) => c[1].filters.to)).toBe(false);
  });

  it("opens the detail page on row click and on Enter", async () => {
    renderPage();
    await screen.findByText("ALLOC0001001");
    fireEvent.click(screen.getByRole("row", { name: "View allocation ALLOC0001001" }));
    expect(await screen.findByText("detail page")).toBeInTheDocument();
    expect(screen.getByTestId("where").textContent).toBe("/allocations/00000000-0000-0000-0000-0000000000a1");
  });

  it("opens the detail page from the keyboard", async () => {
    renderPage();
    await screen.findByText("ALLOC0001002");
    fireEvent.keyDown(screen.getByRole("row", { name: "View allocation ALLOC0001002" }), { key: "Enter" });
    expect(await screen.findByText("detail page")).toBeInTheDocument();
  });
});
