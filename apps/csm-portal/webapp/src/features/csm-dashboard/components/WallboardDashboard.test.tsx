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
import "@testing-library/jest-dom/vitest";
import type { BeDashboard, BeDashboardWidget, BeWidgetResourceType } from "@api/backend/types";
import type { WallboardPanelProps } from "@features/csm-dashboard/components/WallboardPanel";
import type { WallboardCreSectionProps } from "@features/csm-dashboard/components/WallboardCreSection";
import type { WallboardSreSectionProps } from "@features/csm-dashboard/components/WallboardSreSection";
import type { WallboardStatGridProps } from "@features/csm-dashboard/components/WallboardStatGrid";
import type { WallboardSecondaryStatProps } from "@features/csm-dashboard/components/WallboardSecondaryStat";

const useDashboardMock = vi.fn();
vi.mock("@features/csm-dashboard/api/useDashboard", () => ({
  useDashboard: (...args: unknown[]) => useDashboardMock(...args),
}));

vi.mock("@features/csm-dashboard/components/WallboardPanel", () => ({
  default: ({ section, title, children }: WallboardPanelProps) => (
    <div data-testid={`panel-${section}`}>
      <div data-testid={`panel-title-${section}`}>{title}</div>
      {children}
    </div>
  ),
}));
vi.mock("@features/csm-dashboard/components/WallboardCreSection", () => ({
  default: ({ widgets }: WallboardCreSectionProps) => (
    <div data-testid="cre-section">{widgets.map((w) => w.displayName).join(",")}</div>
  ),
}));
vi.mock("@features/csm-dashboard/components/WallboardSreSection", () => ({
  default: ({ widgets }: WallboardSreSectionProps) => (
    <div data-testid="sre-section">{widgets.map((w) => w.displayName).join(",")}</div>
  ),
}));
vi.mock("@features/csm-dashboard/components/WallboardStatGrid", () => ({
  default: ({ widgets, section, columns }: WallboardStatGridProps) => (
    <div data-testid={`stat-grid-${section}`} data-columns={columns}>
      {widgets.map((w) => w.displayName).join(",")}
    </div>
  ),
}));
vi.mock("@features/csm-dashboard/components/WallboardSecondaryStat", () => ({
  default: ({ displayName }: WallboardSecondaryStatProps) => <div data-testid="secondary-tile">{displayName}</div>,
}));

import WallboardDashboard from "@features/csm-dashboard/components/WallboardDashboard";

function widget(
  id: string,
  displayName: string,
  section: string,
  resourceType: BeWidgetResourceType = "case",
  shape: BeDashboardWidget["shape"] = "count",
): BeDashboardWidget {
  return { widgetId: id, displayName, resourceType, shape, gridWidth: 4, query: {}, section };
}

function dashboardResult(widgets: BeDashboardWidget[]): BeDashboard {
  return {
    id: "cs-overview",
    displayName: "CS Overview",
    isDefault: false,
    isTeamBased: false,
    widgets,
  };
}

