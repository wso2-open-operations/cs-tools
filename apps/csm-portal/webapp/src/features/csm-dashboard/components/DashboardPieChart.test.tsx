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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { ReactNode } from "react";

// Recharts' ResponsiveContainer measures a real layout size, which jsdom
// always reports as 0 — nothing would render. Stubbed to plain clickable
// stand-ins, same approach `DashboardWidgetTile.test.tsx` uses for this same
// package. Unlike that file's own mock, `Pie` here also invokes its own
// `label` render-prop (when one is passed) once per data point, mirroring
// how real recharts calls a custom pie label with the wedge's own geometry
// — needed to exercise `DashboardPieChart`'s `inlineLabels` mode.
vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  PieChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  Pie: ({
    data,
    onClick,
    label,
  }: {
    data: { name: string; value: number }[];
    onClick?: (item: unknown, index: number, event?: unknown) => void;
    label?: false | ((props: Record<string, unknown>) => ReactNode);
  }) => (
    <div>
      {data.map((item, i) => (
        <button key={item.name} type="button" onClick={(e) => onClick?.(item, i, e)}>
          slice:{item.name}:{item.value}
        </button>
      ))}
      {typeof label === "function" && (
        <svg>
          {data.map((_item, i) =>
            label({
              cx: 100,
              cy: 100,
              midAngle: 45,
              innerRadius: 46,
              outerRadius: 66,
              index: i,
            }),
          )}
        </svg>
      )}
    </div>
  ),
  Cell: () => null,
}));

import DashboardPieChart from "@features/csm-dashboard/components/DashboardPieChart";
import type { PieSliceResult } from "@features/csm-dashboard/api/useWidgetPieData";

const SLICES: PieSliceResult[] = [
  { label: "Critical", value: 1, query: null, color: "error" },
  { label: "High", value: 3, query: null, color: "warning" },
];

describe("DashboardPieChart", () => {
  it("renders a skeleton per slice while loading", () => {
    const { container } = render(
      <DashboardPieChart
        slices={SLICES}
        total={0}
        isLoading={true}
        isError={false}
        onSliceClick={vi.fn()}
      />,
    );
    // One circular skeleton for the ring, one rounded skeleton per slice.
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1 + SLICES.length);
  });

  it("renders its error state instead of the chart", () => {
    render(
      <DashboardPieChart
        slices={SLICES}
        total={0}
        isLoading={false}
        isError={true}
        onSliceClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Could not load this widget.")).toBeInTheDocument();
  });

  it("renders its empty state when total is 0, not an all-grey ring", () => {
    render(
      <DashboardPieChart
        slices={[]}
        total={0}
        isLoading={false}
        isError={false}
        onSliceClick={vi.fn()}
      />,
    );
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
  });

  it("default (no inlineLabels): renders the legend list, not outer labels, and clicking a legend row fires onSliceClick", () => {
    const onSliceClick = vi.fn();
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={onSliceClick}
      />,
    );

    expect(screen.getByText("Critical")).toBeInTheDocument();
    expect(screen.getByText("1 (25%)")).toBeInTheDocument();
    expect(screen.getByText("3 (75%)")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Critical"));
    expect(onSliceClick).toHaveBeenCalledWith(SLICES[0]);
  });

  it("default (no inlineLabels): wedge click still fires onSliceClick", () => {
    const onSliceClick = vi.fn();
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={onSliceClick}
      />,
    );

    fireEvent.click(screen.getByText("slice:Critical:1"));
    expect(onSliceClick).toHaveBeenCalledWith(SLICES[0]);
  });

  it("inlineLabels: renders each slice's own outer '{label} {value}' text, no percentage, and no separate legend list", () => {
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={vi.fn()}
        inlineLabels
      />,
    );

    expect(screen.getByText("Critical 1")).toBeInTheDocument();
    expect(screen.getByText("High 3")).toBeInTheDocument();
    // The default legend row's own "{value} ({pct}%)" text must not appear.
    expect(screen.queryByText("1 (25%)")).not.toBeInTheDocument();
    expect(screen.queryByText("3 (75%)")).not.toBeInTheDocument();
  });

  it("inlineLabels: clicking an outer label fires onSliceClick", () => {
    const onSliceClick = vi.fn();
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={onSliceClick}
        inlineLabels
      />,
    );

    fireEvent.click(screen.getByText("High 3"));
    expect(onSliceClick).toHaveBeenCalledWith(SLICES[1]);
  });

  it("inlineLabels: an outer label is keyboard-activatable (Enter) as its slice's sole accessible target", () => {
    const onSliceClick = vi.fn();
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={onSliceClick}
        inlineLabels
      />,
    );

    const label = screen.getByText("Critical 1");
    expect(label).toHaveAttribute("role", "button");
    expect(label).toHaveAttribute("tabindex", "0");
    fireEvent.keyDown(label, { key: "Enter" });
    expect(onSliceClick).toHaveBeenCalledWith(SLICES[0]);
  });

  it("inlineLabels: the wedge itself still fires onSliceClick too (unchanged from the default mode)", () => {
    const onSliceClick = vi.fn();
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={onSliceClick}
        inlineLabels
      />,
    );

    fireEvent.click(screen.getByText("slice:Critical:1"));
    expect(onSliceClick).toHaveBeenCalledWith(SLICES[0]);
  });

  it("inlineLabels: still renders the centered total, unchanged", () => {
    render(
      <DashboardPieChart
        slices={SLICES}
        total={4}
        isLoading={false}
        isError={false}
        onSliceClick={vi.fn()}
        inlineLabels
      />,
    );
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("Total")).toBeInTheDocument();
  });
});
