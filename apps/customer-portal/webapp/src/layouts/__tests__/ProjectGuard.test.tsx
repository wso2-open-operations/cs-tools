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
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";
import ProjectGuard from "@layouts/ProjectGuard";
import { ErrorPageProvider } from "@context/error-page/ErrorPageContext";

vi.mock("@api/useGetProjectDetails", () => ({
  default: () => ({
    data: { id: "proj", closureState: "Active" },
    error: null,
    isLoading: false,
  }),
}));

vi.mock("react-router", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router")>();
  return {
    ...actual,
    Outlet: () => <div data-testid="outlet">Child route</div>,
  };
});

describe("ProjectGuard", () => {
  it("renders outlet when project loads successfully", () => {
    render(
      <MemoryRouter initialEntries={["/projects/proj/dashboard"]}>
        <ErrorPageProvider>
          <Routes>
            <Route path="/projects/:projectId/*" element={<ProjectGuard />} />
          </Routes>
        </ErrorPageProvider>
      </MemoryRouter>,
    );
    expect(screen.getByTestId("outlet")).toBeInTheDocument();
  });
});
