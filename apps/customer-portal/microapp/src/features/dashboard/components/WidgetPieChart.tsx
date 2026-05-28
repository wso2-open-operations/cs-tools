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
import { Circle } from "@mui/icons-material";
import { Box, CardActionArea, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { PieChart as OxygenPieChart } from "@wso2/oxygen-ui-charts-react";

import { WidgetRoot } from "@features/dashboard/components";

export interface PieDataItem {
  id: string | number;
  label: string;
  value: number;
  color: string;
  [key: string]: string | number;
}

interface WidgetPieChartProps {
  title: string;
  data?: PieDataItem[];
  onClick?: (id: string | number, label: string) => void;
}

export function WidgetPieChart({ title, data, onClick }: WidgetPieChartProps) {
  const isLoading = !data;
  const total = data?.reduce((sum, item) => sum + item.value, 0) ?? 0;
  const isEmpty = !isLoading && total === 0;

  return (
    <WidgetRoot title={title}>
      <PieChart data={data} total={total} isEmpty={isEmpty} isLoading={isLoading} onClick={onClick} />
      <PieLegend data={data} isLoading={isLoading} onClick={onClick} />
    </WidgetRoot>
  );
}

interface PieChartProps {
  data?: PieDataItem[];
  total: number;
  isEmpty: boolean;
  isLoading: boolean;
  onClick?: WidgetPieChartProps["onClick"];
}

function PieChart({ data, total, isEmpty, isLoading, onClick }: PieChartProps) {
  return (
    <Box position="relative" display="flex" alignItems="center" justifyContent="center" width="100%">
      {isLoading ? (
        <Box display="flex" justifyContent="center" width="100%" py={1}>
          <Skeleton variant="circular" width={130} height={130} animation="wave" />
        </Box>
      ) : isEmpty ? (
        <Box
          sx={{
            width: 125,
            height: 125,
            borderRadius: "50%",
            border: "3px dashed",
            borderColor: "divider",
            my: 2,
            display: "grid",
            placeItems: "center",
          }}
        >
          <Typography
            variant="subtitle1"
            sx={{ fontWeight: "medium", color: "text.disabled", textAlign: "center", px: 2, opacity: 0.8 }}
          >
            N/A
          </Typography>
        </Box>
      ) : (
        <OxygenPieChart
          height={150}
          data={data}
          colors={data!.map((item) => item.color)}
          pies={[
            {
              nameKey: "label",
              dataKey: "value",
              innerRadius: "50%",
              paddingAngle: 0,
              onClick: (item: PieDataItem) => onClick?.(item.id, item.label),
            },
          ]}
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
            inset: 0,
            display: "grid",
            placeItems: "center",
            pointerEvents: "none",
          }}
        >
          {isLoading ? null : total}
        </Typography>
      )}
    </Box>
  );
}

interface PieLegendProps {
  data?: PieDataItem[];
  isLoading: boolean;
  onClick?: WidgetPieChartProps["onClick"];
}

function PieLegend({ data, isLoading, onClick }: PieLegendProps) {
  return (
    <Stack gap={0.5} mt={1}>
      {isLoading
        ? Array.from({ length: 5 }, (_, i) => (
            <Stack key={i} direction="row" justifyContent="space-between">
              <Stack direction="row" alignItems="center" gap={1}>
                <Skeleton variant="circular" width={12} height={12} />
                <Skeleton width={80} height={20} />
              </Stack>
              <Skeleton width={20} height={20} />
            </Stack>
          ))
        : data!.map((item) => (
            <CardActionArea
              key={item.id}
              disabled={!onClick}
              sx={{ display: "flex", justifyContent: "space-between" }}
              onClick={() => onClick?.(item.id, item.label)}
            >
              <Stack direction="row" alignItems="center" gap={1}>
                <Circle style={{ fontSize: 12, color: item.color }} />
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  {item.label}
                </Typography>
              </Stack>
              <Typography variant="subtitle2" color="text.secondary">
                {item.value}
              </Typography>
            </CardActionArea>
          ))}
    </Stack>
  );
}
