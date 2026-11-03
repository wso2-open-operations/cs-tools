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

import { render } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

// The "Export CSV" action builds its own `/cases/search` request, so it must
// resolve relative dates exactly as the listing (`useGetCsmCases`) does. A raw
// placeholder would be resolved by entity-service in UTC instead of the
// viewer's local time. The export button is stubbed to hand over its
// `fetchPage`, and the request it posts is asserted.

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
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "user@example.test" }),
}));
vi.mock("@api/useDirectoryUsers", () => ({
  useDirectoryUsers: () => ({ data: [] }),
}));
vi.mock("@features/csm-cases/api/useGetCsmCases", () => ({
  useGetCsmCases: () => ({
    data: { cases: [], total: 5, hasMore: false },
    isLoading: false,
    isFetching: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
    dataUpdatedAt: 0,
  }),
}));
vi.mock("@features/csm-cases/components/CasesFilterBar", () => ({
  default: () => <div>FilterBar</div>,
}));
vi.mock("@features/csm-cases/components/CasesList", () => ({
  default: () => <div>CasesList</div>,
}));
let exportFetchPage: ((offset: number, limit: number) => Promise<unknown>) | undefined;
vi.mock("@components/FilteredCsvExportButton", () => ({
  default: ({ fetchPage }: { fetchPage: (offset: number, limit: number) => Promise<unknown> }) => {
    exportFetchPage = fetchPage;
    return <div>ExportButton</div>;
  },
}));
vi.mock("@components/RefreshButton", () => ({
  default: () => <div>RefreshButton</div>,
}));

import CsmIssuesView from "@features/csm-cases/components/CsmIssuesView";

// Node 26 exposes `window.localStorage` as undefined unless the process was
// started with `--localstorage-file`, which hides jsdom's own Storage.
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
      clear: () => {
        store.clear();
      },
      key: (index: number) => [...store.keys()][index] ?? null,
      get length() {
        return store.size;
      },
    },
  });
}

beforeEach(() => {
  installLocalStorage();
  window.localStorage.clear();
  postMock.mockReset();
  postMock.mockResolvedValue({ cases: [], total: 0 });
  exportFetchPage = undefined;
});

describe("CsmIssuesView Export CSV and relative dates", () => {
  it("resolves the URL's relative dates to local instants before posting the export search", async () => {
    render(
      <MemoryRouter initialEntries={["/cases?createdFrom=__daysAgo:30__&createdTo=__today__"]}>
        <Routes>
          <Route path="/cases" element={<CsmIssuesView title="Cases" />} />
        </Routes>
      </MemoryRouter>,
    );
    expect(exportFetchPage).toBeDefined();
    await exportFetchPage!(0, 100);

    const [path, payload] = postMock.mock.calls[0];
    expect(path).toBe("/cases/search");
    const created = (payload.filters.filters as { field: string; op: string; values: string[] }[])
      .filter((f) => f.field === "createdOn");
    const gte = created.find((f) => f.op === "gte")!.values[0];
    const lte = created.find((f) => f.op === "lte")!.values[0];
    expect(JSON.stringify(payload)).not.toContain("__");

    const today = new Date();
    const thirtyDaysAgo = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 30);
    expect(new Date(gte).getTime()).toBe(thirtyDaysAgo.getTime());
    // Up to the end of today, local time.
    const endOfToday = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1);
    expect(new Date(lte).getTime()).toBeLessThan(endOfToday.getTime());
    expect(new Date(lte).getTime()).toBeGreaterThanOrEqual(endOfToday.getTime() - 1000);
  });
});
