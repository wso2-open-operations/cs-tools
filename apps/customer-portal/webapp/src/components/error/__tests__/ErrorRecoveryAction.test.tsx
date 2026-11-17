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

import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ErrorRecoveryAction from "@components/error/ErrorRecoveryAction";

const mockUseInfiniteProjects = vi.fn();
const mockSetLastSelectedProject = vi.fn();

vi.mock("@api/useGetProjects", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@api/useGetProjects")>();
  return { ...actual, default: () => mockUseInfiniteProjects() };
});

vi.mock("@features/settings/utils/settingsStorage", () => ({
  setLastSelectedProject: (p: unknown) => mockSetLastSelectedProject(p),
}));

const PROJECT_A = "00000000-0000-0000-0000-00000000000a";
const PROJECT_B = "00000000-0000-0000-0000-00000000000b";
const FOREIGN = "00000000-0000-0000-0000-0000000000ff";

function projectsResult(projects: { id: string; name: string }[], total = projects.length) {
  return {
    data: { pages: [{ projects, totalRecords: total }], pageParams: [0] },
    isLoading: false,
    isError: false,
    isFetched: true,
  };
}

function LocationProbe() {
  const { pathname } = useLocation();
  return <div data-testid="location">{pathname}</div>;
}

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/projects/:projectId/*"
          element={
            <>
              <ErrorRecoveryAction />
              <LocationProbe />
            </>
          }
        />
        <Route
          path="*"
          element={
            <>
              <ErrorRecoveryAction />
              <LocationProbe />
            </>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ErrorRecoveryAction", () => {
  beforeEach(() => {
    mockUseInfiniteProjects.mockReset();
    mockSetLastSelectedProject.mockReset();
  });

  it("offers a way back to the project when a sub-resource of an accessible project fails", () => {
    mockUseInfiniteProjects.mockReturnValue(
      projectsResult([
        { id: PROJECT_A, name: "Project A" },
        { id: PROJECT_B, name: "Project B" },
      ]),
    );
    renderAt(`/projects/${PROJECT_A}/support/cases/case-1`);

    fireEvent.click(screen.getByRole("button", { name: "Back to project" }));
    expect(screen.getByTestId("location")).toHaveTextContent(
      `/projects/${PROJECT_A}/dashboard`,
    );
    expect(mockSetLastSelectedProject).toHaveBeenCalledWith({ id: PROJECT_A });
  });

  it("does not offer back-to-project loop on the project's own dashboard", () => {
    mockUseInfiniteProjects.mockReturnValue(
      projectsResult([
        { id: PROJECT_A, name: "Project A" },
        { id: PROJECT_B, name: "Project B" },
      ]),
    );
    renderAt(`/projects/${PROJECT_A}/dashboard`);

    expect(screen.queryByRole("button", { name: "Back to project" })).toBeNull();
    expect(
      screen.getByRole("button", { name: "Go to your projects" }),
    ).toBeInTheDocument();
  });

  it("sends a single-project user with a foreign project id to their own project", () => {
    mockUseInfiniteProjects.mockReturnValue(
      projectsResult([{ id: PROJECT_A, name: "Project A" }]),
    );
    renderAt(`/projects/${FOREIGN}/dashboard`);

    fireEvent.click(screen.getByRole("button", { name: "Go to Project A" }));
    expect(screen.getByTestId("location")).toHaveTextContent(
      `/projects/${PROJECT_A}/dashboard`,
    );
    expect(mockSetLastSelectedProject).toHaveBeenCalledWith({ id: PROJECT_A });
  });

  it("sends a multi-project user with a foreign project id to the project hub", () => {
    mockUseInfiniteProjects.mockReturnValue(
      projectsResult([
        { id: PROJECT_A, name: "Project A" },
        { id: PROJECT_B, name: "Project B" },
      ]),
    );
    renderAt(`/projects/${FOREIGN}/dashboard`);

    fireEvent.click(screen.getByRole("button", { name: "Go to your projects" }));
    expect(screen.getByTestId("location")).toHaveTextContent("/");
    expect(mockSetLastSelectedProject).not.toHaveBeenCalled();
  });

  it("treats a route without a project id like an inaccessible project", () => {
    mockUseInfiniteProjects.mockReturnValue(
      projectsResult([{ id: PROJECT_A, name: "Project A" }]),
    );
    renderAt("/some/missing/page");

    expect(
      screen.getByRole("button", { name: "Go to Project A" }),
    ).toBeInTheDocument();
  });

  it("does not guess a single project from a partial first page", () => {
    mockUseInfiniteProjects.mockReturnValue(
      projectsResult([{ id: PROJECT_A, name: "Project A" }], 25),
    );
    renderAt(`/projects/${FOREIGN}/dashboard`);

    expect(
      screen.getByRole("button", { name: "Go to your projects" }),
    ).toBeInTheDocument();
  });

  it("falls back to the project hub when the project list fails to load", () => {
    mockUseInfiniteProjects.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      isFetched: true,
    });
    renderAt(`/projects/${FOREIGN}/dashboard`);

    expect(
      screen.getByRole("button", { name: "Go to your projects" }),
    ).toBeInTheDocument();
  });

  it("renders nothing while the project list is loading", () => {
    mockUseInfiniteProjects.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      isFetched: false,
    });
    renderAt(`/projects/${FOREIGN}/dashboard`);

    expect(screen.queryByRole("button")).toBeNull();
  });

  it("renders nothing when the projects query is disabled (not signed in)", () => {
    mockUseInfiniteProjects.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      isFetched: false,
    });
    renderAt("/404");

    expect(screen.queryByRole("button")).toBeNull();
  });
});
