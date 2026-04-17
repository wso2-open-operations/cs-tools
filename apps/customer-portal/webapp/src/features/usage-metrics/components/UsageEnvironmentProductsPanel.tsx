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
  Activity,
  ChevronDown,
  Code2,
  Cpu,
  Package,
  Server,
} from "@wso2/oxygen-ui-icons-react";
import EmptyState from "@components/empty-state/EmptyState";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import {
  USAGE_LINE_CHART_MARGIN,
  USAGE_METRICS_CHART_LINE_CORES_SHORT,
  USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR,
  USAGE_METRICS_INSTANCE_CHART_TX_TITLE,
  USAGE_METRICS_INSTANCE_CORE_COUNT,
  USAGE_METRICS_INSTANCE_JAVA,
  USAGE_METRICS_INSTANCE_TOTAL_TX,
  USAGE_METRICS_INSTANCE_U2,
  USAGE_METRICS_NO_INSTANCE_DATA,
  USAGE_METRICS_NO_PRODUCTS_IN_DEPLOYMENT,
  USAGE_METRICS_PRODUCT_CORE_METRIC_INSTANCES,
  USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES,
  USAGE_METRICS_PRODUCT_PANEL_TOTAL_TRANSACTIONS,
  USAGE_METRICS_PRODUCT_TREND_CORE_SECTION,
  USAGE_METRICS_PRODUCT_TREND_TX_SECTION,
  USAGE_METRICS_PRODUCT_INSTANCES_SECTION,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import { UsageChartSurface } from "@features/usage-metrics/components/UsageChartSurface";
import usePostDeploymentInstancesUsagesSearch from "@features/project-details/api/usePostDeploymentInstancesUsagesSearch";
import usePostDeploymentInstancesMetricsSearch from "@features/project-details/api/usePostDeploymentInstancesMetricsSearch";
import type {
  InstanceAccordionRowProps,
  InstanceMiniTrendCardProps,
  MetricPillProps,
  ProductAccordionRowProps,
  ProductExpandedViewProps,
  UsageEnvironmentProductsPanelProps,
} from "@features/usage-metrics/types/usageMetrics";
import { getUsageEnvironmentPanelAccent } from "@features/usage-metrics/utils/usageMetricsAccent";
import {
  buildUsageProductInstanceAccordionKey,
  deriveUsageEnvironmentProducts,
} from "@features/usage-metrics/utils/usageMetricsEnvironmentProducts";
import { resolveSignedDeltaColor } from "@features/usage-metrics/utils/usageMetricSignedDelta";

// ─── Per-product expanded view (Renders pre-computed inherited instances directly) ──────

/**
 * Renders the expanded product instance grid utilizing the pre-processed grouped payload.
 */
function ProductExpandedView({
  product,
  expandedInstanceKeys,
  onToggleInstance,
}: ProductExpandedViewProps): JSX.Element {
  const instances = product.instances;

  if (!instances || instances.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {USAGE_METRICS_NO_INSTANCE_DATA}
      </Typography>
    );
  }

  return (
    <>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, lg: 6 }}>
          <Card sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
              {USAGE_METRICS_PRODUCT_TREND_TX_SECTION}
            </Typography>
            <UsageChartSurface minHeight={200}>
              <LineChart
                data={product.transactionTrend}
                xAxisDataKey="name"
                height={200}
                width="100%"
                margin={USAGE_LINE_CHART_MARGIN}
                accessibilityLayer={false}
                legend={{ show: false }}
                grid={{ show: true, strokeDasharray: "3 3" }}
                lines={[
                  {
                    dataKey: "value",
                    name: USAGE_METRICS_INSTANCE_CHART_TX_TITLE,
                    stroke: colors.blue?.[500] ?? "#3B82F6",
                    strokeWidth: 2.5,
                    dot: false,
                  },
                ]}
              />
            </UsageChartSurface>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, lg: 6 }}>
          <Card sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
              {USAGE_METRICS_PRODUCT_TREND_CORE_SECTION}
            </Typography>
            <UsageChartSurface minHeight={200}>
              <LineChart
                data={product.coreUsageTrend}
                xAxisDataKey="name"
                height={200}
                width="100%"
                margin={USAGE_LINE_CHART_MARGIN}
                accessibilityLayer={false}
                legend={{ show: false }}
                grid={{ show: true, strokeDasharray: "3 3" }}
                lines={[
                  {
                    dataKey: "current",
                    name: USAGE_METRICS_CHART_LINE_CORES_SHORT,
                    stroke: colors.orange?.[500] ?? "#F97316",
                    strokeWidth: 2.5,
                    dot: false,
                  },
                ]}
              />
            </UsageChartSurface>
          </Card>
        </Grid>
      </Grid>

      <Box sx={{ mt: 2 }}>
        <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 1.5 }}>
          {USAGE_METRICS_PRODUCT_INSTANCES_SECTION} ({instances.length})
        </Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {instances.map((inst) => (
            <InstanceAccordionRow
              key={inst.id}
              instance={inst}
              expanded={expandedInstanceKeys.has(
                buildUsageProductInstanceAccordionKey(product.id, inst.id),
              )}
              onToggle={() =>
                onToggleInstance(
                  buildUsageProductInstanceAccordionKey(product.id, inst.id),
                )
              }
            />
          ))}
        </Box>
      </Box>
    </>
  );
}

