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
import EmptyState from "@components/common/empty-state/EmptyState";
import { LineChart } from "@wso2/oxygen-ui-charts-react";
import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import type { InstanceUsageEntry, InstanceMetricEntry } from "@/types/usage";
import type { UsageEnvironmentProduct, UsageInstanceChartBlock, UsageProductInstanceRow, UsageTrendRow } from "@/types/usage";
import { USAGE_LINE_CHART_MARGIN } from "@constants/usageMetricsConstants";
import { UsageChartSurface } from "@components/project-details/usage-metrics/UsageChartSurface";
import usePostDeploymentInstancesUsagesSearch from "@api/usePostDeploymentInstancesUsagesSearch";
import usePostDeploymentInstancesMetricsSearch from "@api/usePostDeploymentInstancesMetricsSearch";

function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}


function buildTrendFromUsages(
  usages: InstanceUsageEntry[],
  countKey: string,
): UsageTrendRow[] {
  const periodMap = new Map<string, number>();
  for (const entry of usages) {
    for (const ps of entry.periodSummaries) {
      const prev = periodMap.get(ps.period) ?? 0;
      periodMap.set(ps.period, prev + (ps.counts[countKey] ?? 0));
    }
  }
  return Array.from(periodMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([period, value]) => ({ name: period.slice(5), value }));
}

function buildCoreTrendFromMetrics(metrics: InstanceMetricEntry[]): UsageTrendRow[] {
  const periodMap = new Map<string, { total: number; count: number }>();
  for (const entry of metrics) {
    for (const dp of entry.dataPoints) {
      const cores =
        dp.coreCount != null
          ? dp.coreCount
          : dp.deploymentMetadata?.numberOfCores != null
            ? Number(dp.deploymentMetadata.numberOfCores)
            : 0;
      const existing = periodMap.get(dp.date) ?? { total: 0, count: 0 };
      existing.total += cores;
      existing.count += 1;
      periodMap.set(dp.date, existing);
    }
  }
  return Array.from(periodMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, { total, count }]) => ({
      name: date.slice(5),
      current: total,
      average: count > 0 ? Math.round(total / count) : 0,
    }));
}

function computeHeadlineDelta(trend: UsageTrendRow[]): { headline: string; delta: string; deltaPositive: boolean } {
  if (trend.length === 0) return { headline: "—", delta: "—", deltaPositive: true };
  const last = trend[trend.length - 1].value ?? 0;
  if (trend.length === 1) return { headline: formatCount(last), delta: "—", deltaPositive: true };
  const prev = trend[trend.length - 2].value ?? 0;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  const sign = pct >= 0 ? "+" : "";
  return {
    headline: formatCount(last),
    delta: `${sign}${pct.toFixed(1)}%`,
    deltaPositive: pct >= 0,
  };
}

/**
 * Derives product rows from deployment-scoped usages + metrics.
 * Groups entries by deployedProduct.id (falling back to product.id then instanceId).
 */
