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
