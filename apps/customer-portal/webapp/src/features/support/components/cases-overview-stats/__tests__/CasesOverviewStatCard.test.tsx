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
import { describe, expect, it } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import CasesOverviewStatCard from "@features/support/components/cases-overview-stats/CasesOverviewStatCard";
import { SUPPORT_STAT_CONFIGS } from "@features/support/constants/supportConstants";

describe("CasesOverviewStatCard", () => {
  const theme = createTheme();

  it("should render loading state correctly", () => {
    render(
      <ThemeProvider theme={theme}>
        <CasesOverviewStatCard isLoading={true} stats={undefined} />
      </ThemeProvider>,
    );

    expect(document.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(
      0,
    );
  });

  it("should render statistics correctly when data is loaded", () => {
    const mockStats = {
      ongoingCases: 10,
      resolvedPast30DaysCasesCount: 15,
      resolvedChats: 20,
      activeChats: 5,
    };

    render(
      <ThemeProvider theme={theme}>
        <CasesOverviewStatCard isLoading={false} stats={mockStats} />
      </ThemeProvider>,
    );

    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("15")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();

    SUPPORT_STAT_CONFIGS.forEach((config) => {
      expect(screen.getByText(config.label)).toBeInTheDocument();
    });
  });
});
