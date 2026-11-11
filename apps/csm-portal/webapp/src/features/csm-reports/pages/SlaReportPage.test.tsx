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

import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const get = vi.fn();
const slaReportPDF = vi.fn();
const navigate = vi.fn();

vi.mock("@api/backend/client", () => ({
  BackendApiError: class BackendApiError extends Error {},
  useBackendApi: () => ({ get }),
}));
vi.mock("react-router", () => ({ useParams: () => ({ projectId: "p-1" }) }));
vi.mock("@hooks/useNavTransition", () => ({ useNavTransition: () => navigate }));
vi.mock("@features/csm-reports/components/SlaReportPDF", () => ({
  slaReportPDF: (...args: unknown[]) => slaReportPDF(...args),
}));

import SlaReportPage from "@features/csm-reports/pages/SlaReportPage";
import type { SlaReportCase, SlaReportDetails, SlaReportMetric } from "@features/csm-reports/api/reportTypes";

const NONE: SlaReportMetric = { elapsed: "—", target: "Not applicable", met: 0, missed: 0, running: 0 };

function caseRow(n: number, over: Partial<SlaReportCase> = {}): SlaReportCase {
  return {
    caseId: `00000000-0000-4000-8000-${String(n).padStart(12, "0")}`,
    caseNumber: `CS${String(n).padStart(4, "0")}`,
    caseType: "Incident",
    severity: "Critical (P1)",
    state: "Work In Progress",
    opened: "2026-10-05T08:00:00Z",
    response: { elapsed: "1 Hour", status: "Met" },
    workaround: { elapsed: "—", status: "" },
    resolution: { elapsed: "3 Days", status: "Missed" },
    ...over,
  };
}

function report(over: Partial<SlaReportDetails> = {}): SlaReportDetails {
  return {
    projectName: "Example Project",
    projectKey: "EXP",
    periodStart: "2026-10-01",
    periodEnd: "2026-11-11",
    summary: [
      {
        label: "Incident - Critical (P1)",
        caseType: "incident",
        severity: "S1",
        cases: 2,
        response: { elapsed: "2 Hours 54 Minutes", target: "4 Hours", met: 1, missed: 1, running: 0 },
        workaround: NONE,
        resolution: { elapsed: "—", target: "3 Days", met: 0, missed: 2, running: 0 },
      },
    ],
    cases: [caseRow(1), caseRow(2, { caseType: "Query", response: { elapsed: "10 Minutes", status: "Running" } })],
    ...over,
  };
}

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SlaReportPage />
    </QueryClientProvider>,
  );
}

const lastPath = (): string => get.mock.calls[get.mock.calls.length - 1][0] as string;