// ─── Instance minicard ────────────────────────────────────────────────────────

function InstanceMiniTrendCard({
  block,
}: InstanceMiniTrendCardProps): JSX.Element {
  const deltaColor = resolveSignedDeltaColor(block.deltaPositive);

  return (
    <Card
      sx={{
        p: 2,
        border: 1,
        borderColor: "divider",
        borderRadius: 0,
        display: "flex",
        flexDirection: "column",
        gap: 0,
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
          mb: 1.5,
          gap: 1,
        }}
      >
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            {block.title}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {block.caption}
          </Typography>
        </Box>
        <Box sx={{ textAlign: "right", flexShrink: 0 }}>
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            {block.headlineValue}
          </Typography>
          <Typography
            variant="caption"
            sx={{ fontWeight: 600, color: deltaColor }}
          >
            {block.deltaLabel}
          </Typography>
        </Box>
      </Box>
      <UsageChartSurface minHeight={150}>
        <LineChart
          data={block.data}
          xAxisDataKey="name"
          height={150}
          width="100%"
          margin={USAGE_LINE_CHART_MARGIN}
          accessibilityLayer={false}
          legend={{ show: false }}
          grid={{ show: true, strokeDasharray: "3 3" }}
          lines={[
            {
              dataKey: "value",
              name: block.title,
              stroke: block.stroke,
              strokeWidth: 2,
              dot: false,
            },
          ]}
        />
      </UsageChartSurface>
    </Card>
  );
}

function InstanceAccordionRow({
  instance,
  expanded,
  onToggle,
}: InstanceAccordionRowProps): JSX.Element {
  const borderMuted = alpha(colors.grey?.[500] ?? "#6B7280", 0.2);
  const wellBg = alpha(colors.grey?.[500] ?? "#6B7280", 0.04);

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
        borderColor: borderMuted,
        borderRadius: 0,
        bgcolor: "background.paper",
        "&.Mui-expanded": { margin: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={
          <ChevronDown size={20} color={colors.grey?.[400] ?? "#9CA3AF"} />
        }
        sx={{
          px: 2,
          py: 2,
          minHeight: 56,
          textAlign: "left",
          borderRadius: 0,
          "&:hover": { bgcolor: wellBg },
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", lg: "row" },
            alignItems: { xs: "stretch", lg: "center" },
            justifyContent: "space-between",
            gap: 2,
            width: "100%",
            pr: 1,
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
              gap: { xs: 2, lg: 4 },
              alignItems: "center",
              justifyContent: { xs: "flex-start", lg: "flex-end" },
              flex: 1,
            }}
          >
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
            <MetricPill
              icon={<Activity size={16} color={colors.grey?.[400]} />}
              label={USAGE_METRICS_INSTANCE_TOTAL_TX}
              value={instance.transactionsLabel}
            />
            <MetricPill
              icon={<Cpu size={16} color={colors.grey?.[400]} />}
              label={USAGE_METRICS_INSTANCE_CORE_COUNT}
              value={String(instance.coreCount)}
            />
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails
        sx={{
          px: 2,
          pb: 2,
          pt: 0,
          borderTop: 1,
          borderColor: "divider",
          bgcolor: alpha(colors.grey?.[500] ?? "#6B7280", 0.06),
          borderRadius: 0,
        }}
      >
        <Grid container spacing={2} sx={{ mt: 1 }}>
          <Grid size={{ xs: 12, lg: 6 }}>
            <InstanceMiniTrendCard block={instance.charts.transactions} />
          </Grid>
          <Grid size={{ xs: 12, lg: 6 }}>
            <InstanceMiniTrendCard block={instance.charts.cores} />
          </Grid>
        </Grid>
      </AccordionDetails>
    </Accordion>
  );
}

