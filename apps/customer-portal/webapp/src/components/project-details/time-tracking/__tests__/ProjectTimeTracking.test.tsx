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
import { describe, it, expect, vi, beforeEach } from "vitest";
import ProjectTimeTracking from "@time-tracking/ProjectTimeTracking";
import useSearchProjectTimeCards from "@api/useSearchProjectTimeCards";
import useGetTimeCardsStats from "@api/useGetTimeCardsStats";
import useGetProjectFilters from "@api/useGetProjectFilters";

vi.mock("@api/useSearchProjectTimeCards");
vi.mock("@api/useGetTimeCardsStats");
vi.mock("@api/useGetProjectFilters");

vi.mock("@time-tracking/TimeTrackingStatCards", () => ({
  default: ({ isLoading, isError }: { isLoading?: boolean; isError?: boolean }) => (
    <div data-testid="stat-cards">
      {isLoading ? "Stats Loading" : "Stats Loaded"}
      {isError ? "Stats Error" : "Stats OK"}
    </div>
  ),
}));

vi.mock("@time-tracking/TimeCardsDateFilter", () => ({
  default: ({ state, onStateChange, startDate, endDate }: {
    state: string;
    onStateChange: (value: string) => void;
    startDate: string;
    endDate: string;
  }) => (
    <div data-testid="date-filter">
      <span data-testid="filter-state">{state || "no-state"}</span>
      <span data-testid="filter-start-date">{startDate}</span>
      <span data-testid="filter-end-date">{endDate}</span>
      <button
        data-testid="change-state-btn"
        onClick={() => onStateChange("Approved")}
      >
        Change State
      </button>
    </div>
  ),
}));

vi.mock("@time-tracking/TimeTrackingCard", () => ({
  default: ({ card }: { card: { case?: { label?: string } } }) => (
    <div data-testid="time-card">
      {card?.case?.label ?? "No label"}
    </div>
  ),
}));

vi.mock("@time-tracking/TimeTrackingCardSkeleton", () => ({
  default: () => <div data-testid="skeleton" />,
}));

vi.mock("@time-tracking/TimeTrackingErrorState", () => ({
  default: () => <div data-testid="error-state">Error State</div>,
}));

vi.mock("@components/common/empty-state/EmptyState", () => ({
  default: ({ description }: { description: string }) => (
    <div data-testid="empty-state">{description}</div>
  ),
}));

describe("ProjectTimeTracking", () => {
  const projectId = "proj-1";

  beforeEach(() => {
    vi.clearAllMocks();
    
    // Mock project filters with timeCardStates
    vi.mocked(useGetProjectFilters).mockReturnValue({
      data: {
        timeCardStates: [
          { id: "Pending", label: "Pending" },
          { id: "Submitted", label: "Submitted" },
          { id: "Approved", label: "Approved" },
        ],
      },
      isLoading: false,
      isError: false,
    } as any);
  });

  it("should render 7 skeletons when time cards are loading", () => {
    vi.mocked(useGetTimeCardsStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);

    vi.mocked(useSearchProjectTimeCards).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    } as any);

    render(<ProjectTimeTracking projectId={projectId} />);

    expect(screen.getAllByTestId("skeleton")).toHaveLength(7);
  });

  it("should render error state when time cards fail to load", () => {
    vi.mocked(useGetTimeCardsStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);

    vi.mocked(useSearchProjectTimeCards).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    } as any);

    render(<ProjectTimeTracking projectId={projectId} />);

    expect(screen.getByTestId("error-state")).toBeInTheDocument();
  });

  it("should render time cards when data is loaded", () => {
    const mockData = {
      pages: [
        {
          timeCards: [
            {
              id: "1",
              case: { label: "Log 1", number: "CS001", id: "c1" },
              totalTime: 60,
              state: { id: "approved", label: "Approved" },
              hasBillable: false,
              approvedBy: null,
              project: { id: "p1", label: "Project 1" },
              createdOn: "2025-12-10",
            },
            {
              id: "2",
              case: { label: "Log 2", number: "CS002", id: "c2" },
              totalTime: 30,
              state: { id: "submitted", label: "Submitted" },
              hasBillable: true,
              approvedBy: null,
              project: { id: "p1", label: "Project 1" },
              createdOn: "2025-12-11",
            },
          ],
          totalRecords: 2,
          offset: 0,
          limit: 10,
        },
      ],
    };

    vi.mocked(useGetTimeCardsStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);

    vi.mocked(useSearchProjectTimeCards).mockReturnValue({
      data: mockData,
      isLoading: false,
      isError: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    } as any);

    render(<ProjectTimeTracking projectId={projectId} />);

    expect(screen.getByText("Log 1")).toBeInTheDocument();
    expect(screen.getByText("Log 2")).toBeInTheDocument();
    expect(screen.getAllByTestId("time-card")).toHaveLength(2);
  });

  it("should always render stat cards regardless of time cards state", () => {
    vi.mocked(useGetTimeCardsStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);

    vi.mocked(useSearchProjectTimeCards).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    } as any);

    render(<ProjectTimeTracking projectId={projectId} />);

    expect(screen.getByTestId("stat-cards")).toBeInTheDocument();
  });

  it("should render date filter between stats and time cards", () => {
    vi.mocked(useGetTimeCardsStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);

    vi.mocked(useSearchProjectTimeCards).mockReturnValue({
      data: { pages: [{ timeCards: [], totalRecords: 0, offset: 0, limit: 10 }] },
      isLoading: false,
      isError: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    } as any);

    render(<ProjectTimeTracking projectId={projectId} />);

    expect(screen.getByTestId("date-filter")).toBeInTheDocument();
  });

  it("should render empty state when there are no time cards", () => {
    vi.mocked(useGetTimeCardsStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
    } as any);

    vi.mocked(useSearchProjectTimeCards).mockReturnValue({
      data: { pages: [{ timeCards: [], totalRecords: 0, offset: 0, limit: 10 }] },
      isLoading: false,
      isError: false,
      hasNextPage: false,
      fetchNextPage: vi.fn(),
    } as any);

    render(<ProjectTimeTracking projectId={projectId} />);

    expect(screen.getByTestId("empty-state")).toBeInTheDocument();
    expect(screen.getByText("No time logs available.")).toBeInTheDocument();
  });
});
