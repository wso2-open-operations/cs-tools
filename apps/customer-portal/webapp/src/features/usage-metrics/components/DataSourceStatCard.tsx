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

import { Box, Card, Divider, Skeleton, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type { DataSourceStatCardProps } from "@features/usage-metrics/types/usageMetrics";
import {
  USAGE_METRICS_STAT_AVG,
  USAGE_METRICS_STAT_MAX,
  USAGE_METRICS_STAT_MIN,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import { formatUsageMetricCount } from "@features/project-details/utils/usageMetrics";

function StatTile({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <Box sx={{ textAlign: "center" }}>
      <Typography variant="caption" color="text.secondary" display="block">
        {label}
      </Typography>
      <Typography variant="body2" fontWeight={600}>
        {value}
      </Typography>
    </Box>
  );
}

export default function DataSourceStatCard({
  summary,
  isLoading,
}: DataSourceStatCardProps): JSX.Element {
  if (isLoading) {
    return (
      <Card variant="outlined" sx={{ p: 2 }}>
        <Skeleton variant="text" width={120} height={24} sx={{ mb: 1 }} />
        <Skeleton variant="text" width={80} height={40} sx={{ mb: 1.5 }} />
        <Skeleton variant="rounded" height={32} />
      </Card>
    );
  }

  return (
    <Card variant="outlined" sx={{ p: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 0.5 }}>
        {summary.label}
      </Typography>
      <Typography variant="h5" fontWeight={700} sx={{ mb: 1.5 }}>
        {formatUsageMetricCount(summary.curr)}
      </Typography>
      <Divider sx={{ mb: 1.5 }} />
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: 1,
        }}
      >
        <StatTile label={USAGE_METRICS_STAT_MIN} value={formatUsageMetricCount(summary.min)} />
        <StatTile label={USAGE_METRICS_STAT_MAX} value={formatUsageMetricCount(summary.max)} />
        <StatTile
          label={USAGE_METRICS_STAT_AVG}
          value={formatUsageMetricCount(Math.round(summary.avg))}
        />
      </Box>
    </Card>
  );
}
