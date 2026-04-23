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
import DeploymentHeader from "@features/project-details/components/deployments/DeploymentHeader";

describe("DeploymentHeader", () => {
  it("should render deployment count and Add Deployment button", () => {
    render(<DeploymentHeader count={3} />);

    expect(screen.getByText("3 deployment environments")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Add Deployment/i }),
    ).toBeInTheDocument();
  });

  it("should use singular form when count is 1", () => {
    render(<DeploymentHeader count={1} />);

    expect(screen.getByText("1 deployment environment")).toBeInTheDocument();
  });

  it("should default count to 0", () => {
    render(<DeploymentHeader />);

    expect(screen.getByText("0 deployment environments")).toBeInTheDocument();
  });

  it("should call onAddClick when Add Deployment button is clicked", () => {
    const onAddClick = vi.fn();
    render(<DeploymentHeader count={2} onAddClick={onAddClick} />);

    fireEvent.click(screen.getByRole("button", { name: /Add Deployment/i }));

    expect(onAddClick).toHaveBeenCalledTimes(1);
  });

  it("should not throw when onAddClick is not provided", () => {
    render(<DeploymentHeader count={1} />);
    expect(() =>
      fireEvent.click(screen.getByRole("button", { name: /Add Deployment/i })),
    ).not.toThrow();
  });

  it("should show skeleton for count when isLoading is true", () => {
    render(<DeploymentHeader count={0} isLoading />);

    expect(
      screen.queryByText("0 deployment environments"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByTestId("deployment-header-skeleton"),
    ).toBeInTheDocument();
  });
});
