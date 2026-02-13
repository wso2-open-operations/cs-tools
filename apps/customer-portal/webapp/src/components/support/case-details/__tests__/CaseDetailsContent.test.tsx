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
import CaseDetailsContent from "@components/support/case-details/CaseDetailsContent";
import { mockCaseDetails } from "@models/mockData";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

vi.mock("@utils/casesTable", () => ({
  getStatusColor: vi.fn(() => "warning.main"),
  getSeverityColor: vi.fn(() => "error.main"),
}));

vi.mock("@utils/support", () => ({
  resolveColorFromTheme: vi.fn(() => "#fa7b3f"),
  getStatusIcon: vi.fn(() => () => <span data-testid="status-icon" />),
  getStatusIconElement: vi.fn(() => <span data-testid="status-icon" />),
  formatValue: vi.fn((value: unknown) => (value == null ? "--" : String(value))),
}));

describe("CaseDetailsContent", () => {
  const theme = createTheme();
  const onBack = vi.fn();

  it("should render skeleton when loading", () => {
    render(
      <ThemeProvider theme={theme}>
        <CaseDetailsContent
          data={undefined}
          isLoading={true}
          isError={false}
          caseId="case-001"
          onBack={onBack}
        />
      </ThemeProvider>,
    );

    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
    // CaseDetailsSkeleton renders multiple Skeleton elements
    const skeletons = document.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render case details when data is provided", () => {
    render(
      <ThemeProvider theme={theme}>
        <CaseDetailsContent
          data={mockCaseDetails}
          isLoading={false}
          isError={false}
          caseId={mockCaseDetails.id}
          onBack={onBack}
        />
      </ThemeProvider>,
    );

    expect(screen.getByText(mockCaseDetails.number!)).toBeInTheDocument();
    expect(screen.getByText(mockCaseDetails.title!)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: /details/i }));
    expect(screen.getByText("Details appear here.")).toBeInTheDocument();
  });

  it("should display -- for missing values", () => {
    const partialData = {
      ...mockCaseDetails,
      product: null,
      csManager: null,
    };

    render(
      <ThemeProvider theme={theme}>
        <CaseDetailsContent
          data={partialData}
          isLoading={false}
          isError={false}
          caseId={mockCaseDetails.id}
          onBack={onBack}
        />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByRole("tab", { name: /details/i }));
    const dashes = screen.getAllByText("--");
    expect(dashes.length).toBeGreaterThan(0);
  });

  it("should call onBack when Back button is clicked", () => {
    render(
      <ThemeProvider theme={theme}>
        <CaseDetailsContent
          data={mockCaseDetails}
          isLoading={false}
          isError={false}
          caseId={mockCaseDetails.id}
          onBack={onBack}
        />
      </ThemeProvider>,
    );

    fireEvent.click(screen.getByText("Back to Support Center"));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it("should hide content above sub nav when Focus Mode is toggled", () => {
    render(
      <ThemeProvider theme={theme}>
        <CaseDetailsContent
          data={mockCaseDetails}
          isLoading={false}
          isError={false}
          caseId={mockCaseDetails.id}
          onBack={onBack}
        />
      </ThemeProvider>,
    );

    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
    expect(screen.getByText(mockCaseDetails.title!)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /focus mode/i }));
    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
    expect(screen.queryByText(mockCaseDetails.title!)).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /exit focus mode/i }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /exit focus mode/i }));
    expect(screen.getByText("Back to Support Center")).toBeInTheDocument();
    expect(screen.getByText(mockCaseDetails.title!)).toBeInTheDocument();
  });

  it("should render case action buttons in CaseDetailsActionRow", () => {
    render(
      <ThemeProvider theme={theme}>
        <CaseDetailsContent
          data={mockCaseDetails}
          isLoading={false}
          isError={false}
          caseId={mockCaseDetails.id}
          onBack={onBack}
        />
      </ThemeProvider>,
    );

    expect(screen.getByText("Escalate Case")).toBeInTheDocument();
    expect(screen.getByText("Waiting on WSO2")).toBeInTheDocument();
    expect(screen.getByText("Mark as Resolved")).toBeInTheDocument();
    expect(screen.getByText("Close Case")).toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
  });
});
