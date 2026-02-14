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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import { CaseDetailsSection } from "@components/support/case-creation-layout/form-sections/case-details-section/CaseDetailsSection";

vi.mock("@utils/casesTable", () => ({
  getSeverityColor: () => "error.main",
}));

const defaultFilters = {
  issueTypes: [{ id: "bug", label: "Bug" }],
  severities: [
    { id: "1", label: "S1", description: "High" },
    { id: "2", label: "S2" },
  ],
};

function renderSection(
  props: Partial<Parameters<typeof CaseDetailsSection>[0]> = {},
) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailsSection
        title=""
        setTitle={vi.fn()}
        description=""
        setDescription={vi.fn()}
        issueType=""
        setIssueType={vi.fn()}
        severity=""
        setSeverity={vi.fn()}
        filters={defaultFilters}
        {...props}
      />
    </ThemeProvider>,
  );
}

describe("CaseDetailsSection", () => {
  it("should render section title Case Details", () => {
    renderSection();
    expect(screen.getByText("Case Details")).toBeInTheDocument();
  });

  it("should render Title label and Generated from chat chip", () => {
    renderSection();
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("Generated from chat")).toBeInTheDocument();
  });

  it("should render Description label and From conversation chip", () => {
    renderSection();
    expect(screen.getByText("Description")).toBeInTheDocument();
    expect(screen.getByText("From conversation")).toBeInTheDocument();
  });

  it("should render Issue Type and Severity Level labels", () => {
    renderSection();
    expect(screen.getByText("Issue Type")).toBeInTheDocument();
    expect(screen.getByText("Severity Level")).toBeInTheDocument();
  });

  it("should render Novera reference text", () => {
    renderSection();
    expect(
      screen.getByText(/This includes all the information you shared with Novera/i),
    ).toBeInTheDocument();
  });

  it("should render edit button with accessible label", () => {
    renderSection();
    expect(
      screen.getByRole("button", { name: /edit case details/i }),
    ).toBeInTheDocument();
  });

  it("should display title and description when provided", () => {
    renderSection({
      title: "My issue",
      description: "Steps to reproduce",
    });
    expect(screen.getByDisplayValue("My issue")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Steps to reproduce")).toBeInTheDocument();
  });

  it("should render description textarea with test id", () => {
    renderSection();
    expect(
      screen.getByTestId("case-description-editor"),
    ).toBeInTheDocument();
  });

  it("should render issue type and severity section with AI chips", () => {
    renderSection();
    expect(screen.getByText("AI classified")).toBeInTheDocument();
    expect(screen.getByText("AI assessed")).toBeInTheDocument();
    expect(screen.getByText("Select Issue Type...")).toBeInTheDocument();
    expect(screen.getByText("Select Severity Level...")).toBeInTheDocument();
  });
});