describe("WallboardDashboard", () => {
  it("shows loading skeletons before the dashboard resolves", () => {
    useDashboardMock.mockReturnValue({ data: undefined, isLoading: true, isError: false });
    const { container } = render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBeGreaterThan(0);
  });

  it("shows an error message rather than crashing when the dashboard fails to load", () => {
    useDashboardMock.mockReturnValue({ data: undefined, isLoading: false, isError: true });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(screen.getByText(/Could not load the dashboard/)).toBeInTheDocument();
  });

  it("shows a dash for 'Last updated' before any fetch has resolved (dataUpdatedAt is 0)", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([]),
      isLoading: false,
      isError: false,
      dataUpdatedAt: 0,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(screen.getByText("Last updated: —")).toBeInTheDocument();
  });

  // The actual regression this covers: React Query's default structural
  // sharing keeps the same `data` reference across a refetch that returns
  // identical values, so tracking "did `data`'s reference change" would
  // miss this refetch entirely. `dataUpdatedAt` (from the query cache,
  // not local component state) must still reflect it.
  it("shows the time from dataUpdatedAt, not from whether 'data' itself changed", () => {
    const fetchedAt = new Date("2026-01-01T15:04:05").getTime();
    useDashboardMock.mockReturnValue({
      data: dashboardResult([]),
      isLoading: false,
      isError: false,
      dataUpdatedAt: fetchedAt,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(screen.getByText(`Last updated: ${new Date(fetchedAt).toLocaleTimeString()}`)).toBeInTheDocument();
  });

  it("routes each section to its own panel by keyword-matching the section name, case-insensitively", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([
        widget("w1", "Open", "Customer Reliability Engineering (CRE)"),
        widget("w2", "Open", "site reliability"),
        widget("w3", "Open", "SECURITY Report"),
        widget("w4", "Open Onboarding", "Forward Deployed Engineering (FDE)"),
      ]),
      isLoading: false,
      isError: false,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);

    expect(screen.getByTestId("panel-cre")).toBeInTheDocument();
    expect(screen.getByTestId("cre-section")).toHaveTextContent("Open");
    expect(screen.getByTestId("panel-sre")).toBeInTheDocument();
    expect(screen.getByTestId("sre-section")).toHaveTextContent("Open");
    expect(screen.getByTestId("panel-security")).toBeInTheDocument();
    expect(screen.getByTestId("stat-grid-security")).toHaveAttribute("data-columns", "2");
    expect(screen.getByTestId("panel-fde")).toBeInTheDocument();
    expect(screen.getByTestId("stat-grid-fde")).toHaveAttribute("data-columns", "3");
  });

  // Regression test (CodeRabbit): `section` is free-text, and
  // groupWidgetsBySection preserves every distinct string as its own
  // group — so two differently-named CRE sections must not silently
  // drop one of them; both should merge into the one CRE panel.
  it("merges widgets from every section string that maps to the same family, not just the first one found", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([
        widget("w1", "Open", "CRE Operations"),
        widget("w2", "Escalations", "CRE Escalations"),
      ]),
      isLoading: false,
      isError: false,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);

    expect(screen.getAllByTestId("panel-cre")).toHaveLength(1);
    expect(screen.getByTestId("cre-section")).toHaveTextContent("Open,Escalations");
  });

  it("resolves the live config's mismatched displayNames to the original's exact labels before any child ever sees them", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([
        widget("w1", "SRE - Open Incident", "sre"),
        widget("w2", "Being Fixed/Update", "CRE"),
      ]),
      isLoading: false,
      isError: false,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    // Exact equality, not substring toHaveTextContent — "Open" is itself a
    // substring of the raw "SRE - Open Incident", so only an exact
    // comparison actually proves aliasing happened.
    expect(screen.getByTestId("sre-section").textContent).toBe("Open");
    expect(screen.getByTestId("cre-section").textContent).toBe("Being Fixed");
  });

  it("filters out any non-'count'-shape widget before grouping, matching the original wallboard's all-count design", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([
        widget("w1", "Open", "CRE"),
        widget("w2", "Recent Cases", "CRE", "case", "list"),
      ]),
      isLoading: false,
      isError: false,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(screen.getByTestId("cre-section")).toHaveTextContent("Open");
    expect(screen.getByTestId("cre-section")).not.toHaveTextContent("Recent Cases");
  });

  it("renders an unrecognized section as a fallback grid instead of silently dropping its widgets", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([widget("w1", "Some Metric", "Some Unrelated Section")]),
      isLoading: false,
      isError: false,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(screen.getByText("Some Unrelated Section")).toBeInTheDocument();
    expect(screen.getByText("Some Metric")).toBeInTheDocument();
  });

  it("renders nothing for a section family with no widgets configured, rather than an empty panel", () => {
    useDashboardMock.mockReturnValue({
      data: dashboardResult([widget("w1", "Open", "CRE")]),
      isLoading: false,
      isError: false,
    });
    render(<WallboardDashboard dashboardId="cs-overview" />);
    expect(screen.getByTestId("panel-cre")).toBeInTheDocument();
    expect(screen.queryByTestId("panel-sre")).not.toBeInTheDocument();
    expect(screen.queryByTestId("panel-security")).not.toBeInTheDocument();
    expect(screen.queryByTestId("panel-fde")).not.toBeInTheDocument();
  });
});
