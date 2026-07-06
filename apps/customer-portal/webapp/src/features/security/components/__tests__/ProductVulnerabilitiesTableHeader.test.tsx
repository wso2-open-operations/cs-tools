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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import ProductVulnerabilitiesTableHeader from "@features/security/components/ProductVulnerabilitiesTableHeader";
import { PRODUCT_VULNERABILITIES_SEARCH_PLACEHOLDER } from "@features/security/constants/securityConstants";

describe("ProductVulnerabilitiesTableHeader", () => {
  it("invokes search callbacks and clear icon action", () => {
    const onSearchChange = vi.fn();
    render(
      <ProductVulnerabilitiesTableHeader
        searchValue="cve"
        onSearchChange={onSearchChange}
        onFilterToggle={vi.fn()}
        isFiltersOpen={false}
        activeFiltersCount={0}
      />,
    );

    fireEvent.change(
      screen.getByPlaceholderText(PRODUCT_VULNERABILITIES_SEARCH_PLACEHOLDER),
      { target: { value: "cve-123" } },
    );
    const clearButton = screen
      .getAllByRole("button")
      .find((button) => button.className.includes("MuiIconButton"));
    expect(clearButton).toBeDefined();
    fireEvent.click(clearButton!);
    expect(onSearchChange).toHaveBeenCalledWith("cve-123");
    expect(onSearchChange).toHaveBeenCalledWith("");
  });

  it("shows clear-filters label when active filters exist", () => {
    render(
      <ProductVulnerabilitiesTableHeader
        searchValue=""
        onSearchChange={vi.fn()}
        onFilterToggle={vi.fn()}
        isFiltersOpen
        activeFiltersCount={2}
      />,
    );

    expect(screen.getByRole("button", { name: /clear filters \(2\)/i })).toBeInTheDocument();
  });
});
