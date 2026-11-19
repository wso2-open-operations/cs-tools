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
import { MemoryRouter, Route, Routes } from "react-router";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const getMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));
// jsdom has no layout, so the real charts draw nothing: stand-ins that expose
// the data under test.
vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  BarChart: ({ data, bars }: { data: Record<string, unknown>[]; bars?: { dataKey: string; name?: string }[] }) => (
    <div data-testid="bar-chart">
      {(bars ?? []).map((b) => (
        <div key={b.dataKey} data-testid={`series-${b.dataKey}`}>
          {b.name}: {data.map((r) => `${r.label}=${r[b.dataKey]}`).join(", ")}
        </div>
      ))}
      {!bars && <div data-testid="slices">{data.map((r) => `${r.name}=${r.value}`).join(", ")}</div>}
    </div>
  ),
  Bar: () => null,
  Cell: () => null,
  XAxis: () => null,
  LineChart: () => null,
}));
vi.mock("@features/csm-cases/components/ProductIssueDetailDialog", () => ({
  ProductIssueDetailDialog: ({ workItemId, onClose }: { workItemId: string; onClose: () => void }) => (
    <div data-testid="issue-dialog">
      {workItemId}
      <button onClick={onClose}>close dialog</button>
    </div>
  ),
}));
vi.mock("@components/error/Error403Page", () => ({
  default: () => <div data-testid="error-403">403</div>,
}));

import ProductIssueDashboardPage from "@features/csm-dashboard/pages/ProductIssueDashboardPage";

const SLA = {
  state: "BREACHED",
  budgetSeconds: 3600,
  consumedSeconds: 7200,
  remainingSeconds: 0,
  percentage: 200,
  running: true,
};

const ITEM = {
  id: "wi-1",
  number: "PI0000001",
  title: "Token endpoint fails",
  productName: "Identity Server",
  owner: "wso2",
  repository: "product-is",
  issueNumber: 12,
  htmlUrl: "https://github.com/wso2/product-is/issues/12",
  boardStatus: "WAITING_ON_CS",
  priority: "P1",
  openedOn: "2026-11-02T10:00:00+05:30",
  wocSince: "2026-11-05T10:00:00+05:30",
  case: { id: "case-9", number: "CS0009" },
  openedByEmail: "a@example.com",
  sla: SLA,
};

const OPEN_BY_SLA = { ok: 3, atRisk: 1, breached: 2, paused: 1, none: 0 };