function deriveProducts(
  usages: InstanceUsageEntry[],
  metrics: InstanceMetricEntry[],
): UsageEnvironmentProduct[] {
  const productMap = new Map<
    string,
    {
      label: string;
      usages: InstanceUsageEntry[];
      metrics: InstanceMetricEntry[];
    }
  >();

  for (const u of usages) {
    const pid = u.deployedProduct?.id ?? u.product?.id ?? u.instanceId;
    const label = u.deployedProduct?.label ?? u.product?.label ?? u.instanceKey;
    const existing = productMap.get(pid) ?? { label, usages: [], metrics: [] };
    existing.usages.push(u);
    productMap.set(pid, existing);
  }
  for (const m of metrics) {
    const pid = m.deployedProduct?.id ?? m.product?.id ?? m.instanceId;
    const label = m.deployedProduct?.label ?? m.product?.label ?? m.instanceKey;
    const existing = productMap.get(pid) ?? { label, usages: [], metrics: [] };
    existing.metrics.push(m);
    productMap.set(pid, existing);
  }

  return Array.from(productMap.entries()).map(([pid, { label, usages: pUsages, metrics: pMetrics }]) => {
    const txTrend = buildTrendFromUsages(pUsages, "TRANSACTION_COUNT");
    const coreTrend = buildCoreTrendFromMetrics(pMetrics);
    const totalTx = pUsages.reduce(
      (sum, u) => sum + u.periodSummaries.reduce((s, ps) => s + (ps.counts["TRANSACTION_COUNT"] ?? 0), 0),
      0,
    );
    const totalCores = pMetrics.reduce(
      (sum, m) => sum + m.dataPoints.reduce((s, dp) => {
        const c = dp.coreCount != null ? dp.coreCount
          : dp.deploymentMetadata?.numberOfCores != null ? Number(dp.deploymentMetadata.numberOfCores) : 0;
        return s + c;
      }, 0),
      0,
    );
    const totalApis = pUsages.reduce(
      (sum, u) => sum + u.periodSummaries.reduce((s, ps) => s + (ps.counts["API_COUNT"] ?? 0), 0),
      0,
    );

    const instanceRows: UsageProductInstanceRow[] = pUsages.map((u) => {
      const instUsageTrend = buildTrendFromUsages([u], "TRANSACTION_COUNT");
      const instTxHD = computeHeadlineDelta(instUsageTrend);
      const instMetric = pMetrics.find((m) => m.instanceId === u.instanceId);
      const instCoreTrend = instMetric ? buildCoreTrendFromMetrics([instMetric]) : [];
      const lastCore = instMetric?.dataPoints.at(-1)?.coreCount
        ?? (instMetric?.dataPoints.at(-1)?.deploymentMetadata?.numberOfCores != null
          ? Number(instMetric.dataPoints.at(-1)!.deploymentMetadata!.numberOfCores)
          : 0)
        ?? 0;
      const coreHD = computeHeadlineDelta(instCoreTrend.map((r) => ({ name: r.name, value: r.current })));

      const javaVer = instMetric?.dataPoints.at(-1)?.jdkVersion
        ?? instMetric?.dataPoints.at(-1)?.deploymentMetadata?.jdkVersion
        ?? "—";
      const u2Level = instMetric?.dataPoints.at(-1)?.deploymentMetadata?.updateLevel ?? "—";
      const instanceStroke = colors.blue?.[500] ?? "#3B82F6";

      const txBlock: UsageInstanceChartBlock = {
        title: "Transactions",
        caption: "Periodic transaction count",
        headlineValue: instTxHD.headline,
        deltaLabel: instTxHD.delta,
        deltaPositive: instTxHD.deltaPositive,
        stroke: instanceStroke,
        data: instUsageTrend,
      };
      const coresBlock: UsageInstanceChartBlock = {
        title: "Core Count",
        caption: "Cores over time",
        headlineValue: String(lastCore),
        deltaLabel: coreHD.delta,
        deltaPositive: coreHD.deltaPositive,
        stroke: colors.orange?.[500] ?? "#F97316",
        data: instCoreTrend.map((r) => ({ name: r.name, value: r.current })),
      };

      return {
        id: u.instanceId,
        hostName: u.instanceId,
        javaVersion: javaVer,
        u2Level,
        transactionsLabel: instTxHD.headline,
        coreCount: lastCore,
        charts: { transactions: txBlock, cores: coresBlock },
      };
    });

    return {
      id: pid,
      name: label,
      version: "",
      runningInstances: pUsages.length,
      transactionsLabel: formatCount(totalTx),
      thirdMetricLabel: "API Count",
      thirdMetricValue: String(totalApis),
      coreMetrics: [
        { label: "Total Cores", value: String(totalCores) },
        { label: "Instances", value: String(pUsages.length) },
      ],
      transactionTrend: txTrend,
      coreUsageTrend: coreTrend,
      instances: instanceRows,
    };
  });
}

const ACCENT_PALETTE = [
  colors.orange?.[600] ?? "#EA580C",
  colors.blue?.[600] ?? "#2563EB",
  colors.green?.[600] ?? "#16A34A",
  colors.purple?.[600] ?? "#9333EA",
  colors.cyan?.[600] ?? "#0891B2",
  colors.red?.[600] ?? "#DC2626",
];

function accentIndexForTypeId(typeId: string): number {
  let h = 0;
  for (let i = 0; i < typeId.length; i++) h += typeId.charCodeAt(i);
  return h % ACCENT_PALETTE.length;
}

/** Returns accent colours for a deployment typeId (stable, palette-based). */
function panelAccent(typeId: string): {
  main: string;
  stroke: string;
  borderDefault: string;
  borderHover: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
} {
  const main = ACCENT_PALETTE[accentIndexForTypeId(typeId)];
  return {
    main,
    stroke: main,
    borderDefault: alpha(main, 0.15),
    borderHover: alpha(main, 0.35),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
  };
}

