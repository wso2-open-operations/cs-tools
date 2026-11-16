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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncUserIdMultiSelect from "@features/csm-cases/components/AsyncUserIdMultiSelect";
import { useInfiniteUserSearch } from "@features/csm-cases/api/useUserSearch";
import { useUserNamesById } from "@features/csm-users/api/useUserNamesById";

vi.mock("@features/csm-cases/api/useUserSearch", () => ({
  useInfiniteUserSearch: vi.fn(),
}));
vi.mock("@features/csm-users/api/useUserNamesById", () => ({
  useUserNamesById: vi.fn(() => new Map()),
}));
const get = vi.hoisted(() => vi.fn());
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get, post: vi.fn() }),
}));

const mockedUseUserNamesById = vi.mocked(useUserNamesById);

const mockedUseInfiniteUserSearch = vi.mocked(useInfiniteUserSearch);

afterEach(() => {
  mockedUseInfiniteUserSearch.mockReset();
  mockedUseUserNamesById.mockReset();
  mockedUseUserNamesById.mockImplementation(() => new Map());
});

const NO_RESULTS = {
  users: [],
  isFetching: false,
  isFetchingNextPage: false,
  hasNextPage: false,
  isError: false,
  fetchNextPage: vi.fn(),
};

describe("AsyncUserIdMultiSelect — @me label", () => {
  it("renders a selected value equal to currentUserId as 'Me', not the raw UUID", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdMultiSelect
        values={["11111111-1111-1111-1111-111111111111"]}
        onChange={vi.fn()}
        currentUserId="11111111-1111-1111-1111-111111111111"
      />,
    );

    expect(screen.getByText("Me")).toBeInTheDocument();
    expect(
      screen.queryByText("11111111-1111-1111-1111-111111111111"),
    ).not.toBeInTheDocument();
  });

  it("falls back to the raw UUID when the value doesn't match currentUserId and has no nameSeed", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdMultiSelect
        values={["22222222-2222-2222-2222-222222222222"]}
        onChange={vi.fn()}
        currentUserId="11111111-1111-1111-1111-111111111111"
      />,
    );

    expect(screen.getByText("22222222-2222-2222-2222-222222222222")).toBeInTheDocument();
    expect(screen.queryByText("Me")).not.toBeInTheDocument();
  });

  it("prefers a nameSeed label over the raw UUID for a non-'me' selection", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdMultiSelect
        values={["22222222-2222-2222-2222-222222222222"]}
        onChange={vi.fn()}
        nameSeed={new Map([["22222222-2222-2222-2222-222222222222", "Jane Doe"]])}
      />,
    );

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
  });
});

describe("AsyncUserIdMultiSelect — server-side role scoping", () => {
  it("forwards roleIds/active straight through to useInfiniteUserSearch's scope, not just the typed query", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdMultiSelect
        values={[]}
        onChange={vi.fn()}
        roleIds={["internal", "agent"]}
        active
      />,
    );

    expect(mockedUseInfiniteUserSearch).toHaveBeenCalledWith("", false, {
      roleIds: ["internal", "agent"],
      active: true,
    });
  });

  it("passes an unset scope through unchanged when roleIds/active aren't given", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(<AsyncUserIdMultiSelect values={[]} onChange={vi.fn()} />);

    expect(mockedUseInfiniteUserSearch).toHaveBeenCalledWith("", false, {
      roleIds: undefined,
      active: undefined,
    });
  });
});

describe("AsyncUserIdMultiSelect — names looked up by id", () => {
  it("shows the looked-up name for a selected id with no known name", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);
    mockedUseUserNamesById.mockImplementation(
      (ids) => new Map(ids.map((id) => [id, "Jane Doe"] as [string, string])),
    );

    render(
      <AsyncUserIdMultiSelect
        values={["22222222-2222-2222-2222-222222222222"]}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.queryByText("22222222-2222-2222-2222-222222222222")).not.toBeInTheDocument();
  });

  it("does not look up an id already named by nameSeed or the current user", () => {
    mockedUseInfiniteUserSearch.mockReturnValue(NO_RESULTS);

    render(
      <AsyncUserIdMultiSelect
        values={["11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"]}
        onChange={vi.fn()}
        currentUserId="11111111-1111-1111-1111-111111111111"
        nameSeed={new Map([["22222222-2222-2222-2222-222222222222", "Seeded"]])}
      />,
    );

    expect(mockedUseUserNamesById).toHaveBeenLastCalledWith([]);
  });
});

describe("AsyncUserIdMultiSelect — picking while a lookup is pending", () => {
  it("still shows the first user's name when their lookup settles after another user is picked", async () => {
    const A = "33333333-3333-3333-3333-333333333333";
    const B = "44444444-4444-4444-4444-444444444444";
    const actual = await vi.importActual<typeof import("@features/csm-users/api/useUserNamesById")>(
      "@features/csm-users/api/useUserNamesById",
    );
    mockedUseUserNamesById.mockImplementation(actual.useUserNamesById);
    let settleA: (user: unknown) => void = () => {};
    get.mockReset();
    get.mockImplementation(() => new Promise((resolve) => (settleA = resolve)));
    mockedUseInfiniteUserSearch.mockReturnValue({
      ...NO_RESULTS,
      users: [{ id: B, name: "Bob Smith", email: "bob@wso2.com" }],
    });
    const onNamesResolved = vi.fn();

    function Harness() {
      const [values, setValues] = useState([A]);
      return (
        <AsyncUserIdMultiSelect values={values} onChange={setValues} onNamesResolved={onNamesResolved} />
      );
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <Harness />
      </QueryClientProvider>,
    );
    expect(screen.getByText(A)).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Bob Smith" }));

    // A's raw-id label is neither cached nor reported as its name.
    expect(onNamesResolved).toHaveBeenLastCalledWith([[B, "Bob Smith"]]);

    settleA({ id: A, userName: "alice", firstName: "Alice", lastName: "Jones", email: "alice@wso2.com" });

    expect((await screen.findAllByText("Alice Jones, Bob Smith")).length).toBeGreaterThan(0);
    expect(get).toHaveBeenCalledTimes(1);
    expect(get).toHaveBeenCalledWith(`/users/${A}`);
  });
});
