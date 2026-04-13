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
  StatCard,
  Typography,
  alpha,
  colors,
  useTheme,
  useMediaQuery,
} from "@wso2/oxygen-ui";
import {
  ChevronDown,
  Layers,
  Package,
  Server,
  Server as ServerIcon,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useMemo } from "react";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";
import usePostProjectInstancesSearch from "@api/usePostProjectInstancesSearch";
import usePostProjectInstancesUsagesSearch from "@api/usePostProjectInstancesUsagesSearch";
import usePostProjectInstancesMetricsSearch from "@api/usePostProjectInstancesMetricsSearch";
import usePostDeploymentInstancesSearch from "@api/usePostDeploymentInstancesSearch";
import usePostDeploymentInstancesUsagesSearch from "@api/usePostDeploymentInstancesUsagesSearch";
import useGetProjectUsageStats from "@api/useGetProjectUsageStats";
import type { UsageAggregatedMetricDefinition, UsageTrendRow } from "@/types/usage";
import type { InstanceItem, InstanceUsageEntry, InstanceMetricEntry } from "@/types/usage";
import { colors as oxygenColors } from "@wso2/oxygen-ui";
import UsageMetricTrendCard from "@components/project-details/usage-metrics/UsageMetricTrendCard";


const ORANGE_STROKE = oxygenColors.orange?.[600] ?? "#EA580C";
const VIOLET_STROKE = oxygenColors.purple?.[500] ?? "#8B5CF6";
const GREEN_STROKE = oxygenColors.green?.[500] ?? "#22C55E";
const CYAN_STROKE = oxygenColors.cyan?.[500] ?? "#06B6D4";
const AMBER_STROKE = oxygenColors.amber?.[500] ?? "#F59E0B";

/** Format ISO date (YYYY-MM-DD) to date format for the chart, responsive to screen size. */
function formatDateForChart(isoDate: string, isSmallScreen: boolean = false): string {
  try {
    const date = new Date(`${isoDate}T00:00:00Z`);
    const monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    const month = monthNames[date.getUTCMonth()];
    const day = date.getUTCDate();
    const year = date.getUTCFullYear();
    
    // Small screens: show only month-day (MM-DD format)
    if (isSmallScreen) {
      return `${month} ${day}`;
    }
    // Large screens: show month-day with year on new line
    return `${month} ${day}\n${year}`;
  } catch {
    return isoDate.slice(5); // Fallback to MM-DD
  }
}

/** Format a raw transaction count to a human-readable label. */
function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

/** Sum TRANSACTION_COUNT across all periodSummaries for a usage entry. */
function sumTransactions(entry: InstanceUsageEntry): number {
  return entry.periodSummaries.reduce(
    (total, ps) => total + (ps.counts["TRANSACTION_COUNT"] ?? 0),
    0,
  );
}

/** Build chart trend rows from usages grouped by period, summing a count key. */
function buildTrendFromUsages(
  usages: InstanceUsageEntry[],
  countKey: string,
  isSmallScreen: boolean = false,
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
    .map(([period, value]) => ({ name: formatDateForChart(period, isSmallScreen), value }));
}

/** Compute headline value (last point) and delta % vs previous point. */
function computeHeadlineDelta(trend: UsageTrendRow[]): {
  headline: string;
  delta: string;
} {
  if (trend.length === 0) return { headline: "—", delta: "—" };
  const last = trend[trend.length - 1].value ?? 0;
  if (trend.length === 1) return { headline: formatCount(last), delta: "—" };
  const prev = trend[trend.length - 2].value ?? 0;
  const pct = prev === 0 ? 0 : ((last - prev) / prev) * 100;
  const sign = pct >= 0 ? "+" : "";
  return { headline: formatCount(last), delta: `${sign}${pct.toFixed(1)}%` };
}

