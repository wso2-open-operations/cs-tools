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

// Ported from the source app's components/CSPieChart.tsx — recharts instead
// of chart.js/react-chartjs-2 (see CSBarChart.tsx's header comment for why).
import { Cell, Legend, Pie, PieChart as RechartsPieChart, ResponsiveContainer, Tooltip } from "recharts";
import { COLOURS } from "../constants";

interface ChartProps {
  deploymentNames: string[];
  data: { [key: string]: number };
}

export default function CSPieChart({ deploymentNames, data }: ChartProps) {
  if (deploymentNames.length === 0 || Object.keys(data).length === 0) {
    return <p style={{ textAlign: "center" }}>No data available for deployment cases.</p>;
  }

  const colours = COLOURS.slice(0, deploymentNames.length);
  const rows = deploymentNames.map((name) => ({ name, value: data[name] || 0 }));

  return (
    <div style={{ width: "100%", height: 250 }}>
      <p style={{ fontWeight: 600, marginBottom: 4 }}>Cases by Deployment</p>
      <ResponsiveContainer width="100%" height="85%">
        <RechartsPieChart>
          <Pie data={rows} dataKey="value" nameKey="name" label={(entry) => `${entry.name} (${entry.value})`}>
            {rows.map((row, i) => (
              <Cell key={row.name} fill={colours[i % colours.length]} />
            ))}
          </Pie>
          <Tooltip formatter={(value, name) => [`${value} cases`, name]} />
          <Legend verticalAlign="bottom" />
        </RechartsPieChart>
      </ResponsiveContainer>
    </div>
  );
}
