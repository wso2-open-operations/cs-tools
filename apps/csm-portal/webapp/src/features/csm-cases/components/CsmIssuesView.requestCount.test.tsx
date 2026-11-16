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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// Real `useGetCsmCases`, `useDirectoryUsers`, `CasesFilterBar` and
// `AsyncAssigneeMultiSelect`: only the network edge, auth context and the
// heavy table/export chrome are stubbed, so this counts the requests a fresh
// visit to the Support page genuinely puts on the wire.

const postMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({
    user: { id: "user-1", roles: ["cs_engineer"] },
    isLoading: false,
    isError: false,
  }),
}));
vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));
vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));
// The ID-token claims decode asynchronously in the app: undefined on the first
// render, the email a tick later. Tests flip this and re-render to mimic that.
let mockClaimsEmail: string | undefined;
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => (mockClaimsEmail ? { email: mockClaimsEmail } : undefined),
}));
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn(), debug: vi.fn() }),
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  default: () => <div>CasesList</div>,
}));
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: () => <div>ExportButton</div>,
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

function installLocalStorage(): void {
  if (typeof window.localStorage?.clear === "function") return;
  const store = new Map<string, string>();
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, String(value));
      },
      removeItem: (key: string) => {
        store.delete(key);
      },
      clear: () => store.clear(),
    },
  });
}

function posts(path: string): { filters: { filters?: { field: string; values?: string[] }[] } }[] {
  return postMock.mock.calls.filter(([url]) => url === path).map(([, body]) => body);
}

function typesOf(body: ReturnType<typeof posts>[number]): string[] | undefined {
  return body.filters.filters?.find((f) => f.field === "type")?.values;
}

function renderView(url: string, mode: "cases" | "locked" = "cases") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const element = (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[url]}>
        {mode === "cases" ? (
          <CsmIssuesView
            title="Cases"
            entityNoun="cases"
            defaultCaseTypes={["case"]}
            lockedFilters={{ caseTypes: ["case"] }}
          />
        ) : (
          <CsmIssuesView
            title="Service requests"
            hideTypeFilter
            lockedFilters={{ caseTypes: ["service_request"] }}
          />
        )}
      </MemoryRouter>
    </QueryClientProvider>
  );
  const utils = render(element);
  return { ...utils, rerenderSame: () => utils.rerender(element) };
}

beforeEach(() => {
  installLocalStorage();
  window.localStorage.clear();
  postMock.mockReset();
  mockClaimsEmail = undefined;
  postMock.mockImplementation((url: string) =>
    Promise.resolve(
      url === "/users/search"
        ? { users: [{ firstName: "Jane", lastName: "Doe", email: "jane.doe@example.com" }] }
        : { cases: [], total: 0, limit: 20, offset: 0, hasMore: false },
    ),
  );
});

describe("CsmIssuesView first-visit request count", () => {
  it("issues exactly one /cases/search on a fresh /cases visit, already filtered to cases", async () => {
    renderView("/cases");
    await waitFor(() => expect(posts("/cases/search").length).toBeGreaterThan(0));
    // Let any follow-up render (URL write, late claims) settle before counting.
    await act(async () => {
      mockClaimsEmail = "jane.doe@example.com";
      await new Promise((r) => setTimeout(r, 50));
    });

    const searches = posts("/cases/search");
    expect(searches).toHaveLength(1);
    expect(typesOf(searches[0])).toEqual(["case"]);
  });

  it("does not refetch the same search when the signed-in email arrives after first render", async () => {
    const { rerenderSame } = renderView("/cases?types=case");
    await waitFor(() => expect(posts("/cases/search")).toHaveLength(1));

    mockClaimsEmail = "jane.doe@example.com";
    await act(async () => {
      rerenderSame();
      await new Promise((r) => setTimeout(r, 50));
    });

    expect(posts("/cases/search")).toHaveLength(1);
  });

  it("writes the default types into the URL without issuing a differently-keyed search first", async () => {
    renderView("/cases");
    await waitFor(() => expect(posts("/cases/search").length).toBeGreaterThan(0));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });
    expect(posts("/cases/search").every((b) => typesOf(b)?.join() === "case")).toBe(true);
  });

  it("clearing the type selection back to none still means every visible type (default is not re-seeded)", async () => {
    renderView("/cases");
    await waitFor(() => expect(posts("/cases/search")).toHaveLength(1));

    // Open the type control and untick the only selected type.
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /type/i }));
    fireEvent.click(await screen.findByRole("option", { name: /^case$/i }));

    await waitFor(() => expect(posts("/cases/search")).toHaveLength(2));
    expect(typesOf(posts("/cases/search")[1])?.length).toBeGreaterThan(1);
  });

  it("honours an explicit ?types= from the URL instead of the default", async () => {
    renderView("/cases?types=engagement");
    await waitFor(() => expect(posts("/cases/search")).toHaveLength(1));
    expect(typesOf(posts("/cases/search")[0])).toEqual(["engagement"]);
  });

  it("a locked view (hideTypeFilter) still queries only its locked type, once", async () => {
    renderView("/service-requests", "locked");
    await waitFor(() => expect(posts("/cases/search")).toHaveLength(1));
    expect(typesOf(posts("/cases/search")[0])).toEqual(["service_request"]);
  });
});

describe("CsmIssuesView user directory fetch", () => {
  it("does not call /users/search on initial render", async () => {
    renderView("/cases");
    await waitFor(() => expect(posts("/cases/search")).toHaveLength(1));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });
    expect(posts("/users/search")).toHaveLength(0);
  });

  it("does not call /users/search for an @me-only assignee selection", async () => {
    renderView("/cases?assignees=%40me");
    await waitFor(() => expect(posts("/cases/search").length).toBeGreaterThan(0));
    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });
    expect(posts("/users/search")).toHaveLength(0);
  });

  it("calls /users/search once the assignee filter is opened", async () => {
    renderView("/cases");
    await waitFor(() => expect(posts("/cases/search")).toHaveLength(1));
    expect(posts("/users/search")).toHaveLength(0);

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /assignee/i }));
    await waitFor(() => expect(posts("/users/search").length).toBeGreaterThan(0));
  });

  it("loads the directory when the URL already carries a named assignee, to label its chip", async () => {
    renderView("/cases?assignees=jane.doe%40example.com");
    await waitFor(() => expect(posts("/users/search").length).toBeGreaterThan(0));
  });
});
