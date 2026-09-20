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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";
import ChangeRequestsFilterBar from "@features/csm-operations/components/ChangeRequestsFilterBar";
import { DEFAULT_CR_FILTERS } from "@features/csm-operations/utils/changeRequests";
import { useTeams } from "@features/csm-dashboard/api/useTeams";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock, get: vi.fn() }),
}));

vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));

// Mocked directly (same approach as `CasesFilterBar.test.tsx` mocking
// `useSearchTags`) so tests don't have to drive the real SRE-team network
// query — this bar's own control, unrelated to the project filter under test.
vi.mock("@features/csm-dashboard/api/useTeams", () => ({
  useTeams: vi.fn(),
}));
const mockedUseTeams = vi.mocked(useTeams);

beforeEach(() => {
  postMock.mockReset();
  postMock.mockResolvedValue({ projects: [] });
  mockedUseTeams.mockReturnValue({
    data: [],
    isLoading: false,
    isError: false,
  } as unknown as ReturnType<typeof useTeams>);
});

function renderBar(
  filters = DEFAULT_CR_FILTERS,
  onChange = vi.fn(),
): { onChange: ReturnType<typeof vi.fn> } {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    (
      <QueryClientProvider client={queryClient}>
        <ChangeRequestsFilterBar
          filters={filters}
          onChange={onChange}
          onReset={() => {}}
          isFiltersOpen
          onFiltersToggle={() => {}}
        />
      </QueryClientProvider>
    ) as ReactNode,
  );
  return { onChange };
}

describe("ChangeRequestsFilterBar — Project filter", () => {
  it("renders a Project control alongside the existing filters", () => {
    renderBar();
    expect(screen.getByRole("combobox", { name: "Project" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "State" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Impact" })).toBeInTheDocument();
  });

  it("shows the selected project count in the active-filters badge", () => {
    renderBar({ ...DEFAULT_CR_FILTERS, projectIds: ["proj-1"] });
    expect(screen.getByRole("button", { name: "Filters (1)" })).toBeInTheDocument();
  });

  it("shows a Clear filters action once a project is selected", () => {
    renderBar({ ...DEFAULT_CR_FILTERS, projectIds: ["proj-1"] });
    expect(screen.getByRole("button", { name: /Clear filters/i })).toBeInTheDocument();
  });
});
