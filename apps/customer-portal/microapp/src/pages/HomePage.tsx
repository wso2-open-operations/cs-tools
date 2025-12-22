// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { CheckCircle, Circle, Report, TrendingDown, TrendingUp, WatchLater } from "@mui/icons-material";
import { Box, Card, Grid, Stack, styled, Typography } from "@mui/material";
import { BarChart, useDrawingArea } from "@mui/x-charts";
import { PieChart } from "@mui/x-charts/PieChart";
import type { ReactNode } from "react";

export default function HomePage() {
  const data = [
    { id: 0, value: 1, label: "Critical (P1)", color: "#FF4522" },
    { id: 1, value: 4, label: "High (P2)", color: "#FF8C00" },
    { id: 2, value: 7, label: "Medium (P3)", color: "#4D53E8" },
  ];

  const dataset = [
    { year: "2020", acme: 40, bites: 35, cupertino: 25, dunlop: 20 },
    { year: "2021", acme: 55, bites: 40, cupertino: 35, dunlop: 30 },
    { year: "2022", acme: 65, bites: 50, cupertino: 40, dunlop: 35 },
    { year: "2023", acme: 80, bites: 60, cupertino: 50, dunlop: 45 },
    { year: "2024", acme: 95, bites: 75, cupertino: 65, dunlop: 55 },
    { year: "2025", acme: 105, bites: 85, cupertino: 75, dunlop: 65 },
  ];

  return (
    <Box p={2} mb={20}>
      <Grid spacing={1.5} container>
        <Grid size={6}>
          <MetricTile
            label="Active Cases"
            value={10}
            trend={{ direction: "up", value: "+10%" }}
            icon={<Report sx={{ color: "semantic.portal.accent.orange" }} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricTile
            label="All Cases"
            value={25}
            trend={{ direction: "up", value: "+3%" }}
            icon={<Report sx={{ color: "semantic.portal.accent.yellow" }} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricTile
            label="Resolved This Month"
            value={47}
            trend={{ direction: "up", value: "+18%" }}
            icon={<CheckCircle sx={{ color: "semantic.portal.accent.green" }} />}
          />
        </Grid>
        <Grid size={6}>
          <MetricTile
            label="Average Response Time"
            value="2.4H"
            trend={{ direction: "down", value: "-15%" }}
            icon={<WatchLater sx={{ color: "semantic.portal.accent.purple" }} />}
          />
        </Grid>
        <Grid size={6}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h6" fontWeight="medium" color="text.secondary">
              Outstanding Incidents
            </Typography>
            <PieChart
              series={[
                {
                  paddingAngle: 2,
                  innerRadius: "50%",
                  outerRadius: "90%",
                  data,
                },
              ]}
              hideLegend
            >
              <PieCenterLabel>12</PieCenterLabel>
            </PieChart>
            <Stack gap={0.5} mt={1}>
              <Stack direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(12), color: "#FF4522" })} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    Critical (P1)
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  1
                </Typography>
              </Stack>
              <Stack direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(12), color: "#FF8C00" })} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    High (P2)
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  4
                </Typography>
              </Stack>
              <Stack direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(12), color: "#4D53E8" })} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    Medium (P3)
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  7
                </Typography>
              </Stack>
            </Stack>
          </Card>
        </Grid>
        <Grid size={6}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h6" fontWeight="medium" color="text.secondary">
              Active Cases
            </Typography>
            <PieChart
              series={[
                {
                  paddingAngle: 2,
                  innerRadius: "50%",
                  outerRadius: "90%",
                  data,
                },
              ]}
              hideLegend
            >
              <PieCenterLabel>12</PieCenterLabel>
            </PieChart>
            <Stack gap={0.5} mt={1}>
              <Stack direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(12), color: "#FF4522" })} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    Critical (P1)
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  1
                </Typography>
              </Stack>
              <Stack direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(12), color: "#FF8C00" })} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    High (P2)
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  4
                </Typography>
              </Stack>
              <Stack direction="row" justifyContent="space-between">
                <Stack direction="row" alignItems="center" gap={1}>
                  <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(12), color: "#4D53E8" })} />
                  <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                    Medium (P3)
                  </Typography>
                </Stack>
                <Typography variant="subtitle2" fontWeight="regular" color="text.secondary">
                  7
                </Typography>
              </Stack>
            </Stack>
          </Card>
        </Grid>
        <Grid size={12}>
          <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
            <Typography variant="h6" fontWeight="medium" color="text.secondary">
              Cases Trend
            </Typography>
            <BarChart
              dataset={dataset}
              xAxis={[{ scaleType: "band", dataKey: "year" }]}
              series={[
                { dataKey: "acme", label: "Acme", stack: "total", color: "#4D53E8" },
                { dataKey: "bites", label: "Bites", stack: "total", color: "#14A9C1" },
                { dataKey: "cupertino", label: "CupertinoHQ", stack: "total", color: "#E50051" },
                { dataKey: "dunlop", label: "Dunlop", stack: "total", color: "#FF8C00" },
              ]}
              height={200}
              margin={{ left: 0 }}
              slotProps={{
                legend: {
                  direction: "horizontal",
                  position: { vertical: "bottom", horizontal: "center" },
                },
              }}
            />
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
}

const StyledText = styled("text")(({ theme }) => ({
  fill: theme.palette.text.secondary,
  textAnchor: "middle",
  dominantBaseline: "central",
  fontSize: 20,
  fontWeight: 500,
}));

function PieCenterLabel({ children }: { children: React.ReactNode }) {
  const { width, height, left, top } = useDrawingArea();
  return (
    <StyledText x={left + width / 2} y={top + height / 2}>
      {children}
    </StyledText>
  );
}

interface MetricTileProps {
  label: string;
  value: number | string;
  icon: ReactNode;
  trend?: {
    direction: "up" | "down";
    value: number | string;
  };
}

export function MetricTile({ label, value, icon, trend }: MetricTileProps) {
  const TrendIcon = trend?.direction === "up" ? TrendingUp : TrendingDown;

  return (
    <Card component={Stack} p={1.2} gap={0.5} elevation={0}>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        {icon}

        {trend && (
          <Stack direction="row" gap={0.5} alignItems="center">
            <TrendIcon sx={{ color: "semantic.portal.accent.green" }} />
            <Typography variant="body2" fontWeight="medium" sx={{ color: "semantic.portal.accent.green" }}>
              {trend.value}
            </Typography>
          </Stack>
        )}
      </Stack>

      <Typography variant="h3" fontWeight="bold">
        {value}
      </Typography>

      <Typography variant="h6" fontWeight="medium" color="text.secondary">
        {label}
      </Typography>
    </Card>
  );
}
