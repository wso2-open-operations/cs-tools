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
import { describe, it, expect } from "vitest";
import { StatCard } from "@features/updates/components/stat-card-row/StatCard";
import { Activity } from "@wso2/oxygen-ui-icons-react";
import { NULL_PLACEHOLDER } from "@features/updates/utils/updates";

describe("StatCard", () => {
  const defaultProps = {
    label: "Test Stat",
    value: 100,
    icon: <Activity />,
    iconColor: "primary" as const,
    tooltipText: "Tooltip text",
  };

  it("renders the label and value correctly", () => {
    render(<StatCard {...defaultProps} />);

    expect(screen.getByText("Test Stat")).toBeDefined();
    expect(screen.getByText("100")).toBeDefined();
  });

  it("renders a skeleton when loading", () => {
    render(<StatCard {...defaultProps} isLoading={true} />);
    expect(screen.queryByText("100")).toBeNull();
  });

  it("renders a placeholder when there is an error", () => {
    render(<StatCard {...defaultProps} isError={true} />);

    expect(screen.queryByText("100")).toBeNull();
    // ErrorIndicator should be rendered
    expect(screen.getByTestId("error-indicator")).toBeDefined();
  });

  it("renders extra content if provided", () => {
    render(
      <StatCard
        {...defaultProps}
        extraContent={<div data-testid="extra">Extra Info</div>}
      />,
    );

    expect(screen.getByTestId("extra")).toBeDefined();
    expect(screen.getByText("Extra Info")).toBeDefined();
  });

  it("renders the NULL_PLACEHOLDER when value is not provided", () => {
    const { value, ...propsWithoutValue } = defaultProps;
    render(<StatCard {...propsWithoutValue} />);

    expect(screen.getByText(NULL_PLACEHOLDER)).toBeDefined();
  });
});
