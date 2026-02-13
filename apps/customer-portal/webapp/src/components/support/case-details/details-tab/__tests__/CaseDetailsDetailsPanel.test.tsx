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
import CaseDetailsDetailsPanel from "../CaseDetailsDetailsPanel";
import { mockCaseDetails } from "@models/mockData";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

vi.mock("@utils/casesTable", () => ({
  getStatusColor: () => "warning.main",
  getSeverityColor: () => "#fa7b3f",
}));

function renderDetailsPanel(props: {
  data?: typeof mockCaseDetails;
  isError?: boolean;
} = {}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsDetailsPanel
        data={props.data ?? mockCaseDetails}
        isError={props.isError ?? false}
      />
    </ThemeProvider>,
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

  it("should render case overview dates and SLA and assigned engineer", () => {
    renderDetailsPanel();
    expect(screen.getByText("Created Date")).toBeInTheDocument();
    expect(screen.getByText("2026-01-31 10:45:12")).toBeInTheDocument();
    expect(screen.getByText("Last Updated")).toBeInTheDocument();
    expect(screen.getByText("2026-02-10 23:47:57")).toBeInTheDocument();
    expect(screen.getByText("SLA Response Time")).toBeInTheDocument();
    expect(screen.getByText(/\d+\s+(?:hours|minutes|days|seconds)/)).toBeInTheDocument();
    expect(screen.getAllByText("Assigned Engineer").length).toBeGreaterThan(0);
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
    expect(screen.getByText("Customer 3i")).toBeInTheDocument();
    expect(screen.getByText("Account Type")).toBeInTheDocument();
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByText("Customer Portal – Subscription")).toBeInTheDocument();
    expect(screen.getByText("CS Manager")).toBeInTheDocument();
  });

  it("should display -- for null or undefined values", () => {
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
  });

  it("should show ErrorStateIcon and projecthub text when isError is true", () => {
    renderDetailsPanel({ data: undefined, isError: true });
    expect(screen.getByText("Something Went Wrong")).toBeInTheDocument();
    expect(screen.getByText("projecthub")).toBeInTheDocument();
    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});
