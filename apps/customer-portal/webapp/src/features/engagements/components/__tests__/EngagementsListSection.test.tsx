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
import { SortOrder } from "@/types/common";
import { EngagementsSortField } from "@features/engagements/types/engagements";
import EngagementsListSection from "@features/engagements/components/EngagementsListSection";

vi.mock("@components/list-view/ListItems", () => ({
  default: () => <div data-testid="list-items" />,
}));

describe("EngagementsListSection", () => {
  const baseProps = {
    searchTerm: "",
    onSearchChange: () => {},
    isFiltersOpen: false,
    onFiltersToggle: () => {},
    filters: {},
    filterMetadata: { caseStates: [{ id: "1", label: "Open" }] },
    onFilterChange: () => {},
    onClearFilters: () => {},
    isProjectContextLoading: false,
    sortField: EngagementsSortField.UpdatedOn,
    onSortFieldChange: () => {},
    sortOrder: SortOrder.DESC,
    onSortOrderChange: () => {},
    paginatedCases: [],
    isCasesAreaLoading: false,
    isCasesError: false,
    listHasRefinement: false,
    totalItems: 0,
    onCaseClick: undefined,
    page: 1,
    rowsPerPage: 10,
    onPageChange: () => {},
    onRowsPerPageChange: () => {},
  };

  it("renders search placeholder when not stat-filtered", () => {
    render(<EngagementsListSection {...baseProps} />);
    expect(
      screen.getByPlaceholderText(
        "Search engagements by ID, title, or description...",
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("list-items")).toBeInTheDocument();
  });

  it("hides search bar when stat-filtered simplified view", () => {
    render(<EngagementsListSection {...baseProps} isStatFiltered />);
    expect(
      screen.queryByPlaceholderText(
        "Search engagements by ID, title, or description...",
      ),
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("list-items")).toBeInTheDocument();
  });
});
