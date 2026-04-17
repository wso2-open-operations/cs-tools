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
import ListItems from "@components/list-view/ListItems";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

const mockCases = [
  {
    id: "case-1",
    internalId: "INT-1",
    number: "CS001",
    createdOn: "2026-01-01",
    title: "Case One",
    description: "Desc",
    assignedEngineer: null,
    project: { id: "p1", label: "Project A" },
    issueType: { id: "1", label: "Bug" },
    state: { id: 1, label: "Open" },
    severity: { id: "1", label: "High" },
    deployedProduct: null,
    deployment: { id: "d1", label: "Production" },
    status: { id: "s1", label: "Open" },
  },
  {
    id: "case-2",
    internalId: "INT-2",
    number: "CS002",
    createdOn: "2026-01-02",
    title: "Case Two",
    description: "Desc",
    assignedEngineer: null,
    project: { id: "p1", label: "Project A" },
    issueType: { id: "1", label: "Bug" },
    state: { id: 1, label: "Open" },
    severity: { id: "2", label: "High" },
    deployedProduct: null,
    deployment: { id: "d1", label: "Production" },
    status: { id: "s1", label: "Open" },
  },
];

// Mock the table utils
vi.mock("@features/dashboard/utils/casesTable", () => ({
  getStatusColor: vi.fn(() => "success.main"),
  getSeverityColor: vi.fn(() => "error.main"),
}));

// Mock the support utils
vi.mock("@features/support/utils/support", () => ({
  formatDateTime: vi.fn(() => "Jan 1, 2026"),
  formatRelativeTime: vi.fn(() => "2 hours ago"),
  getStatusIcon: vi.fn(() => () => <div data-testid="status-icon" />),
  getStatusColor: vi.fn(() => "#000000"),
  mapSeverityToDisplay: vi.fn((l: string) => l || "—"),
  getAssignedEngineerLabel: vi.fn(() => null),
  resolveColorFromTheme: vi.fn(() => "#000000"),
  stripHtml: vi.fn((html) => html),
}));

describe("ListItems", () => {
  const theme = createTheme();

  it("should render the list of cases", () => {
    render(
      <ThemeProvider theme={theme}>
        <ListItems
          cases={mockCases.slice(0, 2)}
          isLoading={false}
          entityName="cases"
        />
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
        <ListItems cases={[]} isLoading={false} entityName="cases" />
      </ThemeProvider>,
    );

    expect(screen.getByText("No cases yet.")).toBeInTheDocument();
  });

  it("should show search empty state when list is empty and refined", () => {
    render(
      <ThemeProvider theme={theme}>
        <ListItems
          cases={[]}
          isLoading={false}
          hasListRefinement
          entityName="cases"
        />
      </ThemeProvider>,
    );

    expect(
      screen.getByText(
        "No cases found. Try adjusting your filters or search query.",
      ),
    ).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    render(
      <ThemeProvider theme={theme}>
        <ListItems cases={[]} isLoading={true} entityName="cases" />
      </ThemeProvider>,
    );

    // ListSkeleton renders skeleton rows
    const skeletons = screen.getAllByTestId("Skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });
});
