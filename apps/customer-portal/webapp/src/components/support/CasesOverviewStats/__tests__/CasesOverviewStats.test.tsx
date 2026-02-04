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
import CasesOverviewStats from "../CasesOverviewStats";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div data-testid="box">{children}</div>,
  Grid: ({ children, container }: any) => (
    <div data-testid={container ? "grid-container" : "grid-item"}>
      {children}
    </div>
  ),
  StatCard: ({ label, value, icon }: any) => {
    const ValueSkeleton =
      value && typeof value === "object" && "Skeleton" in value
        ? (value as any).Skeleton
        : null;

    return (
      <div data-testid="oxygen-stat-card">
        <div data-testid="stat-card-icon">{icon}</div>
        <span>{label}</span>
        <div data-testid="stat-card-value">
          {ValueSkeleton ? <ValueSkeleton variant="text" /> : value}
        </div>
      </div>
    );
  },
  Skeleton: ({ variant }: any) => (
    <div data-testid="skeleton" data-variant={variant} />
  ),
}));

// Mock icons
vi.mock("@wso2/oxygen-ui-icons-react", () => ({
  Bot: () => <svg data-testid="icon-bot" />,
  CircleAlert: () => <svg data-testid="icon-alert" />,
  CircleCheck: () => <svg data-testid="icon-check" />,
  Clock: () => <svg data-testid="icon-clock" />,
  FileText: () => <svg data-testid="icon-file-text" />,
  MessageSquare: () => <svg data-testid="icon-message" />,
  MessageSquareDiff: () => <svg data-testid="icon-message-diff" />,
  MessageSquareMore: () => <svg data-testid="icon-message-more" />,
  TrendingUp: () => <svg data-testid="icon-trending-up" />,
}));

describe("CasesOverviewStats", () => {
  it("should render loading state correctly", () => {
    render(<CasesOverviewStats isLoading={true} stats={undefined} />);

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons).toHaveLength(4);
    expect(screen.getByTestId("icon-file-text")).toBeInTheDocument();
    expect(screen.getByTestId("icon-message")).toBeInTheDocument();
    expect(screen.getByTestId("icon-trending-up")).toBeInTheDocument();
    expect(screen.getByTestId("icon-bot")).toBeInTheDocument();
  });

  it("should render statistics correctly when data is loaded", () => {
    const mockStats = {
      activeChats: 5,
      resolvedChats: 20,
      sessionChats: 15,
      totalCases: 10,
    };

    render(<CasesOverviewStats isLoading={false} stats={mockStats} />);

    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("15")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();

    expect(screen.getByTestId("icon-trending-up")).toBeInTheDocument();
    expect(screen.getByTestId("icon-bot")).toBeInTheDocument();
    expect(screen.getByText("Ongoing Cases")).toBeInTheDocument();
    expect(screen.getByText("Chat Sessions")).toBeInTheDocument();
    expect(screen.getByText("Resolved via Chat")).toBeInTheDocument();
    expect(screen.getByText("Active Chats")).toBeInTheDocument();
  });
});