// ─── Product accordion row (summary from deployment-scoped data) ───────────────

function ProductAccordionRow({
  product,
  deploymentId,
  expanded,
  onToggle,
  expandedInstanceKeys,
  onToggleInstance,
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
        borderColor: a.borderDefault,
        borderRadius: 0,
        transition: "border-color 0.2s ease",
        bgcolor: "background.paper",
        "&:hover": { borderColor: a.borderHover },
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
          "&:hover": {
            bgcolor: a.headerHoverBg,
          },
        }}
      >
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", lg: "row" },
            alignItems: { xs: "stretch", lg: "flex-start" },
            gap: 2,
            width: "100%",
            pr: 1,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 2,
              minWidth: { lg: 250 },
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
              <Package size={24} color={a.iconColor} />
            </Box>
            <Box sx={{ textAlign: "left" }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                {product.name}
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{
              flex: 1,
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "repeat(4, 1fr)" },
              gap: 2,
              alignItems: "start",
            }}
          >
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {USAGE_METRICS_PRODUCT_PANEL_TOTAL_TRANSACTIONS}
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.transactionsLabel}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {product.thirdMetricLabel}
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.thirdMetricValue}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {product.coreMetrics[0]?.label ||
                  USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES}
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.coreMetrics[0]?.value}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {product.coreMetrics[1]?.label ||
                  USAGE_METRICS_PRODUCT_CORE_METRIC_INSTANCES}
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.coreMetrics[1]?.value}
              </Typography>
            </Box>
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
        <ProductExpandedView
          product={product}
          expandedInstanceKeys={expandedInstanceKeys}
          onToggleInstance={onToggleInstance}
        />
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

/**
 * Deployment tab — fetches deployment-scoped instances/usages/metrics to build
 * the product summary list, then lazily loads per-product detail from
 * POST /deployments/products/{id}/... when a product is expanded.
 *
 * @param deploymentId - The deployment whose products to display.
 * @param projectId - Active project ID (kept for possible future use).
 * @param dateRange - Selected date range for API queries.
 * @param expandedProductIds - Expanded product ids.
 * @param onToggleProduct - Toggle handler.
 * @returns {JSX.Element} Deployment tab content.
 */
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

  // Deployment-scoped calls for the product summary list
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

  const [expandedInstanceKeys, setExpandedInstanceKeys] = useState<Set<string>>(
    () => new Set(),
  );

  const toggleInstance = useCallback((key: string) => {
    setExpandedInstanceKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

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
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
      {products.map((product) => (
        <ProductAccordionRow
          key={product.id}
          product={product}
          deploymentId={deploymentId}
          expanded={expandedProductIds.has(product.id)}
          onToggle={() => onToggleProduct(product.id)}
          expandedInstanceKeys={expandedInstanceKeys}
          onToggleInstance={toggleInstance}
        />
      ))}
    </Box>
  );
}