const DASHBOARD = {
  periodStart: "2026-08-31T00:00:00+05:30",
  periodEnd: "2026-11-19T00:00:00+05:30",
  generatedOn: "2026-11-18T12:00:00+05:30",
  bucket: "week",
  products: [],
  totals: {
    open: 7,
    openedInPeriod: 20,
    resolvedInPeriod: 15,
    waitingOnCs: 4,
    waitingOnProductTeam: 2,
    unmatchedOpen: 1,
    nonStandardColumnOpen: 0,
    openBySla: OPEN_BY_SLA,
    slaMet: 9,
    slaBreached: 1,
    compliancePercentage: 90,
  },
  trend: [
    { start: "2026-11-02T00:00:00+05:30", end: "2026-11-09T00:00:00+05:30", opened: 5, resolved: 3 },
    { start: "2026-11-09T00:00:00+05:30", end: "2026-11-16T00:00:00+05:30", opened: 6, resolved: 4 },
  ],
  byProduct: [
    {
      productName: "Identity Server",
      owner: "wso2",
      repository: "product-is",
      open: 5,
      openedInPeriod: 12,
      resolvedInPeriod: 9,
      waitingOnCs: 3,
      waitingOnProductTeam: 1,
      openBySla: OPEN_BY_SLA,
      slaMet: 6,
      slaBreached: 1,
      compliancePercentage: 85.7,
    },
    {
      productName: null,
      owner: "wso2-enterprise",
      repository: "misc",
      open: 2,
      openedInPeriod: 8,
      resolvedInPeriod: 6,
      waitingOnCs: 1,
      waitingOnProductTeam: 1,
      openBySla: { ok: 0, atRisk: 0, breached: 0, paused: 0, none: 2 },
      slaMet: 0,
      slaBreached: 0,
      compliancePercentage: null,
    },
  ],
  wocAgeing: {
    buckets: [
      { label: "<1d", count: 1 },
      { label: "1-3d", count: 0 },
      { label: "3-7d", count: 2 },
      { label: "7-14d", count: 1 },
      { label: ">14d", count: 0 },
    ],
    oldest: [ITEM],
  },
  blockedCases: {
    total: 1,
    byProduct: [{ productName: "Identity Server", owner: "wso2", repository: "product-is", cases: 1 }],
    cases: [
      {
        caseId: "case-7",
        caseNumber: "CS0007",
        subject: "Cannot sign in",
        issues: [
          {
            id: "wi-2",
            number: "PI0000002",
            title: "SSO broken",
            productName: "Identity Server",
            boardStatus: "WAITING_ON_PRODUCT_TEAM",
            openedOn: "2026-10-01T10:00:00+05:30",
          },
        ],
        oldestOpenedOn: "2026-10-01T10:00:00+05:30",
      },
    ],
  },
  breachedOpen: [{ ...ITEM, id: "wi-3", number: "PI0000003", title: "Breached one", wocSince: null, case: null }],
};

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/dashboard/product-issues"]}>
        <Routes>
          <Route path="/dashboard/product-issues" element={<ProductIssueDashboardPage />} />
          <Route path="/cases/:id" element={<div data-testid="case-page" />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function lastPath(): URL {
  const calls = getMock.mock.calls;
  return new URL(String(calls[calls.length - 1][0]), "https://x.test");
}

describe("ProductIssueDashboardPage", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  describe("past midnight Asia/Kolkata", () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it("asks for a new window after Refresh", async () => {
      vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-11-18T18:00:00Z") }); // 23:30 IST, 18 Nov
      getMock.mockResolvedValue(DASHBOARD);
      renderPage();
      await screen.findByText("Opened and resolved");
      expect(lastPath().searchParams.get("to")).toBe("2026-11-19");

      vi.setSystemTime(new Date("2026-11-18T19:00:00Z")); // 00:30 IST, 19 Nov
      fireEvent.click(screen.getByRole("button", { name: /refresh product issues/i }));
      await waitFor(() => expect(lastPath().searchParams.get("to")).toBe("2026-11-20"));
    });
  });

  it("shows a skeleton while loading", () => {
    getMock.mockReturnValue(new Promise(() => {}));
    renderPage();
    expect(screen.getByTestId("pi-loading")).toBeInTheDocument();
  });

  it("renders every section from the response", async () => {
    getMock.mockResolvedValue(DASHBOARD);
    renderPage();
    expect(await screen.findByText("Opened and resolved")).toBeInTheDocument();

    // KPI tiles
    const kpi = (label: string) => within(screen.getByTestId(`pi-kpi-${label}`));
    expect(kpi("Open").getByText("7")).toBeInTheDocument();
    expect(kpi("Opened in period").getByText("20")).toBeInTheDocument();
    expect(kpi("Resolved in period").getByText("15")).toBeInTheDocument();
    expect(kpi("Waiting on CS").getByText("4")).toBeInTheDocument();
    expect(kpi("Waiting on product team").getByText("2")).toBeInTheDocument();
    expect(kpi("SLA compliance").getByText("90%")).toBeInTheDocument();
    expect(kpi("Breached open").getByText("2")).toBeInTheDocument();
    expect(kpi("Unmatched open").getByText("1")).toBeInTheDocument();

    // Trend: opened vs resolved, one point per bucket
    expect(screen.getByTestId("series-opened")).toHaveTextContent("Opened: Nov 2-8=5, Nov 9-15=6");
    expect(screen.getByTestId("series-closed")).toHaveTextContent("Resolved: Nov 2-8=3, Nov 9-15=4");

    // By product, Unmapped for a null product
    const byProduct = within(screen.getByText("By product").closest(".MuiCard-root") as HTMLElement);
    expect(byProduct.getByText("Identity Server")).toBeInTheDocument();
    expect(byProduct.getByText("Unmapped")).toBeInTheDocument();
    expect(screen.getByText("85.7%")).toBeInTheDocument();
    expect(screen.getByText("wso2-enterprise/misc")).toBeInTheDocument();

    // Ageing bars and oldest list
    expect(screen.getByTestId("slices")).toHaveTextContent("<1d=1, 1-3d=0, 3-7d=2, 7-14d=1, >14d=0");
    expect(screen.getByText("Token endpoint fails")).toBeInTheDocument();

    // Blocked cases
    expect(screen.getByText("CS0007")).toBeInTheDocument();
    expect(screen.getByText("Cannot sign in")).toBeInTheDocument();
    expect(screen.getByText("PI0000002 (Identity Server)")).toBeInTheDocument();

    // Breached open
    expect(screen.getByText("Breached one")).toBeInTheDocument();
  });

  it("requests the default period (12 weeks, no product filter)", async () => {
    getMock.mockResolvedValue(DASHBOARD);
    renderPage();
    await screen.findByText("Opened and resolved");
    const url = lastPath();
    expect(url.pathname).toBe("/product-work-items/dashboard");
    expect(url.searchParams.get("bucket")).toBe("week");
    expect(url.searchParams.get("from")).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(url.searchParams.get("to")).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(url.searchParams.has("products")).toBe(false);
  });

  it("sends bucket=month for a month period", async () => {
    getMock.mockResolvedValue(DASHBOARD);
    renderPage();
    await screen.findByText("Opened and resolved");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /period/i }));
    fireEvent.click(await screen.findByRole("option", { name: "Last 6 months" }));

    await waitFor(() => expect(lastPath().searchParams.get("bucket")).toBe("month"));
    expect(lastPath().searchParams.get("from")).toMatch(/-01$/);
  });

  it("sends the chosen products, offered from the unfiltered load", async () => {
    getMock.mockResolvedValue(DASHBOARD);
    renderPage();
    await screen.findByText("Opened and resolved");

    fireEvent.mouseDown(screen.getByRole("combobox", { name: /products/i }));
    expect(await screen.findByRole("option", { name: "Identity Server" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("option", { name: "Unmapped" }));
    fireEvent.click(screen.getByRole("option", { name: "Identity Server" }));

    await waitFor(() => expect(lastPath().searchParams.get("products")).toBe("Unmapped,Identity Server"));
  });

  it("opens the issue dialog from an issue row", async () => {
    getMock.mockResolvedValue(DASHBOARD);
    renderPage();
    await screen.findByText("Opened and resolved");

    fireEvent.click(screen.getAllByRole("button", { name: "View product issue PI0000001" })[0]);
    expect(screen.getByTestId("issue-dialog")).toHaveTextContent("wi-1");
    fireEvent.click(screen.getByRole("button", { name: "close dialog" }));
    expect(screen.queryByTestId("issue-dialog")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "View product issue PI0000002" }));
    expect(screen.getByTestId("issue-dialog")).toHaveTextContent("wi-2");
  });

  it("links a case number to the case page", async () => {
    getMock.mockResolvedValue(DASHBOARD);
    renderPage();
    await screen.findByText("Opened and resolved");
    expect(screen.getByRole("link", { name: "CS0007" })).toHaveAttribute("href", "/cases/case-7");
    expect(screen.getByRole("link", { name: "CS0009" })).toHaveAttribute("href", "/cases/case-9");
    fireEvent.click(screen.getByRole("link", { name: "CS0007" }));
    expect(await screen.findByTestId("case-page")).toBeInTheDocument();
  });

  it("shows empty states for an empty dashboard", async () => {
    getMock.mockResolvedValue({
      ...DASHBOARD,
      totals: {
        ...DASHBOARD.totals,
        open: 0,
        openedInPeriod: 0,
        resolvedInPeriod: 0,
        waitingOnCs: 0,
        waitingOnProductTeam: 0,
        compliancePercentage: null,
        slaMet: 0,
        slaBreached: 0,
        openBySla: { ok: 0, atRisk: 0, breached: 0, paused: 0, none: 0 },
      },
      trend: [],
      byProduct: [],
      wocAgeing: { buckets: [], oldest: [] },
      blockedCases: { total: 0, byProduct: [], cases: [] },
      breachedOpen: [],
    });
    renderPage();
    expect(await screen.findByText("No product issues in this period.")).toBeInTheDocument();
    expect(screen.getByText("No issues are waiting on CS.")).toBeInTheDocument();
    expect(screen.getByText("No cases are blocked by a product team.")).toBeInTheDocument();
    expect(screen.getByText("No open issue has breached its SLA.")).toBeInTheDocument();
    expect(within(screen.getByTestId("pi-kpi-SLA compliance")).getByText("—")).toBeInTheDocument();
  });

  it("says Not available on a 404 (feature off)", async () => {
    getMock.mockResolvedValue(null);
    renderPage();
    expect(await screen.findByTestId("pi-not-available")).toHaveTextContent("Not available");
  });

  it("shows the standard 403 page on a 403", async () => {
    getMock.mockRejectedValue(Object.assign(new Error("forbidden"), { status: 403 }));
    renderPage();
    expect(await screen.findByTestId("error-403")).toBeInTheDocument();
  });

  it("shows an error message for a server error", async () => {
    getMock.mockRejectedValue(Object.assign(new Error("boom"), { status: 500 }));
    renderPage();
    expect(await screen.findByText("Could not load the product issues dashboard.")).toBeInTheDocument();
  });
});
