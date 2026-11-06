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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const get = vi.fn();

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ get }),
}));
vi.mock("react-router", () => ({ useParams: () => ({ projectId: "p-1" }) }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => vi.fn() }));
vi.mock("jspdf", () => ({ default: class {} }));
vi.mock("@features/csm-reports/components/reportPdfCapture", () => ({
  addElementPages: vi.fn(),
  copyCanvasPixels: vi.fn(),
}));
vi.mock("@features/csm-reports/components/PieChart", () => ({ default: () => null }));

import TimelogsReportPage from "@features/csm-reports/pages/TimelogsReportPage";
import { formatPeriod } from "@features/csm-reports/utils/period";

function response(periodStart: string, periodEnd: string) {
  return {
    projectName: "Example Project",
    projectKey: "EXP",
    projectType: "managed_cloud",
    totalQueryHours: "70h 0m",
    remainingQueryHours: "36h 36m",
    periodStart,
    periodEnd,
    cases: [],
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TimelogsReportPage />
    </QueryClientProvider>,
  );
}

const lastPath = (): string => get.mock.calls[get.mock.calls.length - 1][0] as string;

describe("formatPeriod", () => {
  it("formats both bounds", () => {
    expect(formatPeriod("2024-12-07", "2026-12-06")).toBe("Period: 7 Dec 2024 to 6 Dec 2026");
  });
  it("says all time when both are empty or missing", () => {
    expect(formatPeriod("", "")).toBe("Period: all time");
    expect(formatPeriod(undefined, undefined)).toBe("Period: all time");
  });
  it("handles a single bound", () => {
    expect(formatPeriod("2024-12-07", "")).toBe("Period: from 7 Dec 2024");
    expect(formatPeriod("", "2026-12-06")).toBe("Period: up to 6 Dec 2026");
  });
});

describe("TimelogsReportPage period", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("requests with no period params initially and shows the returned period", async () => {
    get.mockResolvedValue(response("2024-12-07", "2026-12-06"));
    renderPage();
    expect(await screen.findByTestId("timelogs-period")).toHaveTextContent("Period: 7 Dec 2024 to 6 Dec 2026");
    expect(get).toHaveBeenCalledWith("/generate-timelogs-breakdown-report?projectId=p-1");
  });

  it("shows all time when the response is unbounded", async () => {
    get.mockResolvedValue(response("", ""));
    renderPage();
    expect(await screen.findByTestId("timelogs-period")).toHaveTextContent("Period: all time");
  });

  it("sends from/to when the dates change and allTime when toggled", async () => {
    get.mockResolvedValue(response("2024-12-07", "2026-12-06"));
    renderPage();
    await screen.findByTestId("timelogs-period");

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2025-01-01" } });
    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2025-03-31" } });
    await waitFor(() => expect(lastPath()).toContain("from=2025-01-01&to=2025-03-31"));

    fireEvent.click(screen.getByRole("button", { name: "All time" }));
    await waitFor(() => expect(lastPath()).toContain("allTime=true"));
    expect(lastPath()).not.toContain("from=");

    fireEvent.click(screen.getByRole("button", { name: "Subscription period" }));
    await waitFor(() => expect(lastPath()).toBe("/generate-timelogs-breakdown-report?projectId=p-1"));
  });

  it("does not send an invalid range", async () => {
    get.mockResolvedValue(response("2024-12-07", "2026-12-06"));
    renderPage();
    await screen.findByTestId("timelogs-period");

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2025-01-01" } });
    await waitFor(() => expect(lastPath()).toContain("from=2025-01-01"));
    const calls = get.mock.calls.length;

    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2024-01-01" } });
    expect(await screen.findByText("To must be on or after From.")).toBeInTheDocument();
    expect(get.mock.calls.length).toBe(calls);
    expect(screen.getByLabelText("From")).toHaveAttribute("max", "2024-01-01");
    expect(screen.getByLabelText("To")).toHaveAttribute("min", "2025-01-01");
  });
});
