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
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncUserIdSingleSelect from "@features/csm-cases/components/AsyncUserIdSingleSelect";
import { useInfiniteUserSearch } from "@features/csm-cases/api/useUserSearch";

vi.mock("@features/csm-cases/api/useUserSearch", () => ({
  useInfiniteUserSearch: vi.fn(),
}));

const mockedUseInfiniteUserSearch = vi.mocked(useInfiniteUserSearch);

afterEach(() => {
  mockedUseInfiniteUserSearch.mockReset();
});

const NO_RESULTS = {
  users: [],
  isFetching: false,
  isFetchingNextPage: false,
  hasNextPage: false,
  isError: false,
  fetchNextPage: vi.fn(),
};

describe("AsyncUserIdSingleSelect — selection rendering", () => {
  it("renders nothing selected when value is empty", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(<AsyncUserIdSingleSelect value="" onChange={vi.fn()} label="Approver" />);

    expect(screen.getByLabelText("Approver")).toHaveValue("");
  });

  it("falls back to the raw UUID when the value has no nameSeed and no search result", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdSingleSelect
        value="22222222-2222-2222-2222-222222222222"
        onChange={vi.fn()}
        label="Approver"
      />,
    );

    expect(screen.getByDisplayValue("22222222-2222-2222-2222-222222222222")).toBeInTheDocument();
  });

  it("prefers a nameSeed label over the raw UUID", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdSingleSelect
        value="22222222-2222-2222-2222-222222222222"
        onChange={vi.fn()}
        nameSeed={new Map([["22222222-2222-2222-2222-222222222222", "Jane Doe"]])}
        label="Approver"
      />,
    );

    expect(screen.getByDisplayValue("Jane Doe")).toBeInTheDocument();
  });
});

describe("AsyncUserIdSingleSelect — server-side role scoping", () => {
  it("forwards roleIds/active straight through to useInfiniteUserSearch's scope", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdSingleSelect
        value=""
        onChange={vi.fn()}
        roleIds={["timecard_approver"]}
        active
        label="Approver"
      />,
    );

    expect(mockedUseInfiniteUserSearch).toHaveBeenCalledWith("", false, {
      roleIds: ["timecard_approver"],
      active: true,
    });
  });
});
