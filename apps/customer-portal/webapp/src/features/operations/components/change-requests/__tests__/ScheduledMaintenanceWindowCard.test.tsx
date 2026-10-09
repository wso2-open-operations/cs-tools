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

import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import ScheduledMaintenanceWindowCard from "../ScheduledMaintenanceWindowCard";
import type { ChangeRequestDetails } from "@features/operations/types/changeRequests";

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

  it("does not render an inline edit/propose control", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <ScheduledMaintenanceWindowCard changeRequest={mockChangeRequest} />
      </ThemeProvider>,
    );

    expect(
      screen.queryByRole("button", {
        name: /propose new implementation time/i,
      }),
    ).not.toBeInTheDocument();
  });

  describe("duration calculation", () => {
    it("derives and formats duration when duration field is null but start and end dates are present", () => {
      render(
        <ThemeProvider theme={createTheme()}>
          <ScheduledMaintenanceWindowCard
            changeRequest={{
              ...mockChangeRequest,
              startDate: "2026-10-08 13:30:00",
              endDate: "2026-10-08 15:00:00",
              duration: null,
            }}
          />
        </ThemeProvider>,
      );

      expect(screen.getByText("1 hour 30 minutes")).toBeInTheDocument();
    });

    it("uses explicit duration when provided", () => {
      render(
        <ThemeProvider theme={createTheme()}>
          <ScheduledMaintenanceWindowCard
            changeRequest={{
              ...mockChangeRequest,
              duration: "90",
            }}
          />
        </ThemeProvider>,
      );

      expect(screen.getByText("1 hour 30 minutes")).toBeInTheDocument();
    });

    it("displays 'Not available' when neither duration nor start/end dates are available", () => {
      render(
        <ThemeProvider theme={createTheme()}>
          <ScheduledMaintenanceWindowCard
            changeRequest={{
              ...mockChangeRequest,
              startDate: "",
              endDate: "",
              duration: null,
            }}
          />
        </ThemeProvider>,
      );

      const durationSection = screen.getByText("Duration").parentElement!;
      expect(within(durationSection).getByText("Not available")).toBeInTheDocument();
    });

    it("displays 'Not available' when endDate is before startDate", () => {
      render(
        <ThemeProvider theme={createTheme()}>
          <ScheduledMaintenanceWindowCard
            changeRequest={{
              ...mockChangeRequest,
              startDate: "2026-10-08 15:00:00",
              endDate: "2026-10-08 13:30:00",
              duration: null,
            }}
          />
        </ThemeProvider>,
      );

      const durationSection = screen.getByText("Duration").parentElement!;
      expect(within(durationSection).getByText("Not available")).toBeInTheDocument();
    });
  });

  describe("a time the customer proposed", () => {
    const approval = { id: "5", label: "Customer Approval" };
    const renderCard = (overrides: Partial<ChangeRequestDetails>) =>
      render(
        <ThemeProvider theme={createTheme()}>
          <ScheduledMaintenanceWindowCard changeRequest={{ ...mockChangeRequest, ...overrides }} />
        </ThemeProvider>,
      );
    const proposal = (answer: "pending" | "agreed" | "disagreed" | "unanswered") => ({
      startDate: "2026-03-02 09:00:00",
      answer,
    });

    it("shows the proposed start under the planned start while WSO2 has not answered, and keeps the planned window as it is", () => {
      renderCard({ state: approval, customerProposal: proposal("pending") });
      expect(screen.getByText("Planned Maintenance Window")).toBeInTheDocument();
      expect(screen.getByText(/^Proposed start: .*March 2, 2026.* \(waiting for WSO2\)$/)).toBeInTheDocument();
      expect(screen.getByText(/February 28, 2026/)).toBeInTheDocument();
    });

    it("says WSO2 accepted the proposed start once it did, and nothing is pending any more", () => {
      renderCard({
        startDate: "2026-03-02 09:00:00",
        endDate: "2026-03-02 11:00:00",
        customerProposal: proposal("agreed"),
      });
      expect(screen.getByText("Scheduled Maintenance Window")).toBeInTheDocument();
      expect(screen.getByText(/^WSO2 accepted the proposed start, .*March 2, 2026.*\.$/)).toBeInTheDocument();
      expect(screen.queryByText(/waiting for WSO2/)).not.toBeInTheDocument();
    });

    it("does not say WSO2 accepted it when the change request is back in Customer Approval (the customers are asked again)", () => {
      renderCard({ state: approval, customerProposal: proposal("agreed") });
      expect(screen.getByText("Planned Maintenance Window")).toBeInTheDocument();
      expect(screen.queryByText(/WSO2 accepted/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Proposed start:/)).not.toBeInTheDocument();
    });

    it("says nothing for a proposal that was declined, never answered or is not there", () => {
      for (const customerProposal of [proposal("disagreed"), proposal("unanswered"), null, undefined]) {
        const { unmount } = renderCard({ state: approval, customerProposal });
        expect(screen.queryByText(/Proposed start:/)).not.toBeInTheDocument();
        expect(screen.queryByText(/WSO2 accepted/)).not.toBeInTheDocument();
        unmount();
      }
    });

    it("does not show a pending answer left on a change request that is no longer in Customer Approval", () => {
      renderCard({ customerProposal: proposal("pending") });
      expect(screen.queryByText(/Proposed start:/)).not.toBeInTheDocument();
    });
  });
});
