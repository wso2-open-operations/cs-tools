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
import { describe, expect, it, vi, beforeEach } from "vitest";
import AllCasesStatCards from "@features/support/components/all-cases/AllCasesStatCards";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

// Mock the hook
vi.mock("@features/dashboard/api/useGetProjectCasesStats", () => ({
  useGetProjectCasesStats: vi.fn(),
}));

import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";

describe("AllCasesStatCards", () => {
  const theme = createTheme();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should render stat cards with data", () => {
    const mockStats = {
      totalCases: 100,
      averageResponseTime: 0,
      resolvedCases: { total: 60, currentMonth: 0 },
      stateCount: [
        { id: "1", label: "Work In Progress", count: 10 },
        { id: "2", label: "Awaiting Info", count: 20 },
        { id: "3", label: "Waiting On WSO2", count: 30 },
        { id: "4", label: "Closed", count: 40 },
      ],
      severityCount: [],
      outstandingSeverityCount: [],
      caseTypeCount: [],
      casesTrend: [],
    };

    (useGetProjectCasesStats as any).mockReturnValue({
      data: mockStats,
      isLoading: false,
    });

    render(
      <ThemeProvider theme={theme}>
        <AllCasesStatCards isLoading={false} stats={mockStats} />
      </ThemeProvider>,
    );

    // openCases = 10+20+30 = 60 (excluding Closed)
    // workInProgress (10), waitingOnClient (20), waitingOnWso2 (30)
    expect(screen.getByText("60")).toBeInTheDocument();
    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("30")).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    (useGetProjectCasesStats as any).mockReturnValue({
      data: undefined,
      isLoading: true,
    });

    render(
      <ThemeProvider theme={theme}>
        <AllCasesStatCards isLoading={true} stats={undefined} />
      </ThemeProvider>,
    );

    // Should find skeleton elements
    const skeletons = screen.getAllByTestId("Skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