/** Build a daily trend of summed cores across all instance metrics. */
function buildDailyCoreTrend(metrics: InstanceMetricEntry[], isSmallScreen: boolean = false): UsageTrendRow[] {
  const dayMap = new Map<string, number>();

  for (const m of metrics) {
    for (const dp of m.dataPoints) {
      if (!dp.date) continue;
      const c = dp.coreCount != null
        ? dp.coreCount
        : (dp.deploymentMetadata?.numberOfCores != null ? Number(dp.deploymentMetadata.numberOfCores) : 0);
      const prev = dayMap.get(dp.date) ?? 0;
      dayMap.set(dp.date, prev + c);
    }
  }

  return Array.from(dayMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, total]) => ({
      name: formatDateForChart(date, isSmallScreen),
      value: total,
    }));
}

/** Derive the 5 aggregated metric cards from project usages and metrics. */
function deriveAggregatedMetrics(usages: InstanceUsageEntry[], metrics: InstanceMetricEntry[], isSmallScreen: boolean = false): UsageAggregatedMetricDefinition[] {
  const txTrend = buildTrendFromUsages(usages, "TRANSACTION_COUNT", isSmallScreen);
  const apiTrend = buildTrendFromUsages(usages, "API_COUNT", isSmallScreen);
  const userTrend = buildTrendFromUsages(usages, "TOTAL_USERS", isSmallScreen);
  const b2bTrend = buildTrendFromUsages(usages, "TOTAL_B2B_ORGS", isSmallScreen);
  const coreTrend = buildDailyCoreTrend(metrics, isSmallScreen);

  const txHD = computeHeadlineDelta(txTrend);
  const apiHD = computeHeadlineDelta(apiTrend);
  const userHD = computeHeadlineDelta(userTrend);
  const b2bHD = computeHeadlineDelta(b2bTrend);
  const coreHD = computeHeadlineDelta(coreTrend);

  return [
    {
      id: "total-transactions",
      title: "Total Transactions",
      caption: "Across all environments and products",
      headlineValue: txHD.headline,
      deltaLabel: txHD.delta,
      stroke: ORANGE_STROKE,
      data: txTrend,
    },
    {
      id: "total-users",
      title: "Total User Count",
      caption: "Active users across all environments",
      headlineValue: userHD.headline,
      deltaLabel: userHD.delta,
      stroke: GREEN_STROKE,
      data: userTrend,
    },
    {
      id: "api-count",
      title: "Total API Count",
      caption: "Unique APIs accessed across all products",
      headlineValue: apiHD.headline,
      deltaLabel: apiHD.delta,
      stroke: VIOLET_STROKE,
      data: apiTrend,
    },
    {
      id: "total-cores",
      title: "Total Cores",
      caption: "Daily core count allocation across all instances",
      headlineValue: coreHD.headline,
      deltaLabel: coreHD.delta,
      stroke: AMBER_STROKE,
      data: coreTrend,
    },
    {
      id: "total-organization-count",
      title: "Total Organization Count",
      caption: "External organizations using your APIs",
      headlineValue: b2bHD.headline,
      deltaLabel: b2bHD.delta,
      stroke: CYAN_STROKE,
      data: b2bTrend,
    },
  ];
}

/** Cycle through a palette of accent colours based on the typeId string. */
const ACCENT_PALETTE = [
  colors.orange?.[600] ?? "#EA580C",
  colors.blue?.[600] ?? "#2563EB",
  colors.green?.[600] ?? "#16A34A",
  colors.purple?.[600] ?? "#9333EA",
  colors.cyan?.[600] ?? "#0891B2",
  colors.red?.[600] ?? "#DC2626",
];
const ACCENT_TITLE_PALETTE = [
  colors.orange?.[800] ?? "#9A3412",
  colors.blue?.[800] ?? "#1E40AF",
  colors.green?.[800] ?? "#166534",
  colors.purple?.[800] ?? "#6B21A8",
  colors.cyan?.[800] ?? "#155E75",
  colors.red?.[800] ?? "#991B1B",
];

