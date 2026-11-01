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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncProjectSelect from "@features/csm-cases/components/AsyncProjectSelect";
import { useInfiniteProjectSearch } from "@features/csm-cases/api/useProjectSearch";
import type { BeProject } from "@api/backend/types";

vi.mock("@features/csm-cases/api/useProjectSearch", () => ({
  useInfiniteProjectSearch: vi.fn(),
}));

const mockedUseInfiniteProjectSearch = vi.mocked(useInfiniteProjectSearch);

afterEach(() => {
  mockedUseInfiniteProjectSearch.mockReset();
});

const MANAGED_CLOUD_PROJECT: BeProject = {
  id: "proj-managed",
  name: "Managed Cloud Co",
  subscriptionType: "managed_cloud_subscription",
};
const SUBSCRIPTION_PROJECT: BeProject = {
  id: "proj-subscription",
  name: "Subscription Only Co",
  subscriptionType: "subscription",
};

const isManagedCloudProject = (p: BeProject): boolean =>
  p.subscriptionType === "managed_cloud_subscription";

describe("AsyncProjectSelect — filterProject pagination", () => {
  it("keeps loading pages when the current page's matches are all filtered out", async () => {
    const fetchNextPage = vi.fn();
    // First page has no managed-cloud project at all — with filterProject
    // applied, options is empty and the listbox can never be scrolled to
    // trigger the scroll-based fetchNextPage, so this must be driven
    // automatically instead.
    mockedUseInfiniteProjectSearch.mockReturnValue({
      projects: [SUBSCRIPTION_PROJECT],
      isFetching: false,
      isFetchingNextPage: false,
      hasNextPage: true,
      isError: false,
      fetchNextPage,
    });

    render(
      <AsyncProjectSelect
        value=""
        onChange={vi.fn()}
        filterProject={isManagedCloudProject}
      />,
    );
    // The auto-continue effect only runs while the dropdown is open.
    fireEvent.mouseDown(screen.getByRole("combobox"));

    await waitFor(() => expect(fetchNextPage).toHaveBeenCalledTimes(1));
  });

  it("stops requesting more pages once a match is loaded", () => {
    const fetchNextPage = vi.fn();
    mockedUseInfiniteProjectSearch.mockReturnValue({
      projects: [SUBSCRIPTION_PROJECT, MANAGED_CLOUD_PROJECT],
      isFetching: false,
      isFetchingNextPage: false,
      hasNextPage: true,
      isError: false,
      fetchNextPage,
    });

    render(
      <AsyncProjectSelect
        value=""
        onChange={vi.fn()}
        filterProject={isManagedCloudProject}
      />,
    );
    fireEvent.mouseDown(screen.getByRole("combobox"));

    expect(fetchNextPage).not.toHaveBeenCalled();
  });

  it("stops requesting more pages once hasNextPage is false", () => {
    const fetchNextPage = vi.fn();
    mockedUseInfiniteProjectSearch.mockReturnValue({
      projects: [SUBSCRIPTION_PROJECT],
      isFetching: false,
      isFetchingNextPage: false,
      hasNextPage: false,
      isError: false,
      fetchNextPage,
    });

    render(
      <AsyncProjectSelect
        value=""
        onChange={vi.fn()}
        filterProject={isManagedCloudProject}
      />,
    );
    fireEvent.mouseDown(screen.getByRole("combobox"));

    expect(fetchNextPage).not.toHaveBeenCalled();
    expect(screen.queryByText(/loading/i)).not.toBeInTheDocument();
  });
});

describe("AsyncProjectSelect — knownLabel, picked name, clearing", () => {
  const idle = {
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  };

  it("labels a value that no search has returned with knownLabel instead of the raw id", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue({ ...idle, projects: [] });
    render(<AsyncProjectSelect value="proj-x" knownLabel="Restored Project" onChange={vi.fn()} />);
    expect(screen.getByRole("combobox")).toHaveValue("Restored Project");
  });

  it("falls back to the id when there is neither a search result nor a knownLabel", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue({ ...idle, projects: [] });
    render(<AsyncProjectSelect value="proj-x" onChange={vi.fn()} />);
    expect(screen.getByRole("combobox")).toHaveValue("proj-x");
  });

  it("reports the picked project's id and display name, and an empty id on clear", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue({ ...idle, projects: [SUBSCRIPTION_PROJECT] });
    const onChange = vi.fn();
    render(<AsyncProjectSelect value="" onChange={onChange} />);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Subscription Only Co" }));
    expect(onChange).toHaveBeenLastCalledWith("proj-subscription", "Subscription Only Co");
  });

  it("offers a clear button by default, and none with disableClearable", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue({ ...idle, projects: [SUBSCRIPTION_PROJECT] });
    // MUI only shows the clear button on hover/focus, so it is looked up by
    // its class rather than by role.
    const first = render(<AsyncProjectSelect value="proj-subscription" onChange={vi.fn()} />);
    expect(first.container.querySelector(".MuiAutocomplete-clearIndicator")).not.toBeNull();
    first.unmount();
    const second = render(
      <AsyncProjectSelect value="proj-subscription" disableClearable onChange={vi.fn()} />,
    );
    expect(second.container.querySelector(".MuiAutocomplete-clearIndicator")).toBeNull();
  });
});

describe("AsyncProjectSelect — required and helper text", () => {
  const idle = {
    projects: [] as BeProject[],
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  };

  it("marks the field required and shows the helper text under it", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue(idle);
    render(
      <AsyncProjectSelect
        label="Customer Project"
        value=""
        required
        helperText="Required to request approval."
        onChange={vi.fn()}
      />,
    );
    const input = screen.getByRole("combobox", { name: "Customer Project" });
    expect(input).toBeRequired();
    expect(input).toHaveAccessibleDescription("Required to request approval.");
  });

  it("is not required by default, and a failed search's message replaces the helper text", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue({ ...idle, isError: true });
    render(
      <AsyncProjectSelect
        label="Customer Project"
        value=""
        helperText="Required to request approval."
        onChange={vi.fn()}
      />,
    );
    expect(screen.getByRole("combobox", { name: "Customer Project" })).not.toBeRequired();
    expect(screen.getByText("Project search failed.")).toBeInTheDocument();
    expect(screen.queryByText("Required to request approval.")).not.toBeInTheDocument();
  });
});

describe("AsyncProjectSelect — narrowed to one customer", () => {
  const idle = {
    projects: [],
    isFetching: false,
    isFetchingNextPage: false,
    hasNextPage: false,
    isError: false,
    fetchNextPage: vi.fn(),
  };

  it("passes the customer (account) to the server-side search, and says why under the field", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue(idle);
    render(
      <AsyncProjectSelect
        value=""
        onChange={vi.fn()}
        accountId="acct-1"
        helperText="Projects of Acme Corp only."
      />,
    );
    fireEvent.mouseDown(screen.getByRole("combobox"));
    expect(mockedUseInfiniteProjectSearch).toHaveBeenLastCalledWith("", true, "acct-1");
    expect(screen.getByText("Projects of Acme Corp only.")).toBeInTheDocument();
  });

  it("searches every project when no customer is given", () => {
    mockedUseInfiniteProjectSearch.mockReturnValue(idle);
    render(<AsyncProjectSelect value="" onChange={vi.fn()} />);
    expect(mockedUseInfiniteProjectSearch).toHaveBeenLastCalledWith("", false, undefined);
  });
});
