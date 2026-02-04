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
import { BasicInformationSection } from "../BasicInformationSection";
import React from "react";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Chip: ({ label }: any) => <div>{label}</div>,
  FormControl: ({ children, disabled }: any) => (
    <div data-testid="form-control" data-disabled={disabled}>
      {vi.isMockFunction(children)
        ? children
        : React.Children.map(children, (child: any) =>
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
        value={value}
        onChange={onChange}
        disabled={disabled}
        data-testid="select"
      >
        {children}
      </select>
    </div>
  ),
  Typography: ({ children }: any) => <span>{children}</span>,
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Pencil: () => <svg data-testid="icon-pencil" />,
  Sparkles: () => <svg data-testid="icon-sparkles" />,
}));

describe("BasicInformationSection", () => {
  const mockMetadata = {
    projects: ["Project 1", "Project 2"],
    products: ["Product 1", "Product 2"],
    deploymentTypes: ["Dev", "Prod"],
  };

  const defaultProps = {
    project: "Project 1",
    setProject: vi.fn(),
    product: "Product 1",
    setProduct: vi.fn(),
    deployment: "Dev",
    setDeployment: vi.fn(),
    metadata: mockMetadata,
    isLoading: false,
  };

  it("should render all fields correctly", () => {
    render(<BasicInformationSection {...defaultProps} />);

    expect(screen.getByText("Basic Information")).toBeInTheDocument();
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByText("Product & Version")).toBeInTheDocument();
    expect(screen.getByText("Deployment Type")).toBeInTheDocument();

    const selects = screen.getAllByTestId("select");
    expect(selects).toHaveLength(3);
    expect(selects[0]).toHaveValue("Project 1");
    expect(selects[1]).toHaveValue("Product 1");
    expect(selects[2]).toHaveValue("Dev");
  });

  it("should call setter functions when values change", () => {
    render(<BasicInformationSection {...defaultProps} />);

    const selects = screen.getAllByTestId("select");

    fireEvent.change(selects[0], { target: { value: "Project 2" } });
    expect(defaultProps.setProject).toHaveBeenCalledWith("Project 2");

    fireEvent.change(selects[1], { target: { value: "Product 2" } });
    expect(defaultProps.setProduct).toHaveBeenCalledWith("Product 2");

    fireEvent.change(selects[2], { target: { value: "Prod" } });
    expect(defaultProps.setDeployment).toHaveBeenCalledWith("Prod");
  });

  it("should disable selects when loading", () => {
    render(<BasicInformationSection {...defaultProps} isLoading={true} />);

    const selects = screen.getAllByTestId("select");
    selects.forEach((select) => {
      expect(select).toBeDisabled();
    });
  });
});