function instanceAccordionKey(productId: string, instanceId: string): string {
  return `${productId}::${instanceId}`;
}

// ─── Per-product expanded view (Renders pre-computed inherited instances directly) ──────

interface ProductExpandedViewProps {
  product: UsageEnvironmentProduct;
  expandedInstanceKeys: Set<string>;
  onToggleInstance: (key: string) => void;
}

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
        No instance data available.
      </Typography>
    );
  }

  return (
    <>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, lg: 6 }}>
          <Card sx={{ p: 2, borderRadius: 0 }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 600, mb: 2 }}>
              Transaction Trends
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
                    name: "Transactions",
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
              Core Usage Over Time
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
                    name: "Cores",
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
          Instances ({instances.length})
        </Typography>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {instances.map((inst) => (
            <InstanceAccordionRow
              key={inst.id}
              instance={inst}
              expanded={expandedInstanceKeys.has(
                instanceAccordionKey(product.id, inst.id),
              )}
              onToggle={() =>
                onToggleInstance(instanceAccordionKey(product.id, inst.id))
              }
            />
          ))}
        </Box>
      </Box>
    </>
  );
}

// ─── Instance minicard ────────────────────────────────────────────────────────

function InstanceMiniTrendCard({ block }: { block: UsageInstanceChartBlock }): JSX.Element {
  const deltaColor = block.deltaPositive
    ? (colors.green?.[600] ?? "#16A34A")
    : (colors.red?.[600] ?? "#DC2626");

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
          <Typography variant="caption" sx={{ fontWeight: 600, color: deltaColor }}>
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
}: {
  instance: UsageProductInstanceRow;
  expanded: boolean;
  onToggle: () => void;
}): JSX.Element {
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
        expandIcon={<ChevronDown size={20} color={colors.grey?.[400] ?? "#9CA3AF"} />}
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
              label="Java Version"
              value={instance.javaVersion}
            />
            <MetricPill
              icon={<Package size={16} color={colors.grey?.[400]} />}
              label="U2 Level"
              value={instance.u2Level}
            />
            <MetricPill
              icon={<Activity size={16} color={colors.grey?.[400]} />}
              label="Total Transactions"
              value={instance.transactionsLabel}
            />
            <MetricPill
              icon={<Cpu size={16} color={colors.grey?.[400]} />}
              label="Core Count"
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
}: {
  product: UsageEnvironmentProduct;
  deploymentId: string;
  expanded: boolean;
  onToggle: () => void;
  expandedInstanceKeys: Set<string>;
  onToggleInstance: (key: string) => void;
}): JSX.Element {
  const a = panelAccent(deploymentId);

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
                Total Transactions
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
                {product.coreMetrics[0]?.label || "Total Cores"}
              </Typography>
              <Typography variant="h6" sx={{ fontWeight: 600 }}>
                {product.coreMetrics[0]?.value}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "center" }}>
              <Typography variant="caption" color="text.secondary">
                {product.coreMetrics[1]?.label || "Instances"}
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

function MetricPill({
  icon,
  label,
  value,
}: {
  icon: JSX.Element;
  label: string;
  value: string;
}): JSX.Element {
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

interface UsageEnvironmentProductsPanelProps {
  deploymentId: string;
  projectId: string | undefined;
  dateRange: { startDate: string; endDate: string };
  expandedProductIds: Set<string>;
  onToggleProduct: (productId: string) => void;
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
    () => ({ filters: { startDate: dateRange.startDate, endDate: dateRange.endDate } }),
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
    return deriveProducts(usagesData.usages, metricsData.metrics);
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
        Failed to load environment product data. Please try again.
      </Alert>
    );
  }

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {[1, 2, 3].map((i) => (
          <Card key={i} variant="outlined" sx={{ borderRadius: 0, p: 1.5, borderColor: "divider" }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              <Box sx={{ flex: 1, display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <Box>
                  <Skeleton variant="text" width={180} height={24} />
                  <Skeleton variant="text" width={120} height={20} />
                </Box>
                <Box sx={{ display: { xs: "none", sm: "flex" }, gap: 3 }}>
                  <Skeleton variant="rectangular" width={100} height={32} sx={{ borderRadius: 1 }} />
                  <Skeleton variant="rectangular" width={100} height={32} sx={{ borderRadius: 1 }} />
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
        <EmptyState description="No products found for this deployment." />
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
