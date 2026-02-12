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
import CaseDetailField from "@case-details/CaseDetailField";
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";

vi.mock("@components/common/error-indicator/ErrorIndicator", () => ({
  default: ({ entityName }: { entityName: string }) => (
    <span data-testid="error-indicator">{entityName}</span>
  ),
}));

function renderCaseDetailField(props: {
  label: string;
  value?: string | null;
  isLoading?: boolean;
  isError?: boolean;
}) {
  return render(
    <ThemeProvider theme={createTheme()}>
      <CaseDetailField
        label={props.label}
        value={props.value}
        isLoading={props.isLoading ?? false}
        isError={props.isError ?? false}
      />
    </ThemeProvider>,
  );
}

describe("CaseDetailField", () => {
  it("should render label and value when not loading and not error", () => {
    renderCaseDetailField({ label: "Product", value: "Choreo" });

    expect(screen.getByText("Product")).toBeInTheDocument();
    expect(screen.getByText("Choreo")).toBeInTheDocument();
  });

  it("should render -- for null value", () => {
    renderCaseDetailField({ label: "CS Manager", value: null });

    expect(screen.getByText("CS Manager")).toBeInTheDocument();
    expect(screen.getByText("--")).toBeInTheDocument();
  });

  it("should render -- for undefined value", () => {
    renderCaseDetailField({ label: "Account", value: undefined });

    expect(screen.getByText("--")).toBeInTheDocument();
  });

  it("should render error indicator when isError is true", () => {
    renderCaseDetailField({
      label: "Description",
      value: "Some text",
      isError: true,
    });

    expect(screen.getByText("Description")).toBeInTheDocument();
    expect(screen.getByTestId("error-indicator")).toBeInTheDocument();
    expect(screen.getByTestId("error-indicator")).toHaveTextContent(
      "case details",
    );
  });

  it("should render loading placeholder when isLoading is true", () => {
    renderCaseDetailField({
      label: "Created",
      value: "2026-01-01",
      isLoading: true,
    });

    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.queryByText("2026-01-01")).not.toBeInTheDocument();
  });
});
