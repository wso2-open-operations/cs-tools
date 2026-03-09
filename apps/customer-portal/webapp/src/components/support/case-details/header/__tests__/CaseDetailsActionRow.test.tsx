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
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

vi.mock("@api/useGetProjectFilters", () => ({
  default: () => ({
    data: {
      caseStates: [
        { id: "1", label: "Open" },
        { id: "3", label: "Closed" },
        { id: "1003", label: "Waiting On WSO2" },
      ],
    },
  }),
}));

vi.mock("@api/usePatchCase", () => ({
  usePatchCase: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@context/success-banner/SuccessBannerContext", () => ({
  useSuccessBanner: () => ({ showSuccess: vi.fn() }),
}));

vi.mock("@context/error-banner/ErrorBannerContext", () => ({
  useErrorBanner: () => ({ showError: vi.fn() }),
}));

vi.mock("@components/common/error-indicator/ErrorIndicator", () => ({
  default: ({ entityName }: { entityName: string }) => (
    <span data-testid="error-indicator">{entityName}</span>
  ),
}));

describe("CaseDetailsActionRow", () => {
  it("should render engineer name, Support Engineer label and action buttons when not loading", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer="Jane Doe"
          engineerInitials="JD"
          statusLabel="Open"
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
    expect(screen.queryByText("Escalate Case")).not.toBeInTheDocument();
    expect(screen.getByText("Close")).toBeInTheDocument();
  });

  it("should hide engineer section when assignedEngineer is null/undefined", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer={undefined}
          engineerInitials="--"
          statusLabel="Open"
        />
      </ThemeProvider>,
    );
    expect(screen.queryByText("Support Engineer")).not.toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
    expect(screen.getByText("Close")).toBeInTheDocument();
  });

  it("should show skeletons for avatar and engineer but keep play icon, Manage case status when isLoading with engineer", () => {
    const { container } = render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer="Jane Doe"
          engineerInitials="JD"
          statusLabel="Open"
          isLoading={true}
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
    expect(screen.queryByText("Escalate Case")).not.toBeInTheDocument();
    expect(screen.getByText("Close")).toBeInTheDocument();
    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render Open Related Case button for closed status within 2 months", () => {
    const recentClosedOn = "2026-02-01 10:00:00";
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer="Jane Doe"
          engineerInitials="JD"
          statusLabel="Closed"
          closedOn={recentClosedOn}
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Open Related Case")).toBeInTheDocument();
    expect(screen.queryByText("Close")).not.toBeInTheDocument();
  });

  it("should hide Open Related Case button when closed more than 2 months ago", () => {
    const oldClosedOn = "2020-01-01 10:00:00";
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer="Jane Doe"
          engineerInitials="JD"
          statusLabel="Closed"
          closedOn={oldClosedOn}
        />
      </ThemeProvider>,
    );
    expect(screen.queryByText("Open Related Case")).not.toBeInTheDocument();
  });

  it("should call onOpenRelatedCase when Open Related Case button is clicked", () => {
    const onOpenRelatedCase = vi.fn();
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer="Jane Doe"
          engineerInitials="JD"
          statusLabel="Closed"
          closedOn="2026-02-01 10:00:00"
          onOpenRelatedCase={onOpenRelatedCase}
        />
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByText("Open Related Case"));
    expect(onOpenRelatedCase).toHaveBeenCalledTimes(1);
  });
});
