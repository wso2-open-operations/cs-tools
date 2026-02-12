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
import CaseDetailsActionRow from "@case-details/CaseDetailsActionRow";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

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
          isError={false}
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Jane Doe")).toBeInTheDocument();
    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
    expect(screen.getByText("Escalate Case")).toBeInTheDocument();
    expect(screen.getByText("Waiting on WSO2")).toBeInTheDocument();
    expect(screen.getByText("Mark as Resolved")).toBeInTheDocument();
    expect(screen.getByText("Close Case")).toBeInTheDocument();
  });

  it("should show skeletons for avatar and engineer but keep play icon, Manage case status, and action buttons when isLoading", () => {
    const { container } = render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer={undefined}
          engineerInitials="--"
          isError={false}
          isLoading={true}
        />
      </ThemeProvider>,
    );
    expect(screen.getByText("Support Engineer")).toBeInTheDocument();
    expect(screen.getByText("Manage case status")).toBeInTheDocument();
    expect(screen.getByText("Escalate Case")).toBeInTheDocument();
    expect(screen.getByText("Waiting on WSO2")).toBeInTheDocument();
    expect(screen.getByText("Mark as Resolved")).toBeInTheDocument();
    expect(screen.getByText("Close Case")).toBeInTheDocument();
    const skeletons = container.querySelectorAll(".MuiSkeleton-root");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render error indicator when isError is true", () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <CaseDetailsActionRow
          assignedEngineer={undefined}
          engineerInitials="--"
          isError={true}
        />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
  });
});
