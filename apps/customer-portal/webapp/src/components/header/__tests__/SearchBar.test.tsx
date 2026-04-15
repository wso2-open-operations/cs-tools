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
import SearchBar from "@components/header/SearchBar";

vi.mock("react-router", () => ({
  useNavigate: () => vi.fn(),
  useParams: () => ({ projectId: "proj-1" }),
}));

vi.mock("@hooks/useDebouncedValue", () => ({
  useDebouncedValue: (value: string) => value,
}));

vi.mock("@api/useGetProjectCases", () => ({
  default: () => ({
    data: { pages: [{ cases: [] }] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/support/components/AllCasesList", () => ({
  default: () => <div data-testid="all-cases-list" />,
}));

vi.mock("@features/support/components/all-cases/AllCasesListSkeleton", () => ({
  default: () => <div data-testid="all-cases-list-skeleton" />,
}));

vi.mock("@components/empty-state/SearchNoResultsIcon", () => ({
  default: () => <svg data-testid="search-no-results-icon" />,
}));

vi.mock("@components/error/Error500Page", () => ({
  default: () => <svg data-testid="error-state-icon" />,
}));

describe("SearchBar", () => {
  it("should render the search input with correct placeholder", () => {
    render(<SearchBar projectId="proj-1" />);

    const searchInput = screen.getByPlaceholderText(
      "Search cases, tickets, or users",
    );
    expect(searchInput).toBeInTheDocument();
  });
});
