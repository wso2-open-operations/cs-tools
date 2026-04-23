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

import { Box, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { PieChart } from "@wso2/oxygen-ui-charts-react";
import { Circle } from "@mui/icons-material";
import { WidgetBox } from "@components/ui";

export interface PieDataItem {
  label: string;
  value: number;
  color: string;

  [key: string]: string | number;
}

interface PieChartWidgetProps {
  title: string;
  data?: PieDataItem[];
}

export function PieChartWidget({ title, data }: PieChartWidgetProps) {
  const loading = !data;
  const total = data?.reduce((sum, item) => sum + item.value, 0);
  const isEmpty = !loading && total === 0;

  return (
    <WidgetBox title={title}>
      <Box position="relative" display="flex" alignItems="center" justifyContent="center" width="100%">
        {loading ? (
          <Box display="flex" justifyContent="center" width="100%" py={1}>
            <Skeleton variant="circular" width={130} height={130} animation="wave" />
          </Box>
        ) : isEmpty ? (
          <Box
            sx={{
              width: 130,
              height: 130,
              borderRadius: "50%",
              border: "3px dashed",
              borderColor: "divider",
              my: 2,
            }}
          />
        ) : (
          <PieChart
            height={150}
            data={data}
            colors={data.map((item) => item.color)}
            pies={[{ nameKey: "label", dataKey: "value", innerRadius: "50%", paddingAngle: 0 }]}
            margin={{ top: 0, right: 0, bottom: 0, left: 0 }}
            legend={{ show: false }}
            tooltip={{ show: false }}
            isAnimationActive={false}
          />
        )}

        {!isEmpty && (
          <Typography
            variant="h5"
            sx={{
              position: "absolute",
              top: 0,
              left: 0,
              height: "100%",
              width: "100%",
              display: "grid",
              placeItems: "center",
            }}
          >
            {total}
          </Typography>
        )}
      </Box>

      <Stack gap={0.5} mt={1}>
        {loading
          ? [...Array(5)].map((_, i) => (
              <Stack key={i} direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Skeleton variant="circular" width={12} height={12} />
                  <Skeleton width={80} height={20} />
                </Stack>
                <Skeleton width={20} height={20} />
              </Stack>
            ))
          : data.map((item, index) => (
              <Stack key={index} direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={{ fontSize: 12, color: item.color }} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    {item.label}
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" color="text.secondary">
                  {item.value}
                </Typography>
              </Stack>
            ))}
      </Stack>
    </WidgetBox>
  );
}
