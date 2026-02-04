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
import SubscriptionDetails from "../SubscriptionDetails";
import { SUBSCRIPTION_STATUS } from "@/constants/projectDetailsConstants";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Chip: ({ label, color }: any) => (
    <div data-testid="chip" data-color={color}>
      {label}
    </div>
  ),
  LinearProgress: ({ value, color }: any) => (
    <div data-testid="linear-progress" data-value={value} data-color={color} />
  ),
  Skeleton: () => <div data-testid="skeleton" />,
  colors: {
    blue: { 700: "#1976d2" },
    purple: { 400: "#ab47bc" },
  },
}));

// Mock utils
vi.mock("@/utils/projectStats", () => ({
  getSubscriptionStatus: vi.fn((date) =>
    date === "expired-date"
      ? SUBSCRIPTION_STATUS.EXPIRED
      : SUBSCRIPTION_STATUS.ACTIVE,
  ),
  getSubscriptionColor: vi.fn((status) =>
    status === SUBSCRIPTION_STATUS.EXPIRED ? "error" : "success",
  ),
  calculateProgress: vi.fn((_start, end) =>
    end === "expired-date" ? 100 : 50,
  ),
}));

describe("SubscriptionDetails", () => {
  const defaultProps = {
    startDate: "Jan 1, 2023",
    endDate: "Dec 31, 2024",
    isLoading: false,
  };

  it("should render subscription details correctly", () => {
    render(<SubscriptionDetails {...defaultProps} />);

    expect(screen.getByText("Subscription Period")).toBeInTheDocument();
    expect(screen.getByText(SUBSCRIPTION_STATUS.ACTIVE)).toBeInTheDocument();
    expect(screen.getByText("Start")).toBeInTheDocument();
    expect(screen.getByText("Jan 1, 2023")).toBeInTheDocument();
    expect(screen.getByText("End")).toBeInTheDocument();
    expect(screen.getByText("Dec 31, 2024")).toBeInTheDocument();
    expect(screen.getByText(/Expires on/)).toBeInTheDocument();
  });

  it("should render linear progress with correct value", () => {
    render(<SubscriptionDetails {...defaultProps} />);
    const progress = screen.getByTestId("linear-progress");
    expect(progress).toBeInTheDocument();
    expect(progress).toHaveAttribute("data-value", "50");
    expect(progress).toHaveAttribute("data-color", "success");
  });

  it("should render expired state correctly", () => {
    render(
      <SubscriptionDetails
        startDate="Jan 1, 2022"
        endDate="expired-date"
        isLoading={false}
      />,
    );

    expect(screen.getByText(SUBSCRIPTION_STATUS.EXPIRED)).toBeInTheDocument();
    expect(screen.getByText(/Expired on/)).toBeInTheDocument();

    const progress = screen.getByTestId("linear-progress");
    expect(progress).toHaveAttribute("data-value", "100");
    expect(progress).toHaveAttribute("data-color", "error");
  });

  it("should render skeletons when loading", () => {
    render(<SubscriptionDetails {...defaultProps} isLoading={true} />);

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
    expect(screen.queryByText("Active")).toBeNull();
    expect(screen.queryByText("Jan 1, 2023")).toBeNull();
  });
});
