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

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Card,
  Grid,
  Skeleton,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  Code2,
  Monitor,
  Package,
  Server,
} from "@wso2/oxygen-ui-icons-react";
import EmptyState from "@components/empty-state/EmptyState";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { useMemo } from "react";
import {
  USAGE_LINE_CHART_MARGIN,
  USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR,
  USAGE_METRICS_INSTANCE_OS,
  USAGE_METRICS_INSTANCE_JAVA,
  USAGE_METRICS_INSTANCE_U2,
  USAGE_METRICS_NO_INSTANCE_DATA,
  USAGE_METRICS_NO_PRODUCTS_IN_DEPLOYMENT,
  USAGE_METRICS_PRODUCT_INSTANCES_SECTION,
  USAGE_METRICS_PRODUCT_INSTANCE_METRICS,
  USAGE_METRICS_PRODUCT_CORE_METRICS,
  USAGE_METRICS_STAT_LABEL_AVG,
  USAGE_METRICS_STAT_LABEL_MIN,
  USAGE_METRICS_STAT_LABEL_MAX,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import { UsageChartSurface } from "@features/usage-metrics/components/UsageChartSurface";
import usePostDeploymentInstancesUsagesSearch from "@features/project-details/api/usePostDeploymentInstancesUsagesSearch";
import usePostDeploymentInstancesMetricsSearch from "@features/project-details/api/usePostDeploymentInstancesMetricsSearch";
import type { CurrMinMaxAvg, UsageProductInstanceRow } from "@features/project-details/types/usage";
import type {
  MetricPillProps,
  ProductAccordionRowProps,
  ProductExpandedViewProps,
  UsageEnvironmentProductsPanelProps,
} from "@features/usage-metrics/types/usageMetrics";
import { getUsageEnvironmentPanelAccent } from "@features/usage-metrics/utils/usageMetricsAccent";
import { deriveUsageEnvironmentProducts } from "@features/usage-metrics/utils/usageMetricsEnvironmentProducts";

// ─── Per-product expanded view ────────────────────────────────────────────────

function ProductExpandedView({ product }: ProductExpandedViewProps): JSX.Element {
  const instances = product.instances;

  if (!instances || instances.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {USAGE_METRICS_NO_INSTANCE_DATA}
      </Typography>
    );
  }

  const chartGridSize =
    product.chartTrends.length === 1 ? 12
    : product.chartTrends.length === 3 ? 4
    : 6;

  return (
    <>
      <Grid
        container
        spacing={2}
        sx={{ width: "100%", minWidth: 0, overflowX: "hidden" }}
      >
        {product.chartTrends.map((trend) => (
          <Grid key={trend.title} size={{ xs: 12, lg: chartGridSize }}>
            <Card sx={{ p: 2, borderRadius: 0 }}>
              <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
                {trend.title}
              </Typography>
              <UsageChartSurface minHeight={200}>
                <LineChart
                  data={trend.data}
                  xAxisDataKey="name"
                  height={200}
                  width="100%"
                  margin={USAGE_LINE_CHART_MARGIN}
                  accessibilityLayer={false}
                  xAxis={{ interval: Math.max(0, Math.ceil(trend.data.length / 6) - 1) }}
                  legend={{ show: false }}
                  grid={{ show: true, strokeDasharray: "3 3" }}
                  lines={[
                    {
                      dataKey: "value",
                      name: trend.title,
                      stroke: trend.stroke,
                      strokeWidth: 2.5,
                      dot: false,
                    },
                  ]}
                />
              </UsageChartSurface>
            </Card>
          </Grid>
        ))}
      </Grid>

      <Box sx={{ mt: 2 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>
          {USAGE_METRICS_PRODUCT_INSTANCES_SECTION} ({instances.length})
        </Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {instances.map((inst) => (
            <InstanceRow key={inst.id} instance={inst} />
          ))}
        </Box>
      </Box>
    </>
  );
}

// ─── Flat instance row ────────────────────────────────────────────────────────

function InstanceRow({
  instance,
}: {
  instance: UsageProductInstanceRow;
}): JSX.Element {
  const wellBg = alpha(colors.grey?.[500] ?? "#6B7280", 0.04);

  return (
    <Box
      sx={{
        px: 2,
        py: 1.5,
        display: "flex",
        flexDirection: { xs: "column", lg: "row" },
        alignItems: { xs: "stretch", lg: "center" },
        justifyContent: "space-between",
        gap: 2,
        border: "1px solid",
        borderColor: "divider",
        bgcolor: wellBg,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1.5,
          minWidth: { lg: 200 },
        }}
      >
        <Server size={18} color={colors.grey?.[500]} />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {instance.hostName}
        </Typography>
      </Box>
      <Box
        sx={{
          display: "flex",
          flexWrap: "wrap",
          gap: 5,
          alignItems: "center",
          justifyContent: { xs: "flex-start", lg: "flex-end" },
          flex: 1,
          minWidth: 0,
        }}
      >
        <MetricPill
          icon={<Monitor size={16} color={colors.grey?.[400]} />}
          label={USAGE_METRICS_INSTANCE_OS}
          value={instance.os}
        />
        <MetricPill
          icon={<Code2 size={16} color={colors.grey?.[400]} />}
          label={USAGE_METRICS_INSTANCE_JAVA}
          value={instance.javaVersion}
        />
        <MetricPill
          icon={<Package size={16} color={colors.grey?.[400]} />}
          label={USAGE_METRICS_INSTANCE_U2}
          value={instance.u2Level}
        />
      </Box>
    </Box>
  );
}

// ─── Curr/Avg/Min/Max metric block ────────────────────────────────────────────

function CurrMinMaxBlock({
  label,
  summary,
}: {
  label: string;
  summary: CurrMinMaxAvg;
}): JSX.Element {
  const cols = [
    { l: USAGE_METRICS_STAT_LABEL_AVG, v: summary.avg, highlight: false },
    { l: USAGE_METRICS_STAT_LABEL_MIN, v: summary.min, highlight: false },
    { l: USAGE_METRICS_STAT_LABEL_MAX, v: summary.max, highlight: false },
  ];

  return (
    <Box>
      <Typography
        variant="body2"
        sx={{ fontWeight: 700, display: "block", mb: 1 }}
      >
        {label}
      </Typography>
      <Box sx={{ display: "flex", gap: 2 }}>
        {cols.map(({ l, v, highlight }) => (
          <Box key={l} sx={{ textAlign: "left" }}>
            <Typography
              variant="caption"
              color="text.secondary"
              display="block"
              sx={{ mb: 0.25 }}
            >
              {l}
            </Typography>
            <Typography
              variant="body1"
              sx={{
                fontWeight: 700,
                color: highlight ? "primary.main" : "text.primary",
              }}
            >
              {v}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

// ─── Product accordion row ────────────────────────────────────────────────────

function ProductAccordionRow({
  product,
  deploymentId,
  expanded,
  onToggle,
}: ProductAccordionRowProps): JSX.Element {
  const a = getUsageEnvironmentPanelAccent(deploymentId);

  return (
    <Accordion
      expanded={expanded}
      onChange={() => onToggle()}
      disableGutters
      elevation={0}
      sx={{
        "&:before": { display: "none" },
        boxShadow: 1,
        overflow: "hidden",
        border: "1px solid",
        borderColor: "divider",
        borderRadius: 0,
        bgcolor: "background.paper",
        "&:hover": { bgcolor: "action.hover" },
        "&.Mui-expanded": { margin: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={<ChevronDown size={20} color={a.iconColor} />}
        sx={{
          px: 2,
          py: 2,
          minHeight: 56,
          bgcolor: a.headerBg,
          borderRadius: 0,
          "&:hover": { bgcolor: a.headerHoverBg },
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", lg: "row" },
            alignItems: { xs: "stretch", lg: "center" },
            gap: 2,
            width: "100%",
            minWidth: 0,
            pr: 1,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              minWidth: { lg: 240 },
            }}
          >
            <Box
              sx={{
                p: 1.5,
                bgcolor: a.iconWellBg,
                display: "flex",
                flexShrink: 0,
              }}
            >
              <Package size={20} color={a.iconColor} />
            </Box>
            <Box sx={{ textAlign: "left", minWidth: 0 }}>
              <Typography variant="body2" sx={{ fontWeight: 700 }}>
                {product.name}
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{
              flex: 1,
              minWidth: 0,
              display: "flex",
              flexWrap: "wrap",
              justifyContent: "flex-end",
              alignItems: "flex-start",
              gap: 5,
            }}
          >
            {product.summaryStats.map((stat) => (
              <Box key={stat.label}>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  display="block"
                  sx={{ mb: 0.5 }}
                >
                  {stat.label}
                </Typography>
                <Typography variant="body1" sx={{ fontWeight: 700 }}>
                  {stat.value}
                </Typography>
              </Box>
            ))}
            <CurrMinMaxBlock
              label={USAGE_METRICS_PRODUCT_INSTANCE_METRICS}
              summary={product.instanceSummary}
            />
            <CurrMinMaxBlock
              label={USAGE_METRICS_PRODUCT_CORE_METRICS}
              summary={product.coreSummary}
            />
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails
        sx={{
          px: 2,
          py: 2,
          bgcolor: "background.paper",
          borderTop: 1,
          borderColor: "divider",
          borderRadius: 0,
        }}
      >
        <ProductExpandedView product={product} />
      </AccordionDetails>
    </Accordion>
  );
}

function MetricPill({ icon, label, value }: MetricPillProps): JSX.Element {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
      {icon}
      <Box>
        <Typography variant="caption" color="text.secondary">
          {label}
        </Typography>
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          {value}
        </Typography>
      </Box>
    </Box>
  );
}

export default function UsageEnvironmentProductsPanel({
  deploymentId,
  dateRange,
  expandedProductIds,
  onToggleProduct,
}: UsageEnvironmentProductsPanelProps): JSX.Element {
  const metricsPayload = useMemo(
    () => ({
      filters: { startDate: dateRange.startDate, endDate: dateRange.endDate },
    }),
    [dateRange],
  );

  const {
    data: usagesData,
    isLoading: usagesLoading,
    isError: usagesError,
  } = usePostDeploymentInstancesUsagesSearch(deploymentId, metricsPayload);

  const {
    data: metricsData,
    isLoading: metricsLoading,
    isError: metricsError,
  } = usePostDeploymentInstancesMetricsSearch(deploymentId, metricsPayload);

  const isLoading = usagesLoading || metricsLoading;
  const isError = usagesError || metricsError;

  const products = useMemo(() => {
    if (!usagesData || !metricsData) return [];
    return deriveUsageEnvironmentProducts(usagesData.usages, metricsData.metrics);
  }, [usagesData, metricsData]);

  if (isError) {
    return (
      <Alert severity="error" sx={{ mt: 1 }}>
        {USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR}
      </Alert>
    );
  }

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {[1, 2, 3].map((i) => (
          <Card
            key={i}
            variant="outlined"
            sx={{ borderRadius: 0, p: 1.5, borderColor: "divider" }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <Box
                sx={{
                  flex: 1,
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <Box>
                  <Skeleton variant="text" width={180} height={24} />
                  <Skeleton variant="text" width={120} height={20} />
                </Box>
                <Box sx={{ display: { xs: "none", sm: "flex" }, gap: 3 }}>
                  <Skeleton
                    variant="rectangular"
                    width={100}
                    height={32}
                    sx={{ borderRadius: 1 }}
                  />
                  <Skeleton
                    variant="rectangular"
                    width={100}
                    height={32}
                    sx={{ borderRadius: 1 }}
                  />
                </Box>
              </Box>
              <Skeleton variant="circular" width={24} height={24} />
            </Box>
          </Card>
        ))}
      </Box>
    );
  }

  if (products.length === 0) {
    return (
      <Box
        sx={{
          flex: 1,
          minHeight: "60vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <EmptyState description={USAGE_METRICS_NO_PRODUCTS_IN_DEPLOYMENT} />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1.5,
        width: "100%",
        maxWidth: "100%",
        minWidth: 0,
        overflowX: "hidden",
      }}
    >
      {products.map((product) => (
        <ProductAccordionRow
          key={product.id}
          product={product}
          deploymentId={deploymentId}
          expanded={expandedProductIds.has(product.id)}
          onToggle={() => onToggleProduct(product.id)}
        />
      ))}
    </Box>
  );
}
