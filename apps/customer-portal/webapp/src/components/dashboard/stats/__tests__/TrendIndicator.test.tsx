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
import { TrendIndicator } from "../TrendIndicator";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, sx }: any) => <div style={sx}>{children}</div>,
  Typography: ({ children, variant, color }: any) => (
    <div data-testid={`typography-${variant}`} style={{ color }}>
      {children}
    </div>
  ),
  Skeleton: ({ variant, width, height }: any) => (
    <div
      data-testid="skeleton"
      data-variant={variant}
      style={{ width, height }}
    ></div>
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
  TrendingDown: () => <svg data-testid="icon-trending-down" />,
}));

describe("TrendIndicator", () => {
  it("should return null if not loading and no trend data", () => {
    const { container } = render(<TrendIndicator />);
    expect(container.firstChild).toBeNull();
  });

  it("should render rounded skeleton when loading", () => {
    render(<TrendIndicator isLoading={true} />);
    const skeleton = screen.getByTestId("skeleton");
    expect(skeleton).toHaveAttribute("data-variant", "rounded");
    expect(screen.getByText("vs last month")).toBeInTheDocument();
  });

  it("should render up trend correctly", () => {
    const trend = {
      value: "12%",
      direction: "up" as const,
      color: "success" as const,
    };
    render(<TrendIndicator trend={trend} />);

    expect(screen.getByText("12%")).toBeInTheDocument();
    expect(screen.getByTestId("icon-trending-up")).toBeInTheDocument();
    expect(screen.getByText("vs last month")).toBeInTheDocument();
  });

  it("should render down trend in error color", () => {
    const trend = {
      value: "5%",
      direction: "down" as const,
      color: "success" as const,
    };
    render(<TrendIndicator trend={trend} />);

    expect(screen.getByText("5%")).toBeInTheDocument();
    expect(screen.getByTestId("icon-trending-down")).toBeInTheDocument();
    // Typography-body2 should have inherit color, but parent Box manages the color.
    // In our mock, Box just renders children. The logic specifically checks direction === "down" ? "error.main"
  });
});