function accentIndexForTypeId(typeId: string): number {
  let h = 0;
  for (let i = 0; i < typeId.length; i++) h += typeId.charCodeAt(i);
  return h % ACCENT_PALETTE.length;
}

function accentForEnvironment(typeId: string): {
  title: string;
  main: string;
  border: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
  statTileBg: string;
} {
  const idx = accentIndexForTypeId(typeId);
  const main = ACCENT_PALETTE[idx];
  const title = ACCENT_TITLE_PALETTE[idx];
  return {
    main,
    title,
    border: alpha(main, 0.2),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
    statTileBg: alpha(main, 0.08),
  };
}

function EnvironmentIcon({ typeId }: { typeId: string }): JSX.Element {
  const color = accentForEnvironment(typeId).iconColor;
  return <Server size={20} color={color} />;
}

// ─── Per-deployment expanded product cards (sourced from deployment-scoped APIs) ───

interface ExpandedProductCardProps {
  product: {
    id: string;
    label: string;
    instanceCount: number;
    coreCount: number;
    totalTx: number;
  };
  accent: ReturnType<typeof accentForEnvironment>;
}

function ExpandedProductCard({ product, accent: a }: ExpandedProductCardProps): JSX.Element {
  return (
    <Card
      sx={{
        p: 2,
        height: "100%",
        border: "1px solid",
        borderColor: alpha(a.main, 0.15),
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1.5 }}>
        <Box
          sx={{
            p: 1,
            bgcolor: a.iconWellBg,
            display: "flex",
            alignItems: "center",
          }}
        >
          <Package size={20} color={a.iconColor} />
        </Box>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="subtitle2" noWrap sx={{ fontWeight: 600 }}>
            {product.label}
          </Typography>
        </Box>
      </Box>
      <Grid container spacing={1} sx={{ mb: 1 }}>
        <Grid size={6}>
          <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
            <Typography variant="caption" color="text.secondary">
              Instances
            </Typography>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {product.instanceCount}
            </Typography>
          </Box>
        </Grid>
        <Grid size={6}>
          <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
            <Typography variant="caption" color="text.secondary">
              Cores
            </Typography>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {product.coreCount}
            </Typography>
          </Box>
        </Grid>
      </Grid>
      <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
        <Typography variant="caption" color="text.secondary">
          Total Transactions
        </Typography>
        <Typography variant="h6" sx={{ fontWeight: 600 }}>
          {formatCount(product.totalTx)}
        </Typography>
      </Box>
    </Card>
  );
}

/** Expanded view for a single deployment accordion — fetches from deployment-scoped APIs. */
interface DeploymentExpandedViewProps {
  deploymentId: string;
  typeId: string;
  dateRange: { startDate: string; endDate: string };
}

