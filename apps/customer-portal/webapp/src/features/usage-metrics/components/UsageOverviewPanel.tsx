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
import usePostProjectInstancesSearch from "@features/project-details/api/usePostProjectInstancesSearch";
import usePostProjectInstancesUsagesSearch from "@features/project-details/api/usePostProjectInstancesUsagesSearch";
import usePostProjectInstancesMetricsSearch from "@features/project-details/api/usePostProjectInstancesMetricsSearch";
import usePostDeploymentInstancesSearch from "@features/project-details/api/usePostDeploymentInstancesSearch";
import usePostDeploymentInstancesUsagesSearch from "@features/project-details/api/usePostDeploymentInstancesUsagesSearch";
import useGetProjectUsageStats from "@features/project-details/api/useGetProjectUsageStats";
import type { UsageAggregatedMetricDefinition } from "@features/project-details/types/usage";
import type {
  InstanceItem,
  InstanceUsageEntry,
} from "@features/project-details/types/usage";
import UsageMetricTrendCard from "@features/usage-metrics/components/UsageMetricTrendCard";
import {
  USAGE_METRICS_ENVIRONMENT_PRODUCTS_ERROR,
  USAGE_METRICS_ENVIRONMENT_ROW_TRANSACTIONS,
  USAGE_METRICS_EXPANDED_CARD_CORES,
  USAGE_METRICS_EXPANDED_CARD_INSTANCES,
  USAGE_METRICS_EXPANDED_CARD_TOTAL_TX,
  USAGE_METRICS_NO_PRODUCTS_IN_ENVIRONMENT,
  USAGE_METRICS_OVERVIEW_ERROR,
  USAGE_METRICS_SECTION_AGGREGATED,
  USAGE_METRICS_SECTION_ENVIRONMENT_BREAKDOWN,
  USAGE_METRICS_STAT_ENVIRONMENTS,
  USAGE_METRICS_STAT_INSTANCES,
  USAGE_METRICS_STAT_PRODUCTS,
  USAGE_METRICS_UNKNOWN_LABEL,
  USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import type {
  DeploymentExpandedViewProps,
  EnvironmentBreakdownAccordionProps,
  EnvironmentBreakdownRow,
  ExpandedProductCardProps,
  UsageOverviewPanelProps,
} from "@features/usage-metrics/types/usageMetrics";
import { deriveAggregatedMetrics } from "@features/usage-metrics/utils/usageMetricsAggregated";
import { getUsageOverviewAccentForTypeId } from "@features/usage-metrics/utils/usageMetricsAccent";
import { formatUsageMetricCount, sumUsageEntryTransactions } from "@features/project-details/utils/usageMetrics";

function EnvironmentIcon({ typeId }: { typeId: string }): JSX.Element {
  const color = getUsageOverviewAccentForTypeId(typeId).iconColor;
  return <Server size={20} color={color} />;
}

// ─── Per-deployment expanded product cards (sourced from deployment-scoped APIs) ───

function ExpandedProductCard({
  product,
  accent: a,
}: ExpandedProductCardProps): JSX.Element {
  return (
    <Card
      variant="outlined"
      sx={{
        p: 2,
        height: "100%",
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
              {USAGE_METRICS_EXPANDED_CARD_INSTANCES}
            </Typography>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {product.instanceCount}
            </Typography>
          </Box>
        </Grid>
        <Grid size={6}>
          <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
            <Typography variant="caption" color="text.secondary">
              {USAGE_METRICS_EXPANDED_CARD_CORES}
            </Typography>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {product.coreCount}
            </Typography>
          </Box>
        </Grid>
      </Grid>
      <Box sx={{ bgcolor: a.statTileBg, p: 1.5 }}>
        <Typography variant="caption" color="text.secondary">
          {USAGE_METRICS_EXPANDED_CARD_TOTAL_TX}
        </Typography>
        <Typography variant="h6" sx={{ fontWeight: 600 }}>
          {formatUsageMetricCount(product.totalTx)}
        </Typography>
      </Box>
    </Card>
  );
}

/** Expanded view for a single deployment accordion — fetches from deployment-scoped APIs. */
function DeploymentExpandedView({
  deploymentId,
  typeId,
  dateRange,
}: DeploymentExpandedViewProps): JSX.Element {
  const a = getUsageOverviewAccentForTypeId(typeId);

  const metricsPayload = useMemo(
    () => ({
      filters: { startDate: dateRange.startDate, endDate: dateRange.endDate },
    }),
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
    const productMap = new Map<
      string,
      {
        label: string;
        instanceCount: number;
        coreCount: number;
        totalTx: number;
      }
    >();

    for (const inst of instances) {
      const pid = inst.deployedProduct?.id;
      const label = inst.deployedProduct?.label ?? USAGE_METRICS_UNKNOWN_LABEL;
      if (!pid) continue;
      const existing = productMap.get(pid) ?? {
        label,
        instanceCount: 0,
        coreCount: 0,
        totalTx: 0,
      };
      existing.instanceCount += 1;
      existing.coreCount += inst.metadata?.coreCount ?? 0;
      productMap.set(pid, existing);
    }

    // Accumulate transactions from deployment-scoped usages per deployedProduct
    for (const usage of usages) {
      const pid = usage.deployedProduct?.id;
      if (!pid) continue;
      // Ensure entry exists if it came only from usages (edge case)
      const label = usage.deployedProduct?.label ?? USAGE_METRICS_UNKNOWN_LABEL;
      const existing = productMap.get(pid) ?? {
        label,
        instanceCount: 0,
        coreCount: 0,
        totalTx: 0,
      };
      existing.totalTx += sumUsageEntryTransactions(usage);
      productMap.set(pid, existing);
    }

    return Array.from(productMap.entries()).map(([id, p]) => ({ id, ...p }));
  }, [instancesData, usagesData, deploymentId]);

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
        <Skeleton variant="rounded" height={140} />
        <Skeleton variant="rounded" height={140} />
      </Box>
    );
  }

  if (products.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {USAGE_METRICS_NO_PRODUCTS_IN_ENVIRONMENT}
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

function EnvironmentBreakdownAccordion({
  row,
  expanded,
  onToggle,
  dateRange,
}: EnvironmentBreakdownAccordionProps): JSX.Element {
  const a = getUsageOverviewAccentForTypeId(row.kind);

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
              <Typography
                variant="subtitle1"
                sx={{ fontWeight: 600, color: a.title }}
              >
                {row.title}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {row.productCount} product{row.productCount !== 1 ? "s" : ""}
                <Box component="span" sx={{ mx: 0.75, opacity: 0.4 }}>
                  |
                </Box>
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
                {USAGE_METRICS_PRODUCT_CORE_METRIC_TOTAL_CORES}
              </Typography>
              <Typography variant="body2" sx={{ fontWeight: 600 }}>
                {row.totalCores}
              </Typography>
            </Box>
            <Box sx={{ textAlign: "right" }}>
              <Typography variant="caption" color="text.secondary">
                {USAGE_METRICS_ENVIRONMENT_ROW_TRANSACTIONS}
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
    () => ({
      filters: { startDate: dateRange.startDate, endDate: dateRange.endDate },
    }),
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

  const isLoading =
    deploymentsLoading ||
    instancesLoading ||
    usagesLoading ||
    metricsLoading ||
    statsLoading;
  const isError =
    deploymentsError ||
    instancesError ||
    usagesError ||
    metricsError ||
    statsError;

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
      const productIds = new Set(
        depInstances.map((i) => i.deployedProduct?.id).filter(Boolean),
      );
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
          const depUsages = (usagesData?.usages ?? []).filter(
            (u) => u.deployment?.id === dep.id,
          );
          const total = depUsages.reduce(
            (sum, u) => sum + sumUsageEntryTransactions(u),
            0,
          );
          return formatUsageMetricCount(total);
        })(),
      };
    });
  }, [deploymentsData, instancesData, usagesData]);

  // ── Aggregated metric trend cards ─────────────────────────────────────────
  const aggregatedMetrics = useMemo((): UsageAggregatedMetricDefinition[] => {
    return deriveAggregatedMetrics(
      usagesData?.usages ?? [],
      metricsData?.metrics ?? [],
      isSmallScreen,
    );
  }, [usagesData, metricsData, isSmallScreen]);

  if (isError) {
    return (
      <Alert severity="error" sx={{ mt: 1 }}>
        {USAGE_METRICS_OVERVIEW_ERROR}
      </Alert>
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label={USAGE_METRICS_STAT_ENVIRONMENTS}
            value={
              isStatCardsLoading
                ? ((
                    <Skeleton variant="rounded" width={60} height={24} />
                  ) as any)
                : overviewStats.environments
            }
            icon={<Layers />}
            iconColor="info"
          />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label={USAGE_METRICS_STAT_PRODUCTS}
            value={
              isStatCardsLoading
                ? ((
                    <Skeleton variant="rounded" width={60} height={24} />
                  ) as any)
                : overviewStats.products
            }
            icon={<Package />}
            iconColor="primary"
          />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label={USAGE_METRICS_STAT_INSTANCES}
            value={
              isStatCardsLoading
                ? ((
                    <Skeleton variant="rounded" width={60} height={24} />
                  ) as any)
                : overviewStats.instances
            }
            icon={<ServerIcon />}
            iconColor="success"
          />
        </Grid>
      </Grid>

      {timeRangeSelector && <Box sx={{ mt: 1 }}>{timeRangeSelector}</Box>}

      <Box>
        <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
          {USAGE_METRICS_SECTION_ENVIRONMENT_BREAKDOWN}
        </Typography>
        {isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            {[1, 2, 3].map((i) => (
              <Card
                key={i}
                variant="outlined"
                sx={{ borderRadius: 0, p: 1.5, borderColor: "divider" }}
              >
                <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                  <Skeleton variant="rectangular" width={32} height={32} />
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
          {USAGE_METRICS_SECTION_AGGREGATED}
        </Typography>
        {isLoading ? (
          <Grid container spacing={2}>
            {[0, 1, 2, 3, 4].map((i) => (
              <Grid key={i} size={{ xs: 12, lg: 6 }}>
                <Card variant="outlined" sx={{ p: 2 }}>
                  <Box
                    sx={{
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "flex-start",
                      mb: 2,
                    }}
                  >
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
