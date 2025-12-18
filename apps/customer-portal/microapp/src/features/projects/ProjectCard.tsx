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

import { ArrowForward, type SvgIconComponent } from "@mui/icons-material";
import { Box, ButtonBase as Button, Card, Chip, Grid, Stack, Typography } from "@mui/material";
import { PROJECT_METRIC_META, PROJECT_STATUS_META, PROJECT_TYPE_META } from "@root/src/config/constants";

export type ProjectStatus = "All Good" | "Needs Attention";
export type ProjectType = "Managed Cloud" | "Regular";
export type ProjectMetricKey = "cases" | "chats" | "service" | "change" | "users" | "date";
export type ProjectMetricValue = number | string;
export type ProjectMetrics = Partial<Record<ProjectMetricKey, ProjectMetricValue>>;

export interface ProjectMetricMeta {
  label: string;
  icon: SvgIconComponent;
  color?: string;
}

export interface ProjectCardProps {
  id: string;
  name: string;
  description: string;
  type: ProjectType;
  status: ProjectStatus;
  metrics: ProjectMetrics;
}

export function ProjectCard({ id, name, description, type, status, metrics }: ProjectCardProps) {
  const TypeChipIcon = PROJECT_TYPE_META[type].icon;
  const StatusChipIcon = PROJECT_STATUS_META[status].icon;
  const statusChipColorVariant = PROJECT_STATUS_META[status].color;

  return (
    <Card elevation={0} sx={(theme) => ({ borderRadius: 3, border: `1px solid ${theme.palette.divider}` })}>
      <Stack p={2} gap={1}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1.5}>
          <Typography variant="subtitle2" fontWeight="regular">
            {id}
          </Typography>
          <Chip
            label={status}
            size="small"
            color={statusChipColorVariant}
            icon={<StatusChipIcon />}
            iconPosition="end"
          />
        </Stack>
        <Typography variant="h6" mt={-0.8}>
          {name}
        </Typography>
        <Chip label={type} size="small" icon={<TypeChipIcon />} sx={{ alignSelf: "start", borderRadius: 1 }} />
        <Typography variant="body2">{description}</Typography>
      </Stack>
      <Grid bgcolor="semantic.portal.background.secondary" p={2} spacing={1.5} container>
        {Object.keys(metrics).map((key) => {
          const meta = PROJECT_METRIC_META[key as ProjectMetricKey];
          const value = metrics[key as ProjectMetricKey];

          if (value === undefined) return null;

          return (
            <Grid size={{ xs: 6 }}>
              <MetricItem meta={meta} value={value} />
            </Grid>
          );
        })}
      </Grid>
      <Box p={3}>
        <Button variant="contained" sx={{ width: "100%", fontWeight: "bold" }}>
          View Dashboard
          <ArrowForward sx={(theme) => ({ fontSize: theme.typography.pxToRem(20) })} />
        </Button>
      </Box>
    </Card>
  );
}

function MetricItem({ meta, value }: { meta: ProjectMetricMeta; value: ProjectMetricValue }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1}>
      <meta.icon sx={(theme) => ({ color: "text.secondary", fontSize: theme.typography.pxToRem(20) })} />
      <Typography variant="subtitle1" fontWeight="regular">
        {meta.label}
      </Typography>
      <Typography variant="subtitle1" fontWeight="regular" color={meta.color}>
        {value}
      </Typography>
    </Stack>
  );
}
