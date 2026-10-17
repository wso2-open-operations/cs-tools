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

import { Box, Typography } from "@wso2/oxygen-ui";
import { PieChart } from "@wso2/oxygen-ui-charts-react";
import { useReportChartColors } from "./useReportChartColors";

interface ChartProps {
  deploymentNames: string[];
  data: { [key: string]: number };
}

const CHART_HEIGHT = 300;

export default function CSPieChart({ deploymentNames, data }: ChartProps) {
  const colors = useReportChartColors();
  const rows = deploymentNames
    .map((name) => ({ name: `${name} (${data[name] || 0})`, value: data[name] || 0 }))
    .filter((row) => row.value > 0);

  return (
    <Box className="report-chart" sx={{ width: "100%", minWidth: 0 }}>
      <Typography component="p" variant="subtitle2" sx={{ fontWeight: 600, mb: 1 }}>
        Cases by deployment
      </Typography>
      {rows.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center", py: 4 }}>
          No data available for deployment cases.
        </Typography>
      ) : (
        <Box sx={{ width: "100%", height: CHART_HEIGHT }}>
          <PieChart
            height={CHART_HEIGHT}
            width="100%"
            data={rows}
            colors={colors}
            nameKey="name"
            innerRadius={58}
            outerRadius={86}
            paddingAngle={2}
            startAngle={90}
            endAngle={-270}
            isAnimationActive={false}
            pies={[{ dataKey: "value", nameKey: "name" }]}
            margin={{ top: 4, right: 4, bottom: 0, left: 4 }}
            legend={{ show: true, align: "center", verticalAlign: "bottom" }}
            tooltip={{ show: true }}
          />
        </Box>
      )}
    </Box>
  );
}
