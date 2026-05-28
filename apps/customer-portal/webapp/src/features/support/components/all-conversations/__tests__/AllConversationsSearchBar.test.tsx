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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it, vi } from "vitest";
import AllConversationsSearchBar from "../AllConversationsSearchBar";

describe("AllConversationsSearchBar", () => {
  it("updates search text through callback", () => {
    const onSearchChange = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <AllConversationsSearchBar
          searchTerm=""
          onSearchChange={onSearchChange}
          isFiltersOpen={false}
          onFiltersToggle={vi.fn()}
          filters={{ stateId: "" }}
          filterMetadata={undefined}
          onFilterChange={vi.fn()}
          onClearFilters={vi.fn()}
        />
      </ThemeProvider>,
    );

    fireEvent.change(screen.getByPlaceholderText(/search chats/i), {
      target: { value: "error" },
    });
    expect(onSearchChange).toHaveBeenCalledWith("error");
  });
});
