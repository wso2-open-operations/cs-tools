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
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import ProjectHub from "@features/project-hub/pages/ProjectHub";

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return { ...actual, useNavigate: () => vi.fn() };
});

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

vi.mock("@context/linear-loader/LoaderContext", () => ({
  useLoader: () => ({ showLoader: vi.fn(), hideLoader: vi.fn() }),
}));

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

vi.mock("@api/useGetMetadata", () => ({
  default: () => ({ data: {}, isLoading: false }),
}));

vi.mock("@api/useGetProjects", () => ({
  default: () => ({
    data: {
      pages: [
        {
          projects: [
            {
              id: "p1",
              key: "PRJ",
              name: "Demo Project",
              createdOn: "2024-01-01",
              closureState: null,
            },
          ],
          totalRecords: 1,
        },
      ],
    },
    isLoading: false,
    isError: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
  }),
  flattenProjectPages: (data: { pages: { projects: unknown[] }[] } | undefined) =>
    data?.pages.flatMap((p) => p.projects) ?? [],
  getTotalRecords: () => 1,
}));

vi.mock("@hooks/useDebouncedValue", () => ({
  useDebouncedValue: (value: string) => value,
}));

describe("ProjectHub", () => {
  it("renders project hub title with count", () => {
    render(
      <MemoryRouter>
        <ProjectHub />
      </MemoryRouter>,
    );
    expect(screen.getByText("Your Projects (1)")).toBeInTheDocument();
  });
});