describe("SlaReportPage", () => {
  beforeEach(() => {
    get.mockReset();
    slaReportPDF.mockReset();
    navigate.mockReset();
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 10, 11, 12, 0, 0));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("requests the current quarter so far for the project", async () => {
    get.mockResolvedValue(report());
    renderPage();
    expect(await screen.findByTestId("sla-report-period")).toHaveTextContent("Period: 1 Oct 2026 to 11 Nov 2026");
    expect(get).toHaveBeenCalledWith("/generate-sla-report?projectId=p-1&from=2026-10-01&to=2026-11-11");
    expect(screen.getByLabelText("From")).toHaveValue("2026-10-01");
    expect(screen.getByLabelText("To")).toHaveValue("2026-11-11");
  });

  it("shows the summary with the percentile, the target and the counts", async () => {
    get.mockResolvedValue(report());
    renderPage();
    const row = (await screen.findByText("Incident - Critical (P1)")).closest("tr") as HTMLElement;
    expect(within(row).getByText("2 Hours 54 Minutes")).toBeInTheDocument();
    expect(within(row).getByText("Target: 4 Hours")).toBeInTheDocument();
    expect(within(row).getByText("1 met · 1 missed · 0 running")).toBeInTheDocument();
    expect(within(row).getByText("Target: Not applicable")).toBeInTheDocument();
    expect(within(row).getByText("0 met · 2 missed · 0 running")).toBeInTheDocument();
  });

  it("lists each case with its clocks and their status", async () => {
    get.mockResolvedValue(report());
    renderPage();
    const row = (await screen.findByRole("button", { name: "CS0001" })).closest("tr") as HTMLElement;
    expect(within(row).getByText("Critical (P1)")).toBeInTheDocument();
    expect(within(row).getByText("Work In Progress")).toBeInTheDocument();
    expect(within(row).getByText("5 Oct 2026")).toBeInTheDocument();
    expect(within(row).getByText("1 Hour")).toBeInTheDocument();
    expect(within(row).getByText("Met")).toBeInTheDocument();
    expect(within(row).getByText("3 Days")).toBeInTheDocument();
    expect(within(row).getByText("Missed")).toBeInTheDocument();
    const second = screen.getByRole("button", { name: "CS0002" }).closest("tr") as HTMLElement;
    expect(within(second).getByText("Query")).toBeInTheDocument();
    expect(within(second).getByText("Running")).toBeInTheDocument();
  });

  it("says Not recorded for a finished clock with no elapsed time", async () => {
    get.mockResolvedValue(report({ cases: [caseRow(1, { response: { elapsed: "—", status: "Met" } })] }));
    renderPage();
    const row = (await screen.findByRole("button", { name: "CS0001" })).closest("tr") as HTMLElement;
    expect(within(row).getByText("Not recorded")).toBeInTheDocument();
    expect(within(row).getByText("Met")).toBeInTheDocument();
  });

  it("opens a case in a new tab", async () => {
    get.mockResolvedValue(report());
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "CS0001" }));
    expect(open).toHaveBeenCalledWith("/cases/00000000-0000-4000-8000-000000000001", "_blank", "noopener,noreferrer");
    open.mockRestore();
  });

  it("requests the new range when the dates change, and resets to the quarter", async () => {
    get.mockResolvedValue(report());
    renderPage();
    await screen.findByTestId("sla-report-period");

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2026-01-01" } });
    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2026-03-31" } });
    await waitFor(() => expect(lastPath()).toContain("from=2026-01-01&to=2026-03-31"));

    fireEvent.click(screen.getByRole("button", { name: "This quarter" }));
    await waitFor(() => expect(lastPath()).toBe("/generate-sla-report?projectId=p-1&from=2026-10-01&to=2026-11-11"));
    expect(screen.getByLabelText("From")).toHaveValue("2026-10-01");
  });

  it("does not send an invalid range", async () => {
    get.mockResolvedValue(report());
    renderPage();
    await screen.findByTestId("sla-report-period");
    const calls = get.mock.calls.length;

    fireEvent.change(screen.getByLabelText("To"), { target: { value: "2026-09-01" } });

    expect(await screen.findByText("To must be on or after From.")).toBeInTheDocument();
    expect(get.mock.calls.length).toBe(calls);
    expect(screen.getByLabelText("From")).toHaveAttribute("max", "2026-09-01");
  });

  it("says so when no case was opened in the period", async () => {
    get.mockResolvedValue(report({ summary: [], cases: [] }));
    renderPage();
    expect(await screen.findAllByText("No cases were opened in this period.")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: /^CS/ })).not.toBeInTheDocument();
  });

  it("pages the cases", async () => {
    const cases = Array.from({ length: 12 }, (_, i) => caseRow(i + 1));
    get.mockResolvedValue(report({ cases }));
    renderPage();
    expect(await screen.findByRole("button", { name: "CS0001" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "CS0010" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "CS0011" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Go to next page" }));
    expect(await screen.findByRole("button", { name: "CS0011" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "CS0001" })).not.toBeInTheDocument();
  });

  it("builds the PDF from the whole report, in the theme's accent colour", async () => {
    slaReportPDF.mockResolvedValue(undefined);
    const cases = Array.from({ length: 12 }, (_, i) => caseRow(i + 1));
    const full = report({ cases });
    get.mockResolvedValue(full);
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Download PDF" }));
    await waitFor(() => expect(slaReportPDF).toHaveBeenCalledTimes(1));
    const [data, accent] = slaReportPDF.mock.calls[0] as [typeof full, string];
    // every case goes in, not only the ten on the screen's first page
    expect(data.cases).toHaveLength(12);
    expect(data.projectKey).toBe("EXP");
    expect(typeof accent).toBe("string");
    expect(accent.length).toBeGreaterThan(0);
  });

  it("shows an error when the PDF cannot be made", async () => {
    slaReportPDF.mockRejectedValue(new Error("canvas"));
    get.mockResolvedValue(report());
    renderPage();
    fireEvent.click(await screen.findByRole("button", { name: "Download PDF" }));
    expect(await screen.findByText("Could not download the SLA report.")).toBeInTheDocument();
  });

  it("keeps the period controls and a working retry after a period fails", async () => {
    let fail = true;
    get.mockImplementation((path: string) =>
      path.includes("from=2026-01-01") && fail ? Promise.reject(new Error("boom")) : Promise.resolve(report()),
    );
    renderPage();
    await screen.findByTestId("sla-report-period");

    fireEvent.change(screen.getByLabelText("From"), { target: { value: "2026-01-01" } });
    expect(await screen.findByText("Could not load the SLA report.")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Report period" })).toBeInTheDocument();

    fail = false;
    const calls = get.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: /try again/i }));
    await waitFor(() => expect(get.mock.calls.length).toBeGreaterThan(calls));
    expect(await screen.findByTestId("sla-report-period")).toBeInTheDocument();
  });

  it("goes back to the project", async () => {
    get.mockResolvedValue(report());
    renderPage();
    await screen.findByTestId("sla-report-period");
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(navigate).toHaveBeenCalledWith("/customers/projects/p-1");
  });
});
