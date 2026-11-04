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
import type { JSX } from "react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Recharts' ResponsiveContainer (underneath the library's own LineChart)
// measures a real layout size, always 0 under jsdom — stubbed to a plain
// stand-in that renders enough of its own props to assert on, same approach
// DashboardPieChart.test.tsx uses for this same package.
vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  LineChart: ({
    data,
    lines,
  }: {
    data: Record<string, unknown>[];
    lines: {
      dataKey: string;
      name?: string;
      stroke?: string;
      activeDot?: boolean | ((props: { cx?: number; cy?: number; index?: number }) => JSX.Element);
    }[];
  }) => (
    <div>
      {lines.map((line) => (
        <div key={line.dataKey} data-testid={`line-${line.dataKey}`} data-stroke={line.stroke}>
          {line.name}: {data.map((row) => `${row.label}=${row[line.dataKey]}`).join(", ")}
          {/* The hovered point's dot, as recharts would draw it for row 1. */}
          {typeof line.activeDot === "function" && <svg>{line.activeDot({ cx: 10, cy: 20, index: 1 })}</svg>}
        </div>
      ))}
    </div>
  ),
}));

import DashboardLineChart from "@features/csm-dashboard/components/DashboardLineChart";
import type { CaseVolumeTrendChartPoint } from "@features/csm-dashboard/api/useCaseVolumeTrendData";

const POINTS: CaseVolumeTrendChartPoint[] = [
  { date: "2026-09-01", opened: 3, closed: 1 },
  { date: "2026-09-02", opened: 2, closed: 4 },
];

describe("DashboardLineChart", () => {
  it("renders a skeleton while loading", () => {
    const { container } = render(
      <DashboardLineChart points={[]} isLoading={true} isError={false} />,
    );
    expect(container.querySelectorAll(".MuiSkeleton-root").length).toBe(1);
  });

  it("renders its error state instead of the chart", () => {
    render(<DashboardLineChart points={[]} isLoading={false} isError={true} />);
    expect(screen.getByText("Could not load this widget.")).toBeInTheDocument();
  });

  it("renders an empty state when there are no points at all", () => {
    render(<DashboardLineChart points={[]} isLoading={false} isError={false} />);
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
  });

  it("renders an empty state when every point is zero across both series", () => {
    render(
      <DashboardLineChart
        points={[
          { date: "2026-09-01", opened: 0, closed: 0 },
          { date: "2026-09-02", opened: 0, closed: 0 },
        ]}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
  });

  it("renders two lines, Opened and Closed, with their own day-formatted data", () => {
    render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} />);

    expect(screen.getByTestId("line-opened")).toHaveTextContent(
      "Opened: Sep 1=3, Sep 2=2",
    );
    expect(screen.getByTestId("line-closed")).toHaveTextContent(
      "Closed: Sep 1=1, Sep 2=4",
    );
    expect(screen.getAllByText(/^(Opened|Closed)$/)).toHaveLength(2);
  });

  it.each([
    ["opened", "Opened", "closed", "Closed"],
    ["closed", "Closed", "opened", "Opened"],
  ] as const)("draws only the %s line, and only its legend entry, when series names it", (key, label, otherKey, otherLabel) => {
    render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} series={[key]} />);

    expect(screen.getByTestId(`line-${key}`)).toBeInTheDocument();
    expect(screen.queryByTestId(`line-${otherKey}`)).not.toBeInTheDocument();
    expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.queryByText(otherLabel)).not.toBeInTheDocument();
  });

  it("draws both lines, Opened first, when series names both in any order", () => {
    render(
      <DashboardLineChart points={POINTS} isLoading={false} isError={false} series={["closed", "opened"]} />,
    );
    const lines = screen.getAllByTestId(/^line-/).map((el) => el.getAttribute("data-testid"));
    expect(lines).toEqual(["line-opened", "line-closed"]);
  });

  it("reports the clicked point's day and line when points are clickable", () => {
    const onPointClick = vi.fn();
    render(
      <DashboardLineChart points={POINTS} isLoading={false} isError={false} onPointClick={onPointClick} />,
    );

    fireEvent.click(screen.getByTestId("line-point-closed"));
    expect(onPointClick).toHaveBeenCalledWith("2026-09-02", "closed");
    fireEvent.click(screen.getByTestId("line-point-opened"));
    expect(onPointClick).toHaveBeenLastCalledWith("2026-09-02", "opened");
  });

  it("makes only the drawn line's points clickable", () => {
    const onPointClick = vi.fn();
    render(
      <DashboardLineChart
        points={POINTS}
        isLoading={false}
        isError={false}
        series={["opened"]}
        onPointClick={onPointClick}
      />,
    );
    expect(screen.getByTestId("line-point-opened")).toBeInTheDocument();
    expect(screen.queryByTestId("line-point-closed")).not.toBeInTheDocument();
  });

  it("has no clickable points without onPointClick", () => {
    render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} />);
    expect(screen.queryByTestId(/^line-point-/)).not.toBeInTheDocument();
  });

  it("counts only the drawn line for the empty state", () => {
    const onlyClosed = [
      { date: "2026-09-01", opened: 0, closed: 2 },
      { date: "2026-09-02", opened: 0, closed: 1 },
    ];
    const { unmount } = render(
      <DashboardLineChart points={onlyClosed} isLoading={false} isError={false} series={["opened"]} />,
    );
    expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
    unmount();

    render(<DashboardLineChart points={onlyClosed} isLoading={false} isError={false} series={["closed"]} />);
    expect(screen.getByTestId("line-closed")).toBeInTheDocument();
  });
});
