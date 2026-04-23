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

import { Box, Button, Card, Grid, Skeleton, Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import { ArrowRight, type LucideIcon } from "@wso2/oxygen-ui-icons-react";

import { PROJECT_METRIC_META } from "@root/src/config/constants";
import type { Project, ProjectMetricKey, ProjectMetricValue } from "@src/types";

export interface ProjectMetricMeta {
  label: string;
  icon: LucideIcon;
  color?: string;
}

export function ProjectCard({
  projectKey,
  name,
  description,
  type,
  status,
  metrics,
  onClick,
}: Project & { onClick?: () => void }) {
  return (
    <Card sx={{ bgcolor: "background.paper" }}>
      <Stack p={2} gap={1}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1.5}>
          <Typography variant="subtitle2">{projectKey}</Typography>
        </Stack>
        <Typography variant="h6" mt={-0.8}>
          {name}
        </Typography>
      </Stack>
      <Grid p={2} spacing={1.5} sx={{ bgcolor: "background.default" }} container>
        {Object.keys(metrics).map((key, index) => {
          const meta = PROJECT_METRIC_META[key as ProjectMetricKey];
          const value = metrics[key as ProjectMetricKey];

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

function MetricItem({ meta, value }: { meta: ProjectMetricMeta; value?: ProjectMetricValue }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1}>
      <Box color="text.secondary">
        <meta.icon size={pxToRem(18)} />
      </Box>
      <Typography variant="body2" fontWeight="regular">
        {meta.label}
      </Typography>
      <Typography variant="body2" fontWeight="regular" color={meta.color}>
        {value ?? <Skeleton width={30} height={20} />}
      </Typography>
    </Stack>
  );
}

export function ProjectCardSkeleton() {
  return (
    <Card sx={{ bgcolor: "background.paper" }}>
      <Stack p={2} gap={1}>
        <Stack direction="row" justifyContent="space-between" alignItems="center" gap={1.5}>
          <Skeleton variant="text" width="30%" sx={{ fontSize: "subtitle2.fontSize" }} />
          <Skeleton variant="rounded" width={80} height={22} />
        </Stack>

        <Skeleton variant="text" width="100%" height={32} sx={{ mt: -0.8 }} />

        <Skeleton variant="rounded" width={60} height={22} sx={{ alignSelf: "start" }} />

        <Box>
          <Skeleton variant="text" width="100%" />
          <Skeleton variant="text" width="80%" />
        </Box>
      </Stack>

      <Grid p={2} spacing={1.5} sx={{ bgcolor: "background.default" }} container>
        {[...Array(2)].map((_, index) => (
          <Grid key={index} size={{ xs: 6 }}>
            <Stack gap={0.5}>
              <Skeleton variant="text" width="40%" height={14} />
              <Skeleton variant="text" width="60%" height={24} />
            </Stack>
          </Grid>
        ))}
      </Grid>

      <Box p={2} pt={3}>
        <Skeleton variant="rounded" width="100%" height={36} />
      </Box>
    </Card>
  );
}
