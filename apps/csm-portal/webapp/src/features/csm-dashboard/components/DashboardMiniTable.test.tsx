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
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import DashboardMiniTable from "@features/csm-dashboard/components/DashboardMiniTable";

function LocationProbe() {
  const location = useLocation();
  return (
    <>
      <div data-testid="location-probe">{location.pathname}</div>
      <div data-testid="location-state-probe">{JSON.stringify(location.state ?? null)}</div>
    </>
  );
}

describe("DashboardMiniTable", () => {
  it("carries a row's state prop through to the navigated destination", () => {
    render(
      <MemoryRouter initialEntries={["/dashboard"]}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <DashboardMiniTable
                isLoading={false}
                emptyMessage="No rows."
                columns={[{ label: "Number" }]}
                rows={[
                  {
                    key: "row-1",
                    href: "/operations/incidents/inc-1",
                    state: { from: "/dashboard" },
                    cells: [<span key="c">INC0000001</span>],
                  },
                ]}
              />
            }
          />
          <Route path="/operations/incidents/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByText("INC0000001"));

    expect(screen.getByTestId("location-probe")).toHaveTextContent(
      "/operations/incidents/inc-1",
    );
    expect(screen.getByTestId("location-state-probe")).toHaveTextContent(
      JSON.stringify({ from: "/dashboard" }),
    );
  });

  it("navigates with no state when a row sets none", () => {
    render(
      <MemoryRouter initialEntries={["/dashboard"]}>
        <Routes>
          <Route
            path="/dashboard"
            element={
              <DashboardMiniTable
                isLoading={false}
                emptyMessage="No rows."
                columns={[{ label: "Number" }]}
                rows={[
                  {
                    key: "row-1",
                    href: "/operations/incidents/inc-1",
                    cells: [<span key="c">INC0000001</span>],
                  },
                ]}
              />
            }
          />
          <Route path="/operations/incidents/:id" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByText("INC0000001"));

    expect(screen.getByTestId("location-state-probe")).toHaveTextContent("null");
  });

  it("renders an onClick row (e.g. a task) without a state prop, since it never navigates", () => {
    const onClick = () => {};
    render(
      <MemoryRouter>
        <DashboardMiniTable
          isLoading={false}
          emptyMessage="No rows."
          columns={[{ label: "Subject" }]}
          rows={[{ key: "row-1", onClick, cells: [<span key="c">Task subject</span>] }]}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText("Task subject")).toBeInTheDocument();
  });
});