function DeploymentExpandedView({
  deploymentId,
  typeId,
  dateRange,
}: DeploymentExpandedViewProps): JSX.Element {
  const a = accentForEnvironment(typeId);

  const metricsPayload = useMemo(
    () => ({ filters: { startDate: dateRange.startDate, endDate: dateRange.endDate } }),
    [dateRange],
  );

  // Fetch instances for this deployment — gives us products + instance/core counts
  const {
    data: instancesData,
    isLoading: instancesLoading,
    isError: instancesError,
  } = usePostDeploymentInstancesSearch(deploymentId);

  // Fetch usages for this deployment — gives us transactions per deployedProduct
  const {
    data: usagesData,
    isLoading: usagesLoading,
    isError: usagesError,
  } = usePostDeploymentInstancesUsagesSearch(deploymentId, metricsPayload);

  const isLoading = instancesLoading || usagesLoading;
  const isError = instancesError || usagesError;

  // Derive unique products from instances.
  const products = useMemo(() => {
    const instances: InstanceItem[] = instancesData?.instances ?? [];
    const usages: InstanceUsageEntry[] = usagesData?.usages ?? [];

    // Build product map keyed by deployedProduct.id
    const productMap = new Map<string, {
      label: string;
      instanceCount: number;
      coreCount: number;
      totalTx: number;
    }>();

    for (const inst of instances) {
      const pid = inst.deployedProduct?.id;
      const label = inst.deployedProduct?.label ?? "Unknown";
      if (!pid) continue;
      const existing = productMap.get(pid) ?? { label, instanceCount: 0, coreCount: 0, totalTx: 0 };
      existing.instanceCount += 1;
      existing.coreCount += inst.metadata?.coreCount ?? 0;
      productMap.set(pid, existing);
    }

    // Accumulate transactions from deployment-scoped usages per deployedProduct
    for (const usage of usages) {
      const pid = usage.deployedProduct?.id;
      if (!pid) continue;
      // Ensure entry exists if it came only from usages (edge case)
      const label = usage.deployedProduct?.label ?? "Unknown";
      const existing = productMap.get(pid) ?? { label, instanceCount: 0, coreCount: 0, totalTx: 0 };
      existing.totalTx += sumTransactions(usage);
      productMap.set(pid, existing);
    }

    return Array.from(productMap.entries()).map(([id, p]) => ({ id, ...p }));
  }, [instancesData, usagesData, deploymentId]);

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
        <Skeleton variant="rounded" height={140} />
        <Skeleton variant="rounded" height={140} />
      </Box>
    );
  }

  if (products.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No products found for this environment.
      </Typography>
    );
  }

  return (
    <Grid container spacing={2}>
      {products.map((product) => (
        <Grid key={product.id} size={{ xs: 12, md: 6, lg: 4 }}>
          <ExpandedProductCard product={product} accent={a} />
        </Grid>
      ))}
    </Grid>
  );
}

// ─── Environment breakdown accordion ───────────────────────────────────────────

interface EnvironmentBreakdownRow {
  id: string;
  deploymentId: string;
  kind: string; // typeId for accent
  title: string;
  productCount: number;
  instanceCount: number;
  totalCores: number;
  transactionsLabel: string;
}

interface EnvironmentBreakdownAccordionProps {
  row: EnvironmentBreakdownRow;
  expanded: boolean;
  onToggle: () => void;
  dateRange: { startDate: string; endDate: string };
}

function EnvironmentBreakdownAccordion({
  row,
  expanded,
  onToggle,
  dateRange,
}: EnvironmentBreakdownAccordionProps): JSX.Element {
  const a = accentForEnvironment(row.kind);

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
        borderColor: a.border,
        borderRadius: 0,
        bgcolor: "background.paper",
        "&.Mui-expanded": { margin: 0 },
      }}
    >
      <AccordionSummary
        expandIcon={<ChevronDown size={20} color={a.iconColor} />}
        sx={{
          px: 2,
          py: 1.5,
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
            alignItems: "center",
            justifyContent: "space-between",
            width: "100%",
            pr: 1,
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            <Box
              sx={{
                p: 1,
                bgcolor: a.iconWellBg,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <EnvironmentIcon typeId={row.kind} />
            </Box>
            <Box sx={{ textAlign: "left" }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, color: a.title }}>
                {row.title}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {row.productCount} product{row.productCount !== 1 ? "s" : ""}
                <Box component="span" sx={{ mx: 0.75, opacity: 0.4 }}>|</Box>
                {row.instanceCount} instance{row.instanceCount !== 1 ? "s" : ""}
              </Typography>
            </Box>
          </Box>
          <Box
            sx={{
              display: { xs: "none", sm: "flex" },
              alignItems: "center",
              gap: 3,
            }}
          >
            <Box sx={{ textAlign: "right" }}>
              <Typography variant="caption" color="text.secondary">
                Total Cores
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {row.totalCores}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "right" }}>
              <Typography variant="caption" color="text.secondary">
                Transactions
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {row.transactionsLabel}
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
        <DeploymentExpandedView
          deploymentId={row.deploymentId}
          typeId={row.kind}
          dateRange={dateRange}
        />
      </AccordionDetails>
    </Accordion>
  );
}

