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
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import CaseDetailsDetailsPanel from "@case-details-details/CaseDetailsDetailsPanel";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import type { CaseDetails } from "@features/support/types/cases";
import { formatDateOnly } from "@features/support/utils/support";

const mockCaseDetails: Partial<CaseDetails> = {
  id: "case-001",
  internalId: "INT-1",
  number: "CS0001001",
  createdOn: "2026-01-31 10:45:12",
  updatedOn: "2026-02-10 23:47:57",
  title: "Test case",
  description: "Desc",
  slaResponseTime: "129671000",
  product: null,
  account: { type: null, id: "acc-1", label: "Account" },
  csManager: null,
  assignedEngineer: null,
  project: { id: "p1", label: "Project" },
  type: { id: "1", label: "Incident" },
  deployment: { id: "d1", label: "Production" },
  deployedProduct: null,
  parentCase: null,
  conversation: null,
  issueType: null,
  severity: { id: "60", label: "S0" },
  status: { id: "1", label: "Open" },
  closedOn: null,
  closedBy: null,
  closeNotes: null,
  hasAutoClosed: null,
};

vi.mock("@features/dashboard/utils/casesTable", () => ({
  getStatusColor: () => "warning.main",
  getSeverityColor: () => "#fa7b3f",
}));

function renderDetailsPanel(
  props: {
    data?: Partial<CaseDetails>;
    isError?: boolean;
    isServiceRequest?: boolean;
  } = {},
) {
  return render(
    <MemoryRouter initialEntries={["/projects/p1/support/cases/case-001"]}>
      <Routes>
        <Route
          path="/projects/:projectId/support/cases/:caseId"
          element={
            <ThemeProvider theme={createTheme()}>
              <CaseDetailsDetailsPanel
                data={(props.data ?? mockCaseDetails) as CaseDetails}
                isError={props.isError ?? false}
                isServiceRequest={props.isServiceRequest ?? false}
              />
            </ThemeProvider>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("CaseDetailsDetailsPanel", () => {
  it("should render Case Overview card as first section with title", () => {
    renderDetailsPanel();
    expect(screen.getByText("Case Overview")).toBeInTheDocument();
  });

  it("should render case overview section with API data", () => {
    renderDetailsPanel();
    expect(screen.getByText("Case ID")).toBeInTheDocument();
    expect(screen.getByText("CS0001001")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Open")).toBeInTheDocument();
    expect(screen.getByText("Severity")).toBeInTheDocument();
    expect(screen.getByText("S0")).toBeInTheDocument();
    expect(screen.getByText("Category")).toBeInTheDocument();
    expect(screen.getAllByText("--").length).toBeGreaterThan(0);
  });

  it("should render case overview dates and SLA", () => {
    renderDetailsPanel();
    expect(screen.getByText("Created Date")).toBeInTheDocument();
    expect(
      screen.getByText(formatDateOnly(mockCaseDetails.createdOn)),
    ).toBeInTheDocument();
    expect(screen.getByText("Last Updated")).toBeInTheDocument();
    expect(
      screen.getByText(formatDateOnly(mockCaseDetails.updatedOn)),
    ).toBeInTheDocument();
    expect(screen.getByText("SLA Response Time")).toBeInTheDocument();
    expect(
      screen.getByText((content) =>
        /\d+\s+(?:hour|minute|day|second)s?/.test(content),
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("--").length).toBeGreaterThan(0);
  });

  it("should render Product & Environment card with API data", () => {
    renderDetailsPanel();
    expect(screen.getByText("Product & Environment")).toBeInTheDocument();
    expect(screen.getByText("Product Name")).toBeInTheDocument();
    expect(screen.getByText("Product Version")).toBeInTheDocument();
    expect(screen.getByText("Environment Type")).toBeInTheDocument();
    expect(screen.getByText("Production")).toBeInTheDocument();
  });

  it("should render Customer Information card with API data", () => {
    renderDetailsPanel();
    expect(screen.getByText("Customer Information")).toBeInTheDocument();
    expect(screen.getByText("Organization")).toBeInTheDocument();
    expect(screen.getByText("Account")).toBeInTheDocument();
    expect(screen.getByText("Account Type")).toBeInTheDocument();
    expect(screen.getByText("CS Manager")).toBeInTheDocument();
    expect(screen.getAllByText("Project").length).toBeGreaterThan(0);
  });

  it("should render Closed Case Details section when case is closed", () => {
    renderDetailsPanel({
      data: {
        ...mockCaseDetails,
        status: { id: "3", label: "Closed" },
        closedOn: "2026-02-20 01:34:44",
        closedBy: { id: "bcc4881f", name: "Anuradha Basnayake" },
        closeNotes: "Resolved successfully",
      },
    });
    expect(screen.getByText("Closed Case Details")).toBeInTheDocument();
    expect(screen.getByText("Closed On")).toBeInTheDocument();
    expect(screen.getByText("Closed By")).toBeInTheDocument();
    expect(screen.getByText("Close Notes")).toBeInTheDocument();
    expect(screen.getByText("Anuradha Basnayake")).toBeInTheDocument();
    expect(screen.getByText("Resolved successfully")).toBeInTheDocument();
  });

  it("should not render Closed Case Details when case is not closed", () => {
    renderDetailsPanel();
    expect(screen.queryByText("Closed Case Details")).not.toBeInTheDocument();
  });

  it("should display -- for null or undefined values and hide Assigned Engineer when null", () => {
    renderDetailsPanel({
      data: {
        ...mockCaseDetails,
        product: null,
        deployedProduct: null,
        assignedEngineer: null,
        csManager: null,
        issueType: null,
      },
    });
    const dashes = screen.getAllByText("--");
    expect(dashes.length).toBeGreaterThan(0);
    expect(screen.queryByText("Assigned Engineer")).not.toBeInTheDocument();
  });

  it("should show Error500Page and error message when isError is true", () => {
    renderDetailsPanel({ data: undefined, isError: true });
    expect(screen.getByText("Something Went Wrong")).toBeInTheDocument();
    const img = document.querySelector("img");
    expect(img).toBeInTheDocument();
  });

  it("should render service request overview and request detail fields when isServiceRequest", () => {
    renderDetailsPanel({
      isServiceRequest: true,
      data: {
        ...mockCaseDetails,
        description: "<p>deploy jar</p>",
        severity: null,
        issueType: null,
        changeRequests: [{ id: "cr-1", label: "JAR Deployment" }],
        assignedTeam: { id: "team-1", label: "L2 Team" },
        duration: "2h",
      },
    });
    expect(screen.getByText("Service Request Overview")).toBeInTheDocument();
    expect(screen.getByText("Request number")).toBeInTheDocument();
    expect(screen.getByText("WSO2 Case Id")).toBeInTheDocument();
    expect(screen.getByText("Severity")).toBeInTheDocument();
    expect(screen.getByText("Duration")).toBeInTheDocument();
    expect(screen.getAllByText("Assigned team").length).toBeGreaterThan(0);
    expect(screen.getByText("Description")).toBeInTheDocument();
    expect(screen.getByText("deploy jar")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "View related change request" }),
    ).toHaveAttribute("href", "/projects/p1/support/change-requests/cr-1");
  });
});
