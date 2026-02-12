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
import CaseDetailsHeader from "@case-details/CaseDetailsHeader";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

vi.mock("@utils/casesTable", () => ({
  getSeverityColor: vi.fn(() => "text.secondary"),
}));

vi.mock("@components/common/error-indicator/ErrorIndicator", () => ({
  default: ({ entityName }: { entityName: string }) => (
    <span>{entityName}</span>
  ),
}));

describe("CaseDetailsHeader", () => {
  const defaultProps = {
    caseNumber: "CUPRSUB-101",
    title: "Test case title",
    severityLabel: "S1",
    statusLabel: "Open",
    statusChipIcon: <span data-testid="status-chip-icon" />,
    statusChipSx: {},
    isError: false,
  };

  it("should render case number, severity, status chip and title when not loading", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsHeader {...defaultProps} />
      </ThemeProvider>,
    );
    expect(screen.getByText("CUPRSUB-101")).toBeInTheDocument();
    expect(screen.getByText("Test case title")).toBeInTheDocument();
    expect(screen.getByText("S1")).toBeInTheDocument();
    expect(screen.getByText("Open")).toBeInTheDocument();
    expect(screen.getByTestId("status-chip-icon")).toBeInTheDocument();
  });

  it("should render skeletons when isLoading is true", () => {
    const { container } = render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsHeader {...defaultProps} isLoading={true} />
      </ThemeProvider>,
    );
    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
    expect(screen.queryByText("CUPRSUB-101")).not.toBeInTheDocument();
    expect(screen.queryByText("Test case title")).not.toBeInTheDocument();
  });

  it("should render error indicator in value fields only when isError is true", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsHeader
          {...defaultProps}
          caseNumber={undefined}
          title={undefined}
          isError={true}
        />
      </ThemeProvider>,
    );
    const errorIndicators = screen.getAllByText("case details");
    expect(errorIndicators.length).toBeGreaterThanOrEqual(1);
  });
});
