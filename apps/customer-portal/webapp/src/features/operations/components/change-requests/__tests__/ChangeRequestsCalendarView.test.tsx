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
import { describe, expect, it } from "vitest";
import ChangeRequestsCalendarView from "@features/operations/components/change-requests/ChangeRequestsCalendarView";

describe("ChangeRequestsCalendarView", () => {
  it("renders calendar skeleton while loading", () => {
    const { container } = render(
      <ChangeRequestsCalendarView changeRequests={[]} isLoading />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  const authorizeItem = {
    id: "cr-authorize",
    number: "CHG0001234",
    title: "Rotate the gateway certificates",
    startDate: "2031-03-15 09:00:00",
    endDate: "2031-03-15 11:00:00",
    state: { id: "-3", label: "Authorize" },
  } as never;

  it("shows a change request waiting in Authorize on the day it is planned, with its state", () => {
    render(<ChangeRequestsCalendarView changeRequests={[authorizeItem]} isLoading={false} />);
    const entry = screen.getByRole("button", {
      name: "CHG0001234: Rotate the gateway certificates - Authorize",
    });
    expect(entry).toBeInTheDocument();
    expect(entry).toHaveTextContent("Rotate the gateway certificates");
  });

  it("names Authorize in the legend, before the project's filters have arrived and after", () => {
    const { unmount } = render(
      <ChangeRequestsCalendarView changeRequests={[authorizeItem]} isLoading={false} />,
    );
    expect(screen.getByText("Authorize")).toBeInTheDocument();
    expect(screen.queryByText("New")).not.toBeInTheDocument();
    expect(screen.queryByText("Assess")).not.toBeInTheDocument();
    unmount();

    // The page passes the filters' own state list ({id, label} items).
    const legendStates = [
      { id: "-3", label: "Authorize" },
      { id: "5", label: "Customer Approval" },
    ];
    render(
      <ChangeRequestsCalendarView
        changeRequests={[authorizeItem]}
        isLoading={false}
        legendStates={legendStates}
      />,
    );
    expect(screen.getByText("Authorize")).toBeInTheDocument();
    expect(screen.getByText("Customer Approval")).toBeInTheDocument();
  });
});
