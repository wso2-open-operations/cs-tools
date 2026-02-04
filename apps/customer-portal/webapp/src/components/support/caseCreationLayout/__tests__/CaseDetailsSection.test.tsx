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
import React from "react";
import { describe, expect, it, vi } from "vitest";
import { CaseDetailsSection } from "../CaseDetailsSection";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Chip: ({ label }: any) => <div>{label}</div>,
  ComplexSelect: Object.assign(
    ({ children, value, onChange, disabled, renderValue }: any) => (
      <div data-testid="complex-select-wrapper">
        <div data-testid="complex-select-display">
          {renderValue ? renderValue(value) : value}
        </div>
        <select
          data-testid="complex-select"
          value={value}
          onChange={onChange}
          disabled={disabled}
        >
          {children}
        </select>
      </div>
    ),
    {
      MenuItem: Object.assign(
        ({ value }: any) => <option value={value}>{value}</option>,
        {
          Icon: () => null,
          Text: ({ primary }: any) => primary,
        },
      ),
    },
  ),
  Form: {
    ElementWrapper: ({ children }: any) => <div>{children}</div>,
  },
  FormControl: ({ children, disabled }: any) => (
    <div data-testid="form-control" data-disabled={disabled}>
      {React.Children.map(children, (child: any) =>
        child && React.isValidElement(child)
          ? React.cloneElement(child, {
              disabled: disabled || (child.props as any).disabled,
            } as any)
          : child,
      )}
    </div>
  ),
  Grid: ({ children }: any) => <div data-testid="grid">{children}</div>,
  MenuItem: ({ children, value }: any) => (
    <option value={value}>{children}</option>
  ),
  Paper: ({ children }: any) => <div>{children}</div>,
  Select: ({ children, value, onChange, disabled, renderValue }: any) => (
    <div data-testid="select-wrapper">
      <div data-testid="select-display">
        {renderValue ? renderValue(value) : value}
      </div>
      <select
        data-testid="select"
        value={value}
        onChange={onChange}
        disabled={disabled}
      >
        {children}
      </select>
    </div>
  ),
  TextField: ({ value, onChange, disabled, placeholder, id }: any) => (
    <input
      id={id}
      data-testid={`input-${id}`}
      value={value}
      onChange={onChange}
      disabled={disabled}
      placeholder={placeholder}
    />
  ),
  Typography: ({ children }: any) => <span>{children}</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Pencil: () => <svg data-testid="icon-pencil" />,
  Sparkles: () => <svg data-testid="icon-sparkles" />,
}));

describe("CaseDetailsSection", () => {
  const mockMetadata = {
    issueTypes: ["Outage", "Performance"],
    severityLevels: [
      { id: "S1", label: "Critical", description: "Desc 1" },
      { id: "S2", label: "Medium", description: "Desc 2" },
    ],
  };

  const defaultProps = {
    title: "Old Title",
    setTitle: vi.fn(),
    description: "Old Desc",
    setDescription: vi.fn(),
    issueType: "Performance",
    setIssueType: vi.fn(),
    severity: "S2",
    setSeverity: vi.fn(),
    metadata: mockMetadata,
    isLoading: false,
  };

  it("should render all fields correctly", () => {
    render(<CaseDetailsSection {...defaultProps} />);

    expect(screen.getByText("Case Details")).toBeInTheDocument();
    expect(screen.getByTestId("input-title")).toHaveValue("Old Title");
    expect(screen.getByTestId("input-description")).toHaveValue("Old Desc");
    expect(screen.getByTestId("select")).toHaveValue("Performance");
    expect(screen.getByTestId("complex-select")).toHaveValue("S2");
  });

  it("should call setters when inputs change", () => {
    render(<CaseDetailsSection {...defaultProps} />);

    fireEvent.change(screen.getByTestId("input-title"), {
      target: { value: "New Title" },
    });
    expect(defaultProps.setTitle).toHaveBeenCalledWith("New Title");

    fireEvent.change(screen.getByTestId("input-description"), {
      target: { value: "New Desc" },
    });
    expect(defaultProps.setDescription).toHaveBeenCalledWith("New Desc");

    fireEvent.change(screen.getByTestId("select"), {
      target: { value: "Outage" },
    });
    expect(defaultProps.setIssueType).toHaveBeenCalledWith("Outage");

    fireEvent.change(screen.getByTestId("complex-select"), {
      target: { value: "S1" },
    });
    expect(defaultProps.setSeverity).toHaveBeenCalledWith("S1");
  });

  it("should disable fields when loading", () => {
    render(<CaseDetailsSection {...defaultProps} isLoading={true} />);

    expect(screen.getByTestId("input-title")).toBeDisabled();
    expect(screen.getByTestId("input-description")).toBeDisabled();
    expect(screen.getByTestId("select")).toBeDisabled();
    expect(screen.getByTestId("complex-select")).toBeDisabled();
  });
});
