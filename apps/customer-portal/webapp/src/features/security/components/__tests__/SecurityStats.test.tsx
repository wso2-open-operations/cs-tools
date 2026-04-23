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
import { describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { type ReactNode } from "react";
import SecurityStats from "@features/security/components/SecurityStats";

vi.mock("@features/security/api/usePostProductVulnerabilitiesSearch", () => ({
  usePostProductVulnerabilitiesSearch: vi.fn(() => ({
    data: { totalRecords: 1265, offset: 0, limit: 10 },
    isLoading: false,
    isError: false,
  })),
}));

vi.mock("@features/dashboard/api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: vi.fn(() => ({
    data: {
      activeCount: 25,
      resolvedCases: {
        total: 6,
        currentMonth: 3,
        pastThirtyDays: 4,
      },
    },
    isLoading: false,
    isError: false,
  })),
}));

function renderSecurityStats() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={createTheme()}>
        <MemoryRouter initialEntries={["/projects/test-project/security-center"]}>
          <Routes>
            <Route
              path="/projects/:projectId/security-center"
              element={children}
            />
          </Routes>
        </MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );

  return render(<SecurityStats />, { wrapper });
}

describe("SecurityStats", () => {
  it("should render three stat cards with correct labels", () => {
    renderSecurityStats();

    expect(screen.getByText("Total Vulnerabilities")).toBeInTheDocument();
    expect(screen.getByText("Active Security Reports")).toBeInTheDocument();
    expect(
      screen.getByText("Resolved Security Reports (Last 30d)"),
    ).toBeInTheDocument();
  });

  it("should display vulnerability count from API", () => {
    renderSecurityStats();
    expect(screen.getByText("1265")).toBeInTheDocument();
  });

  it("should display active security reports count", () => {
    renderSecurityStats();
    expect(screen.getByText("25")).toBeInTheDocument();
  });

  it("should display resolved security reports count from past 30 days", () => {
    renderSecurityStats();
    expect(screen.getByText("4")).toBeInTheDocument();
  });

  it("should render with loading state", async () => {
    const { usePostProductVulnerabilitiesSearch } = await import(
      "@features/security/api/usePostProductVulnerabilitiesSearch"
    );
    vi.mocked(usePostProductVulnerabilitiesSearch).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as any);

    renderSecurityStats();

    const skeletons = document.querySelectorAll('[class*="MuiSkeleton"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
