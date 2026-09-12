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
