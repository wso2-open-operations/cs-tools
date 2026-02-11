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
import AllCasesList from "@components/support/all-cases/AllCasesList";
import { mockCases } from "@models/mockData";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

// Mock the table utils
vi.mock("@utils/casesTable", () => ({
  getStatusColor: vi.fn(() => "success.main"),
  getSeverityColor: vi.fn(() => "error.main"),
}));

// Mock the support utils
vi.mock("@utils/support", () => ({
  formatRelativeTime: vi.fn(() => "2 hours ago"),
  getStatusIcon: vi.fn(() => () => <div data-testid="status-icon" />),
  resolveColorFromTheme: vi.fn(() => "#000000"),
}));

describe("AllCasesList", () => {
  const theme = createTheme();

  it("should render the list of cases", () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesList cases={mockCases.slice(0, 2)} isLoading={false} />
      </ThemeProvider>,
    );

    expect(screen.getByText(mockCases[0].number)).toBeInTheDocument();
    expect(screen.getByText(mockCases[0].title)).toBeInTheDocument();
    expect(screen.getByText(mockCases[1].number)).toBeInTheDocument();
    expect(screen.getByText(mockCases[1].title)).toBeInTheDocument();
  });

  it("should render 'No cases found' when the list is empty", () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesList cases={[]} isLoading={false} />
      </ThemeProvider>,
    );

    expect(screen.getByText(/No cases found/i)).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    render(
      <ThemeProvider theme={theme}>
        <AllCasesList cases={[]} isLoading={true} />
      </ThemeProvider>,
    );

    // AllCasesListSkeleton renders 5 skeletons
    const skeletons = screen.getAllByTestId("Skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
