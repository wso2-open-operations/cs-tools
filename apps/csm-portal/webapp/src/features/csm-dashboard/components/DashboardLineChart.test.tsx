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
  // Renders each bar series' data, as LineChart below does for its lines.
  BarChart: ({
    data,
    bars,
  }: {
    data: Record<string, unknown>[];
    bars: {
      dataKey: string;
      name?: string;
      fill?: string;
      onClick?: (data: unknown, index: number, event?: unknown) => void;
    }[];
  }) => (
    <div>
      {bars.map((bar) => (
        <div key={bar.dataKey} data-testid={`bar-${bar.dataKey}`} data-fill={bar.fill}>
          {bar.name}: {data.map((row) => `${row.label}=${row[bar.dataKey]}`).join(", ")}
          {/* A clickable bar per row, as recharts calls Bar.onClick. */}
          {bar.onClick &&
            data.map((row, i) => (
              <button
                key={String(row.label)}
                type="button"
                data-testid={`bar-point-${bar.dataKey}-${i}`}
                onClick={(e) => bar.onClick?.(row, i, e)}
              />
            ))}
        </div>
      ))}
    </div>
  ),
  XAxis: () => null,
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

  it("labels week points by the days they cover and month points by month and year", () => {
    const weeks: CaseVolumeTrendChartPoint[] = [
      { date: "2025-12-01", endDate: "2025-12-07", opened: 2, closed: 0 },
      { date: "2025-12-29", endDate: "2025-12-31", opened: 1, closed: 3 },
    ];
    const { unmount } = render(
      <DashboardLineChart points={weeks} isLoading={false} isError={false} bucket="week" />,
    );
    expect(screen.getByTestId("line-opened")).toHaveTextContent("Opened: Dec 1-7=2, Dec 29-31=1");
    unmount();

    const acrossMonths: CaseVolumeTrendChartPoint[] = [
      { date: "2025-12-29", endDate: "2026-01-04", opened: 4, closed: 0 },
    ];
    const { unmount: unmountAcross } = render(
      <DashboardLineChart points={acrossMonths} isLoading={false} isError={false} bucket="week" />,
    );
    expect(screen.getByTestId("line-opened")).toHaveTextContent("Opened: Dec 29-Jan 4=4");
    unmountAcross();

    const months: CaseVolumeTrendChartPoint[] = [
      { date: "2025-11-01", endDate: "2025-11-30", opened: 2, closed: 0 },
      { date: "2025-12-01", endDate: "2025-12-31", opened: 7, closed: 1 },
    ];
    render(<DashboardLineChart points={months} isLoading={false} isError={false} bucket="month" />);
    expect(screen.getByTestId("line-opened")).toHaveTextContent("Opened: Nov 2025=2, Dec 2025=7");
  });

  it("reports a week or month point's last day too", () => {
    const onPointClick = vi.fn();
    const weeks: CaseVolumeTrendChartPoint[] = [
      { date: "2025-12-01", endDate: "2025-12-07", opened: 2, closed: 0 },
      { date: "2025-12-29", endDate: "2025-12-31", opened: 1, closed: 3 },
    ];
    render(
      <DashboardLineChart
        points={weeks}
        isLoading={false}
        isError={false}
        bucket="week"
        onPointClick={onPointClick}
      />,
    );

    fireEvent.click(screen.getByTestId("line-point-closed"));
    expect(onPointClick).toHaveBeenCalledWith("2025-12-29", "closed", "2025-12-31");
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

  describe("chart: bar", () => {
    it("draws a bar per point and series, Opened and Closed, instead of lines", () => {
      render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} chart="bar" />);
      expect(screen.getByTestId("bar-opened")).toHaveTextContent("Opened: Sep 1=3, Sep 2=2");
      expect(screen.getByTestId("bar-closed")).toHaveTextContent("Closed: Sep 1=1, Sep 2=4");
      expect(screen.queryByTestId(/^line-/)).not.toBeInTheDocument();
      expect(screen.getByText("Opened")).toBeInTheDocument();
      expect(screen.getByText("Closed")).toBeInTheDocument();
    });

    it("draws only the series asked for, in the lines' colors", () => {
      render(
        <DashboardLineChart points={POINTS} isLoading={false} isError={false} chart="bar" series={["closed"]} />,
      );
      expect(screen.queryByTestId("bar-opened")).not.toBeInTheDocument();
      const closed = screen.getByTestId("bar-closed");
      render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} series={["closed"]} />);
      expect(closed.getAttribute("data-fill")).toBe(screen.getByTestId("line-closed").getAttribute("data-stroke"));
    });

    it("labels month bars by month and year, as the line does", () => {
      const months: CaseVolumeTrendChartPoint[] = [
        { date: "2026-01-01", endDate: "2026-01-31", opened: 4, closed: 0 },
        { date: "2026-02-01", endDate: "2026-02-28", opened: 6, closed: 0 },
      ];
      render(
        <DashboardLineChart
          points={months}
          isLoading={false}
          isError={false}
          bucket="month"
          series={["opened"]}
          chart="bar"
        />,
      );
      expect(screen.getByTestId("bar-opened")).toHaveTextContent("Opened: Jan 2026=4, Feb 2026=6");
    });

    it("shows the empty state when the drawn series is all zero", () => {
      const onlyClosed: CaseVolumeTrendChartPoint[] = [{ date: "2026-09-01", opened: 0, closed: 5 }];
      render(
        <DashboardLineChart
          points={onlyClosed}
          isLoading={false}
          isError={false}
          series={["opened"]}
          chart="bar"
        />,
      );
      expect(screen.getByText("Nothing to show here right now")).toBeInTheDocument();
    });

    it("reports the clicked bar's day and series when bars are clickable", () => {
      const onPointClick = vi.fn();
      render(
        <DashboardLineChart
          points={POINTS}
          isLoading={false}
          isError={false}
          chart="bar"
          onPointClick={onPointClick}
        />,
      );
      fireEvent.click(screen.getByTestId("bar-point-closed-1"));
      expect(onPointClick).toHaveBeenCalledWith("2026-09-02", "closed");
      fireEvent.click(screen.getByTestId("bar-point-opened-0"));
      expect(onPointClick).toHaveBeenLastCalledWith("2026-09-01", "opened");
    });

    it("reports a week or month bar's last day too", () => {
      const onPointClick = vi.fn();
      const weeks: CaseVolumeTrendChartPoint[] = [
        { date: "2025-12-01", endDate: "2025-12-07", opened: 2, closed: 0 },
        { date: "2025-12-29", endDate: "2025-12-31", opened: 1, closed: 3 },
      ];
      render(
        <DashboardLineChart
          points={weeks}
          isLoading={false}
          isError={false}
          bucket="week"
          chart="bar"
          onPointClick={onPointClick}
        />,
      );
      fireEvent.click(screen.getByTestId("bar-point-opened-1"));
      expect(onPointClick).toHaveBeenCalledWith("2025-12-29", "opened", "2025-12-31");
    });

    it("has no clickable bars without onPointClick", () => {
      render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} chart="bar" />);
      expect(screen.queryByTestId(/^bar-point-/)).not.toBeInTheDocument();
    });

    it("draws lines for chart: line, as with no chart", () => {
      render(<DashboardLineChart points={POINTS} isLoading={false} isError={false} chart="line" />);
      expect(screen.getByTestId("line-opened")).toBeInTheDocument();
      expect(screen.queryByTestId(/^bar-/)).not.toBeInTheDocument();
    });
  });
});
