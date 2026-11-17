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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProjectTimeTracking from "@time-tracking/ProjectTimeTracking";

const authFetchMock = vi.fn();

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ isSignedIn: true, isLoading: false }),
}));

vi.mock("@/hooks/useAuthApiClient", () => ({
  useAuthApiClient: () => authFetchMock,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

vi.mock("@time-tracking/ServiceHoursStatCards", () => ({
  default: () => <div data-testid="service-hours-stat-cards" />,
}));

vi.mock("@time-tracking/TimeCardsCsvExportButton", () => ({
  default: () => <div data-testid="csv-export-button" />,
}));

vi.mock("@time-tracking/TimeTrackingCard", () => ({
  default: ({ card }: { card: { case: { id: string } } }) => (
    <div data-testid="time-tracking-card">{card.case.id}</div>
  ),
}));

vi.mock("@time-tracking/TimeTrackingCardSkeleton", () => ({
  default: () => <div data-testid="time-tracking-card-skeleton" />,
}));

vi.mock("@time-tracking/TimeTrackingErrorState", () => ({
  default: () => <div data-testid="time-tracking-error-state" />,
}));

vi.mock("@components/empty-state/EmptyState", () => ({
  default: () => <div data-testid="empty-state" />,
}));

vi.mock("@wso2/oxygen-ui", async () => {
  const actual = await vi.importActual<Record<string, unknown>>("@wso2/oxygen-ui");
  return {
    ...actual,
    Pagination: ({
      count,
      onChange,
    }: {
      count: number;
      onChange: (event: unknown, value: number) => void;
    }) => (
      <div data-testid="pagination">
        {Array.from({ length: count }, (_, index) => index + 1).map((pageNumber) => (
          <button
            key={pageNumber}
            type="button"
            onClick={() => onChange(null, pageNumber)}
          >
            {pageNumber}
          </button>
        ))}
      </div>
    ),
  };
});

function createWrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

function makeCaseTimeCard(id: string) {
  return {
    case: { id, number: id, name: id, updatedOn: "2026-01-01", project: { id: "p1", label: "p1" } },
    totalTime: 1,
    totalCount: 1,
    billable: { totalTime: 1, count: 1 },
    nonBillable: { totalTime: 0, count: 0 },
  };
}

describe("ProjectTimeTracking", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (
      window as unknown as { config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string } }
    ).config = { CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test" };

    authFetchMock.mockImplementation(async (_url: string, init: RequestInit) => {
      const body = JSON.parse(init.body as string) as {
        pagination: { offset: number; limit: number };
      };
      const { offset, limit } = body.pagination;
      return {
        ok: true,
        status: 200,
        json: async () => ({
          offset,
          limit,
          totalRecords: 25,
          caseTimeCards: Array.from({ length: limit }, (_, i) =>
            makeCaseTimeCard(`case-${offset + i}`),
          ),
        }),
      };
    });
  });

  it("requests offset 0 when a date filter changes from a later page", async () => {
    render(<ProjectTimeTracking projectId="p1" />, { wrapper: createWrapper() });

    await waitFor(() =>
      expect(screen.getAllByTestId("time-tracking-card").length).toBeGreaterThan(0),
    );

    fireEvent.click(screen.getByText("2"));

    await waitFor(() =>
      expect(authFetchMock).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          body: expect.stringContaining("\"offset\":10"),
        }),
      ),
    );

    const startDateInput = screen.getByLabelText("Start date");
    fireEvent.change(startDateInput, { target: { value: "01/01/2026" } });

    await waitFor(() =>
      expect(authFetchMock).toHaveBeenLastCalledWith(
        expect.any(String),
        expect.objectContaining({
          body: expect.stringContaining("\"offset\":0"),
        }),
      ),
    );
  });
});
