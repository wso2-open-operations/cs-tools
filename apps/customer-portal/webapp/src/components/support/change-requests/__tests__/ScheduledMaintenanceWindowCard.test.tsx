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
import { describe, expect, it, vi } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ScheduledMaintenanceWindowCard from "../ScheduledMaintenanceWindowCard";
import type { ChangeRequestDetails } from "@models/responses";

const mockChangeRequest: ChangeRequestDetails = {
  id: "cr-001",
  number: "CHG0038388",
  title: "Test CR",
  project: { id: "p1", label: "Project", number: null },
  case: { id: "c1", label: "", number: "CS0438888" },
  deployment: null,
  deployedProduct: null,
  product: null,
  assignedEngineer: null,
  assignedTeam: { id: "t1", label: "Devops" },
  startDate: "2026-02-28 15:30:50",
  endDate: "2026-03-06 01:30:00",
  duration: null,
  hasServiceOutage: false,
  impact: { id: "3", label: "3 - Low" },
  state: { id: "-2", label: "Scheduled" },
  type: { id: "normal", label: "Normal" },
  createdOn: "2026-02-26 02:02:32",
  updatedOn: "2026-03-05 11:34:36",
  description: null,
  createdBy: "user@example.com",
  justification: null,
  impactDescription: null,
  serviceOutage: null,
  communicationPlan: null,
  rollbackPlan: null,
  testPlan: null,
  hasCustomerApproved: false,
  hasCustomerReviewed: false,
  approvedBy: null,
  approvedOn: null,
};

vi.mock("@api/usePatchChangeRequest", () => ({
  usePatchChangeRequest: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

describe("ScheduledMaintenanceWindowCard", () => {
  it("renders card with title and maintenance window fields", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <ScheduledMaintenanceWindowCard changeRequest={mockChangeRequest} />
      </ThemeProvider>,
    );

    expect(
      screen.getByText("Scheduled Maintenance Window"),
    ).toBeInTheDocument();
    expect(screen.getByText("Planned Start")).toBeInTheDocument();
    expect(screen.getByText("Planned End")).toBeInTheDocument();
    expect(screen.getByText("Duration")).toBeInTheDocument();
  });

  it("shows edit button near planned start", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <ScheduledMaintenanceWindowCard changeRequest={mockChangeRequest} />
      </ThemeProvider>,
    );

    const editButton = screen.getByRole("button", {
      name: /edit planned start/i,
    });
    expect(editButton).toBeInTheDocument();
  });

  it("opens edit mode when edit button is clicked", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <ScheduledMaintenanceWindowCard changeRequest={mockChangeRequest} />
      </ThemeProvider>,
    );

    const editButton = screen.getByRole("button", {
      name: /edit planned start/i,
    });
    fireEvent.click(editButton);

    expect(screen.getByRole("button", { name: /^Update$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Cancel$/i })).toBeInTheDocument();
  });
});
