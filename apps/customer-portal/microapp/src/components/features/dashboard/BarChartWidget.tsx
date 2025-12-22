import { BarChart } from "@mui/x-charts";
import { WidgetBox } from "@components/ui";

export interface BarSeriesConfig {
  dataKey: string;
  label: string;
  stack: string;
  color: string;
}

interface BarChartWidgetProps {
  title: string;
  data: Record<string, string | number>[];
  series: BarSeriesConfig[];
  xAxisKey?: string;
  height?: number;
}

export function BarChartWidget({ title, data, series, xAxisKey, height = 200 }: BarChartWidgetProps) {
  return (
    <WidgetBox title={title}>
      <BarChart
        dataset={data}
        xAxis={[{ scaleType: "band", dataKey: xAxisKey }]}
        series={series.map((s) => ({
          dataKey: s.dataKey,
          label: s.label,
          stack: s.stack,
          color: s.color,
        }))}
        height={height}
        margin={{ left: 0 }}
        slotProps={{
          legend: {
            direction: "horizontal",
            position: { vertical: "bottom", horizontal: "center" },
          },
        }}
      />
    </WidgetBox>
  );
}
