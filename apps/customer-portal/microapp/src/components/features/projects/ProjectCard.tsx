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

import { Box, Button, Card, Chip, Grid, Stack, Typography, alpha, pxToRem } from "@wso2/oxygen-ui";
import { ArrowRight, type LucideIcon } from "@wso2/oxygen-ui-icons-react";

import { PROJECT_METRIC_META, PROJECT_STATUS_META } from "@root/src/config/constants";

export type ProjectStatus = "All Good" | "Needs Attention";
export type ProjectType = "Managed Cloud" | "Regular";
export type ProjectMetricKey = "cases" | "chats" | "service" | "change" | "users" | "date";
export type ProjectMetricValue = number | string;
export type ProjectMetrics = Partial<Record<ProjectMetricKey, ProjectMetricValue>>;

export interface ProjectMetricMeta {
  label: string;
  icon: LucideIcon;
  color?: string;
}

export interface ProjectCardProps {
  id: string;
  name: string;
  description: string;
  type: ProjectType;
  status: ProjectStatus;
  numberOfOpenCases: number;
  metrics: ProjectMetrics;

  onClick?: () => void;
}

export function ProjectCard({ id, name, description, type, status, metrics, onClick }: ProjectCardProps) {
  const statusChipColorVariant = PROJECT_STATUS_META[status].color;

  return (
    <Card sx={{ bgcolor: "background.paper" }}>
      <Stack p={2} gap={1}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1.5}>
          <Typography variant="subtitle2">{id}</Typography>
          <Chip
            label={status}
            size="small"
            sx={(theme) => ({
              bgcolor: alpha(theme.palette[statusChipColorVariant].light, 0.1),
              color: theme.palette[statusChipColorVariant].light,
            })}
          />
        </Stack>
        <Typography variant="h6" mt={-0.8}>
          {name}
        </Typography>
        <Chip label={type} size="small" sx={{ alignSelf: "start" }} />
        <Typography variant="body2">{description}</Typography>
      </Stack>
      <Grid p={2} spacing={1.5} sx={{ bgcolor: "background.default" }} container>
        {Object.keys(metrics).map((key, index) => {
          const meta = PROJECT_METRIC_META[key as ProjectMetricKey];
          const value = metrics[key as ProjectMetricKey];

          if (value === undefined) return null;

          return (
            <Grid key={index} size={{ xs: 6 }}>
              <MetricItem meta={meta} value={value} />
            </Grid>
          );
        })}
      </Grid>
      <Box component={Stack} p={2} pt={3}>
        <Button
          variant="contained"
          sx={{ display: "flex", textTransform: "initial", width: "100%", gap: 1 }}
          onClick={onClick}
        >
          View Dashboard
          <ArrowRight size={pxToRem(18)} />
        </Button>
      </Box>
    </Card>
  );
}

function MetricItem({ meta, value }: { meta: ProjectMetricMeta; value: ProjectMetricValue }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1}>
      <Box color="text.secondary">
        <meta.icon size={pxToRem(18)} />
      </Box>
      <Typography variant="body2" fontWeight="regular">
        {meta.label}
      </Typography>
      <Typography variant="body2" fontWeight="regular" color={meta.color}>
        {value}
      </Typography>
    </Stack>
  );
}
