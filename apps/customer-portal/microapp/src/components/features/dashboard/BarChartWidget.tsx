import { BarChart } from "@wso2/oxygen-ui-charts-react";
import { WidgetBox } from "@components/ui";
import { Box } from "@wso2/oxygen-ui";

export interface BarSeriesConfig {
  dataKey: string;
  name: string;
  stackId: string;
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
      <Box mt={2}>
        <BarChart
          height={height}
          data={data}
          colors={series.map((item) => item.color)}
          xAxisDataKey={xAxisKey}
          margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
          bars={series}
          tooltip={{ show: false }}
          grid={{ show: false }}
        />
      </Box>
    </WidgetBox>
  );
}