// ─── Main panel ─────────────────────────────────────────────────────────────────

interface UsageOverviewPanelProps {
  projectId: string | undefined;
  dateRange: { startDate: string; endDate: string };
  expandedEnvironmentIds: Set<string>;
  onToggleEnvironment: (id: string) => void;
  timeRangeSelector?: React.ReactNode;
}

/**
 * Usage overview: summary tiles, environment breakdown, aggregated line charts.
 *
 * @param projectId - Active project ID from route params.
 * @param dateRange - Selected date range for API queries.
 * @param expandedEnvironmentIds - Which environment rows show product grids.
 * @param onToggleEnvironment - Toggle handler for an environment row id.
 * @returns {JSX.Element} Overview tab content.
 */
export default function UsageOverviewPanel({
  projectId,
  dateRange,
  expandedEnvironmentIds,
  onToggleEnvironment,
  timeRangeSelector,
}: UsageOverviewPanelProps): JSX.Element {
  const theme = useTheme();
  const isSmallScreen = useMediaQuery(theme.breakpoints.down("sm"));

  const metricsPayload = useMemo(
    () => ({ filters: { startDate: dateRange.startDate, endDate: dateRange.endDate } }),
    [dateRange],
  );

  // Deployments list — for environment names and typeIds
  const {
    data: deploymentsData,
    isLoading: deploymentsLoading,
    isError: deploymentsError,
  } = usePostProjectDeploymentsSearchAll(projectId ?? "");

  // Project-wide instances — for product count, instance count, core count per deployment
  const {
    data: instancesData,
    isLoading: instancesLoading,
    isError: instancesError,
  } = usePostProjectInstancesSearch(projectId);

  // Project-wide usages — for aggregated metric trend cards
  const {
    data: usagesData,
    isLoading: usagesLoading,
    isError: usagesError,
  } = usePostProjectInstancesUsagesSearch(projectId, metricsPayload);

  // Project-wide metrics — for aggregated core metrics
  const {
    data: metricsData,
    isLoading: metricsLoading,
    isError: metricsError,
  } = usePostProjectInstancesMetricsSearch(projectId, metricsPayload);

  // Project-wide stats — for the 3 top stat cards
  const {
    data: statsData,
    isLoading: statsLoading,
    isError: statsError,
  } = useGetProjectUsageStats(projectId);

  const isLoading = deploymentsLoading || instancesLoading || usagesLoading || metricsLoading || statsLoading;
  const isError = deploymentsError || instancesError || usagesError || metricsError || statsError;
  
  const isStatCardsLoading = statsLoading;

  // ── Overview summary stats ─────────────────────────────────────────────────
  const overviewStats = useMemo(() => {
    return {
      environments: statsData?.deploymentCount ?? 0,
      products: statsData?.deployedProductCount ?? 0,
      instances: statsData?.instanceCount ?? 0,
    };
  }, [statsData]);

  // ── Environment breakdown rows ─────────────────────────────────────────────
  const environmentBreakdown = useMemo((): EnvironmentBreakdownRow[] => {
    if (!deploymentsData) return [];
    const instances = instancesData?.instances ?? [];

    return deploymentsData.map((dep) => {
      const depInstances = instances.filter((i) => i.deployment?.id === dep.id);
      const productIds = new Set(depInstances.map((i) => i.deployedProduct?.id).filter(Boolean));
      const productCount = productIds.size;
      const instanceCount = depInstances.length;
      const totalCores = depInstances.reduce(
        (sum, i) => sum + (i.metadata?.coreCount ?? 0),
        0,
      );

      return {
        id: `env-${dep.id}`,
        deploymentId: dep.id,
        kind: dep.type.id,
        title: dep.name,
        productCount,
        instanceCount,
        totalCores,
        transactionsLabel: (() => {
          const depUsages = (usagesData?.usages ?? []).filter((u) => u.deployment?.id === dep.id);
          const total = depUsages.reduce((sum, u) => sum + sumTransactions(u), 0);
          return formatCount(total);
        })(),
      };
    });
  }, [deploymentsData, instancesData, usagesData]);

  // ── Aggregated metric trend cards ─────────────────────────────────────────
  const aggregatedMetrics = useMemo((): UsageAggregatedMetricDefinition[] => {
    return deriveAggregatedMetrics(usagesData?.usages ?? [], metricsData?.metrics ?? [], isSmallScreen);
  }, [usagesData, metricsData, isSmallScreen]);

  if (isError) {
    return (
      <Alert severity="error" sx={{ mt: 1 }}>
        Failed to load usage overview data. Please try again.
      </Alert>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label="Environments"
            value={isStatCardsLoading ? (<Skeleton variant="rounded" width={60} height={24} /> as any) : overviewStats.environments}
            icon={<Layers />}
            iconColor="info"
          />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label="Products"
            value={isStatCardsLoading ? (<Skeleton variant="rounded" width={60} height={24} /> as any) : overviewStats.products}
            icon={<Package />}
            iconColor="primary"
          />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label="Instances"
            value={isStatCardsLoading ? (<Skeleton variant="rounded" width={60} height={24} /> as any) : overviewStats.instances}
            icon={<ServerIcon />}
            iconColor="success"
          />
        </Grid>
      </Grid>

      {timeRangeSelector && <Box sx={{ mt: 1 }}>{timeRangeSelector}</Box>}

      <Box>
        <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
          Environment Breakdown
        </Typography>
        {isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            {[1, 2, 3].map((i) => (
              <Card key={i} variant="outlined" sx={{ borderRadius: 0, p: 1.5, borderColor: "divider" }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                  <Skeleton variant="rectangular" width={32} height={32} />
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
                </Box>
              </Card>
            ))}
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            {environmentBreakdown.map((row) => (
              <EnvironmentBreakdownAccordion
                key={row.id}
                row={row}
                expanded={expandedEnvironmentIds.has(row.id)}
                onToggle={() => onToggleEnvironment(row.id)}
                dateRange={dateRange}
              />
            ))}
          </Box>
        )}
      </Box>

      <Box>
        <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
          Aggregated Metrics
        </Typography>
        {isLoading ? (
          <Grid container spacing={2}>
            {[0, 1, 2, 3, 4].map((i) => (
              <Grid key={i} size={{ xs: 12, lg: 6 }}>
                <Card variant="outlined" sx={{ p: 2 }}>
                  <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", mb: 2 }}>
                    <Box>
                      <Skeleton variant="text" width={140} height={24} />
                      <Skeleton variant="text" width={220} height={20} />
                    </Box>
                    <Box sx={{ textAlign: "right" }}>
                      <Skeleton variant="text" width={80} height={32} />
                      <Skeleton variant="text" width={50} height={20} />
                    </Box>
                  </Box>
                  <Skeleton variant="rounded" height={100} />
                </Card>
              </Grid>
            ))}
          </Grid>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            <Grid container spacing={2}>
              {aggregatedMetrics.slice(0, 2).map((metric) => (
                <Grid key={metric.id} size={{ xs: 12, lg: 6 }}>
                  <UsageMetricTrendCard metric={metric} />
                </Grid>
              ))}
            </Grid>
            <Grid container spacing={2}>
              {aggregatedMetrics.slice(2, 4).map((metric) => (
                <Grid key={metric.id} size={{ xs: 12, lg: 6 }}>
                  <UsageMetricTrendCard metric={metric} />
                </Grid>
              ))}
            </Grid>
            {aggregatedMetrics[4] != null && (
              <Grid container spacing={2}>
                <Grid size={{ xs: 12, lg: 6 }}>
                  <UsageMetricTrendCard metric={aggregatedMetrics[4]} />
                </Grid>
              </Grid>
            )}
          </Box>
        )}
      </Box>
    </Box>
  );
}
