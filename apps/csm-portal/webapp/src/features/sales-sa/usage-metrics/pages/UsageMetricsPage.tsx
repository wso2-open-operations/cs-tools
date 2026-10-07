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

// Ported from apps/support-portal-lite/webapp's UsageMetricsPage (itself
// via one-wso2, from SupportPortalLite's UsageMetricsDashboard.tsx). UI/JSX
// unchanged; data layer rewritten onto useUsageMetricsApi.ts (see that
// file's header comment for why it reproduces usePostApi/useParallelPostApi's
// exact call shape rather than going through React Query directly — this
// page's debounced/paginated/multi-parallel-fetch logic doesn't fit a
// declarative queryFn). No SplShell wrapper — audience gating (Sales/SA) is
// RouteGuard's job; canViewUsageMetrics (from usePermissions) below is
// the second, finer-grained check this page has always done on its own.
import { useEffect, useMemo, useRef, useState, type JSX, type SyntheticEvent, type UIEvent } from "react";
import {
  Box,
  Typography,
  FormControl,
  Select,
  MenuItem,
  TextField,
  Button,
  Paper,
  Tabs,
  Tab,
  ListSubheader,
  InputAdornment,
  Alert,
  CircularProgress,
  LinearProgress,
} from "@wso2/oxygen-ui";
import type { SelectChangeEvent } from "@mui/material";
import { LockIcon, SearchIcon, CalendarIcon } from "@wso2/oxygen-ui-icons-react";
import { usePermissions } from "@features/sales-sa/api/permissionsContext";
import { usePostApi, useParallelPostApi } from "@features/sales-sa/usage-metrics/api/useUsageMetricsApi";
import { type StatSummary, fmtNumber, computeStats, extractVersion } from "@features/sales-sa/usage-metrics/utils/formatters";
import { ProductBreakdownRow, type ProductBreakdownRowProps } from "@features/sales-sa/usage-metrics/components/ProductBreakdownRow";
import { METRIC_CHART_CONFIG } from "@features/sales-sa/usage-metrics/utils/usageMetricsProductClassifier";
import type {
  SnInstancesResponse,
  DeploymentsSearchResponse,
  DeploymentItem,
  DeployedProductsSearchResponse,
  SnDeployedProductMetricsResponse,
  SnDeployedProductUsageCountsResponse,
} from "@features/sales-sa/usage-metrics/api/usageMetricsTypes";

interface ProjectListItem {
  id: string;
  name: string;
  key: string;
}
interface ProjectsSearchResponse {
  projects: ProjectListItem[];
  totalRecords: number;
  offset: number;
  limit: number;
}

type TimeRange = "1M" | "3M" | "6M" | "12M" | "Custom";

const KNOWN_METRIC_KEY_ORDER = Object.keys(METRIC_CHART_CONFIG);

const RANGE_DAYS: Record<Exclude<TimeRange, "Custom">, number> = {
  "1M": 30,
  "3M": 90,
  "6M": 180,
  "12M": 365,
};

// Must stay in sync with MAX_METRICS_DATE_RANGE_DAYS in the SupportPortalLite backend's
// modules/operations/operations.bal.
const MAX_RANGE_DAYS = 366;

const DEBOUNCE_DELAY = 500;

// Paths below are relative to CSM_PORTAL_BACKEND_BASE_URL, the same backend
// every other page in this app calls — useBackendApi() resolves them
// against it. (The source app's usage-metrics calls went through
// splBackendUrl, a full separate origin; this app has no such split.)
const PROJECTS_PAGE_SIZE = 20;
const PROJECTS_URL = "/usage-metrics/projects/search";

function todayStr(): string {
  return new Date().toISOString().split("T")[0];
}
function daysAgoStr(n: number): string {
  return new Date(Date.now() - n * 86_400_000).toISOString().split("T")[0];
}
const MIN_ALLOWED_DATE = daysAgoStr(MAX_RANGE_DAYS);

export default function UsageMetricsPage(): JSX.Element {
  const { canViewUsageMetrics } = usePermissions();

  const [selectedProjectId, setSelectedProjectId] = useState("");
  const [selectedProjectLabel, setSelectedProjectLabel] = useState("");
  const [projectSearch, setProjectSearch] = useState("");
  const [projectList, setProjectList] = useState<ProjectListItem[]>([]);
  const [projectsHasMore, setProjectsHasMore] = useState(false);
  const [projectsSearching, setProjectsSearching] = useState(false);
  const projectsOffsetRef = useRef(0);
  const projectListRef = useRef<ProjectListItem[]>([]);
  const dropdownOpenRef = useRef(false);
  const loadingMoreRef = useRef(false);
  const activeSearchQueryRef = useRef("");
  const projectsCacheRef = useRef<Map<string, { list: ProjectListItem[]; totalRecords: number }>>(new Map());
  const searchInputRef = useRef<HTMLInputElement>(null);
  const [activeEnv, setActiveEnv] = useState("");
  const [expandedProductIds, setExpandedProductIds] = useState<Set<string>>(new Set());
  const [timeRange, setTimeRange] = useState<TimeRange>("1M");
  const [customFrom, setCustomFrom] = useState(daysAgoStr(30));
  const [customTo, setCustomTo] = useState(todayStr());
  const [pendingFrom, setPendingFrom] = useState(daysAgoStr(30));
  const [pendingTo, setPendingTo] = useState(todayStr());

  const dateFrom = timeRange === "Custom" ? customFrom : daysAgoStr(RANGE_DAYS[timeRange as Exclude<TimeRange, "Custom">]);
  const dateTo = timeRange === "Custom" ? customTo : todayStr();

  const activeDepId = activeEnv || undefined;

  const { data: projectsPage, loading: projectsLoading, postApiData: fetchProjectsPage } = usePostApi<ProjectsSearchResponse>();
  const fetchProjectsPageRef = useRef(fetchProjectsPage);
  fetchProjectsPageRef.current = fetchProjectsPage;

  const { data: deploymentsData, loading: deploymentsLoading, postApiData: fetchDeployments } = usePostApi<DeploymentsSearchResponse>();

  const { data: deployedProductsData, loading: deployedProductsLoading, postApiData: fetchDeployedProducts } =
    usePostApi<DeployedProductsSearchResponse>();

  const deployments = useMemo(() => (deploymentsData?.deployments ?? []).filter((d) => d.deployedProductCount > 0), [deploymentsData]);

  const {
    dataMap: prodInstances,
    loading: prodInstancesLoading,
    postAll: fetchProdInstances,
    clearAll: clearProdInstances,
  } = useParallelPostApi<SnInstancesResponse>();

  const { dataMap: prodMetricsStats, loading: prodMetricsStatsLoading, postAll: fetchProdMetricsStats } =
    useParallelPostApi<SnDeployedProductMetricsResponse>();

  const { dataMap: prodUsagesStats, loading: prodUsagesStatsLoading, postAll: fetchProdUsagesStats } =
    useParallelPostApi<SnDeployedProductUsageCountsResponse>();

  useEffect(() => {
    if (!projectsPage) return;
    loadingMoreRef.current = false;
    const incoming = projectsPage.projects ?? [];
    const base = projectsPage.offset === 0 ? [] : projectListRef.current;
    const newList = [...base, ...incoming];

    if (projectsPage.offset === 0) setProjectsSearching(false);
    setProjectList(newList);
    projectListRef.current = newList;
    projectsOffsetRef.current = projectsPage.offset + incoming.length;
    setProjectsHasMore(projectsOffsetRef.current < projectsPage.totalRecords);

    projectsCacheRef.current.set(activeSearchQueryRef.current, {
      list: newList,
      totalRecords: projectsPage.totalRecords,
    });
  }, [projectsPage]);

  useEffect(() => {
    if (!dropdownOpenRef.current) return;
    const cacheKey = projectSearch.length >= 2 ? projectSearch : "";
    if (projectsCacheRef.current.has(cacheKey)) return;
    const timer = setTimeout(() => {
      projectsOffsetRef.current = 0;
      loadingMoreRef.current = false;
      activeSearchQueryRef.current = cacheKey;
      fetchProjectsPageRef.current(
        { filters: cacheKey ? { searchQuery: cacheKey } : {}, pagination: { offset: 0, limit: PROJECTS_PAGE_SIZE } },
        PROJECTS_URL,
      );
    }, DEBOUNCE_DELAY);
    return () => clearTimeout(timer);
  }, [projectSearch]);

  useEffect(() => {
    if (!selectedProjectId) return;
    fetchDeployments({ filters: { projectIds: [selectedProjectId] }, pagination: { offset: 0, limit: 50 } }, "/usage-metrics/deployments/search");
    setActiveEnv("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedProjectId]);

  useEffect(() => {
    const deps = deployments;
    if (deps.length === 0) return;
    const isProduction = (d: DeploymentItem) => (d.type?.label ?? "").toLowerCase() === "production" || (d.name ?? "").toLowerCase() === "production";
    const target = deps.find(isProduction) ?? deps[0];
    setActiveEnv((prev) => (prev && deps.some((d) => d.id === prev) ? prev : target.id));
  }, [deployments]);

  useEffect(() => {
    if (!selectedProjectId || !deploymentsData) return;
    const deps = deployments;
    if (deps.length === 0) return;

    const currentEnv = deps.some((d) => d.id === activeEnv) ? activeEnv : deps[0].id;
    if (currentEnv !== activeEnv) setActiveEnv(currentEnv);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedProjectId, deployments, dateFrom, dateTo]);

  useEffect(() => {
    clearProdInstances();
    setExpandedProductIds(new Set());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeDepId]);

  useEffect(() => {
    if (expandedProductIds.size === 0) return;
    const ids = Array.from(expandedProductIds);
    fetchProdInstances(
      ids.map((id) => ({
        id,
        payload: {
          filters: { deployedProductIds: [id], startDate: dateFrom, endDate: dateTo },
          pagination: { offset: 0, limit: 50 },
        },
      })),
      "/usage-metrics/instances/search",
      false,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dateFrom, dateTo]);

  useEffect(() => {
    if (!activeDepId) return;
    fetchDeployedProducts({ filters: { deploymentIds: [activeDepId] }, pagination: { offset: 0, limit: 50 } }, "/usage-metrics/deployed-products/search");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeDepId]);

  useEffect(() => {
    if (!activeDepId) return;
    const allProducts = deployedProductsData?.deployedProducts ?? [];
    const productsForActiveDep = allProducts.filter((p) => p.deployment?.id === activeDepId);
    if (productsForActiveDep.length === 0) return;
    const productIds = productsForActiveDep.map((p) => p.id);

    const payload = { deploymentId: activeDepId, startDate: dateFrom, endDate: dateTo };
    fetchProdMetricsStats(
      productIds.map((id) => ({ id, payload })),
      (id: string) => `/usage-metrics/deployed-products/${encodeURIComponent(id)}/metrics/search`,
      false,
    );
    fetchProdUsagesStats(
      productIds.map((id) => ({ id, payload })),
      (id: string) => `/usage-metrics/deployed-products/${encodeURIComponent(id)}/metrics/usage-counts/search`,
      false,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeDepId, deployedProductsData, dateFrom, dateTo]);

  const handleProjectDropdownOpen = () => {
    dropdownOpenRef.current = true;
    setTimeout(() => searchInputRef.current?.focus(), 0);

    const cacheKey = projectSearch.length >= 2 ? projectSearch : "";
    const cached = projectsCacheRef.current.get(cacheKey);
    if (cached) {
      setProjectsSearching(false);
      setProjectList(cached.list);
      projectListRef.current = cached.list;
      projectsOffsetRef.current = cached.list.length;
      setProjectsHasMore(cached.list.length < cached.totalRecords);
      return;
    }

    projectsOffsetRef.current = 0;
    loadingMoreRef.current = false;
    setProjectsHasMore(false);
    setProjectsSearching(true);
    activeSearchQueryRef.current = cacheKey;
    fetchProjectsPageRef.current(
      { filters: cacheKey ? { searchQuery: cacheKey } : {}, pagination: { offset: 0, limit: PROJECTS_PAGE_SIZE } },
      PROJECTS_URL,
    );
  };

  const loadMoreProjects = () => {
    if (loadingMoreRef.current || projectsLoading || !projectsHasMore) return;
    loadingMoreRef.current = true;
    const cacheKey = projectSearch.length >= 2 ? projectSearch : "";
    activeSearchQueryRef.current = cacheKey;
    fetchProjectsPageRef.current(
      { filters: cacheKey ? { searchQuery: cacheKey } : {}, pagination: { offset: projectsOffsetRef.current, limit: PROJECTS_PAGE_SIZE } },
      PROJECTS_URL,
    );
  };

  const handleProjectMenuScroll = (event: UIEvent<HTMLElement>) => {
    const el = event.currentTarget;
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) {
      loadMoreProjects();
    }
  };

  const handleTabChange = (_: SyntheticEvent, val: string) => setActiveEnv(val);

  const handleProductToggle = (deployedProductId: string) => {
    setExpandedProductIds((prev) => {
      const next = new Set(prev);
      if (next.has(deployedProductId)) {
        next.delete(deployedProductId);
        return next;
      }
      next.add(deployedProductId);
      if (!prodInstances.has(deployedProductId)) {
        fetchProdInstances(
          [
            {
              id: deployedProductId,
              payload: {
                filters: { deployedProductIds: [deployedProductId], startDate: dateFrom, endDate: dateTo },
                pagination: { offset: 0, limit: 50 },
              },
            },
          ],
          "/usage-metrics/instances/search",
          true,
        );
      }
      return next;
    });
  };

  const envTabs = useMemo(() => deployments.map((d) => ({ value: d.id, label: d.name })), [deployments]);

  const activeDeployment = deployments.find((d) => d.id === activeEnv);

  const productBreakdown = useMemo((): Omit<ProductBreakdownRowProps, "expanded" | "onToggle">[] => {
    if (!activeEnv) return [];
    const deployedProducts = (deployedProductsData?.deployedProducts ?? []).filter((p) => p.deployment?.id === activeEnv);
    if (deployedProducts.length === 0) return [];

    return deployedProducts.map((dp) => {
      const dpId = dp.id;
      const productName = dp.product?.name ?? "";
      const displayName = [productName, dp.version?.name].filter(Boolean).join(" ") || "Unknown";
      const version = dp.version?.name ?? extractVersion(undefined, productName);

      const metricsResp = prodMetricsStats.get(dpId);
      const sortedChart = [...(metricsResp?.chartData ?? [])].sort((a, b) => a.date.localeCompare(b.date));

      const instanceStats: StatSummary = computeStats(sortedChart.map((p) => p.instanceCount));

      const coreStats: StatSummary = metricsResp?.summary
        ? {
            curr: sortedChart.length > 0 ? sortedChart[sortedChart.length - 1].totalCores ?? 0 : 0,
            avg: Math.round(metricsResp.summary.avgCores ?? 0),
            min: metricsResp.summary.minCores ?? 0,
            max: metricsResp.summary.maxCores ?? 0,
          }
        : { curr: 0, avg: 0, min: 0, max: 0 };

      const usagesResp = prodUsagesStats.get(dpId);
      const countTypes = usagesResp?.summary.countTypes ?? {};
      const availableKeys = Object.keys(countTypes);
      const metricKeys = [
        ...KNOWN_METRIC_KEY_ORDER.filter((k) => availableKeys.includes(k)),
        ...availableKeys.filter((k) => !KNOWN_METRIC_KEY_ORDER.includes(k)),
      ];
      const usageChartData = usagesResp?.chartData ?? [];
      const summaryStats = metricKeys.map((key) => {
        const stat = countTypes[key];
        let value = 0;
        if (stat) {
          value =
            stat.aggregation === "sum"
              ? usageChartData.reduce((total, point) => total + (point.counts[key]?.value ?? 0), 0)
              : (stat[stat.aggregation as "min" | "max" | "avg"] ?? stat.avg);
        }
        return {
          label: METRIC_CHART_CONFIG[key]?.title ?? key,
          value: fmtNumber(value),
        };
      });

      return {
        id: dpId,
        name: displayName,
        version,
        summaryStats,
        metricKeys,
        instanceStats,
        coreStats,
      };
    });
  }, [activeEnv, deployedProductsData, prodMetricsStats, prodUsagesStats]);

  const mainLoading = deploymentsLoading;

  const prodBreakdownLoading = (() => {
    if (!activeDepId) return deploymentsLoading;
    return deployedProductsLoading || !deployedProductsData;
  })();

  if (!canViewUsageMetrics) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", py: 12 }}>
        <LockIcon size={52} color="var(--mui-palette-text-disabled, currentColor)" style={{ marginBottom: 16, opacity: 0.6 }} />
        <Typography variant="h6" fontWeight={600} gutterBottom>
          Access Restricted
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Usage & Metrics data is restricted to authorized personnel only. This page is accessible to members of the
          Customer Success, Sales, and Sales Engineering teams.
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h5" sx={{ mb: 0.5 }}>
        Usage & Metrics
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Monitor transaction volumes and core usage across deployments
      </Typography>

      <Box sx={{ mb: 3 }}>
        <Typography variant="body2" fontWeight={600} sx={{ mb: 0.75, color: "text.secondary" }}>
          Project
        </Typography>
        <FormControl sx={{ width: 380 }}>
          <Select
            value={selectedProjectId}
            onChange={(e: SelectChangeEvent) => {
              const id = e.target.value;
              const proj = projectList.find((p) => p.id === id);
              setSelectedProjectId(id);
              setSelectedProjectLabel(proj ? `${proj.key} – ${proj.name}` : id);
            }}
            displayEmpty
            onOpen={handleProjectDropdownOpen}
            onClose={() => {
              dropdownOpenRef.current = false;
              setProjectSearch("");
            }}
            renderValue={(val) => {
              if (!val) return <Typography color="text.secondary">Select a project to view metrics...</Typography>;
              return selectedProjectLabel || val;
            }}
            sx={{ bgcolor: "background.paper", fontSize: 15, fontWeight: 500 }}
            MenuProps={{ autoFocus: false, PaperProps: { onScroll: handleProjectMenuScroll, sx: { maxHeight: 360 } } }}
          >
            <ListSubheader sx={{ pt: 1, pb: 0.5, px: 1, bgcolor: "background.paper" }}>
              <TextField
                size="small"
                fullWidth
                placeholder="Search projects..."
                value={projectSearch}
                onChange={(e) => {
                  const val = e.target.value;
                  setProjectSearch(val);
                  const cacheKey = val.length >= 2 ? val : "";
                  const cached = projectsCacheRef.current.get(cacheKey);
                  if (cached) {
                    setProjectsSearching(false);
                    setProjectList(cached.list);
                    projectListRef.current = cached.list;
                    projectsOffsetRef.current = cached.list.length;
                    setProjectsHasMore(cached.list.length < cached.totalRecords);
                  } else {
                    setProjectsSearching(true);
                  }
                }}
                inputRef={searchInputRef}
                onKeyDown={(e) => e.stopPropagation()}
                slotProps={{
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <SearchIcon size={16} />
                      </InputAdornment>
                    ),
                  },
                }}
                sx={{ "& .MuiOutlinedInput-root": { borderRadius: "8px" } }}
              />
            </ListSubheader>

            {projectsSearching && (
              <MenuItem disabled sx={{ justifyContent: "center", gap: 1, py: 2 }}>
                <CircularProgress size={16} />
                <Typography fontSize={13} color="text.secondary">
                  {projectSearch.length >= 2 ? "Searching..." : "Loading..."}
                </Typography>
              </MenuItem>
            )}

            {!projectsSearching &&
              projectList.map((p) => (
                <MenuItem key={p.id} value={p.id}>
                  <Box>
                    <Typography fontSize={13} fontWeight={600}>
                      {p.key}
                    </Typography>
                    <Typography fontSize={12} color="text.secondary">
                      {p.name}
                    </Typography>
                  </Box>
                </MenuItem>
              ))}

            {!projectsSearching && !projectsLoading && projectList.length === 0 && (
              <MenuItem disabled>
                <Typography fontSize={13} color="text.secondary">
                  {projectSearch.length >= 2 ? "No projects found" : "No projects available"}
                </Typography>
              </MenuItem>
            )}

            {!projectsSearching && projectsLoading && (
              <MenuItem disabled sx={{ justifyContent: "center", gap: 1 }}>
                <CircularProgress size={14} />
                <Typography fontSize={13} color="text.secondary">
                  Loading more...
                </Typography>
              </MenuItem>
            )}
          </Select>
        </FormControl>
      </Box>

      {!selectedProjectId ? (
        <Box sx={{ textAlign: "center", py: 10 }}>
          <Typography color="text.secondary">Select a project to view usage and metrics.</Typography>
        </Box>
      ) : (
        <>
          <Paper variant="outlined" sx={{ mb: 3, borderRadius: 0, overflow: "hidden" }}>
            <Tabs
              value={activeEnv}
              onChange={handleTabChange}
              variant="scrollable"
              scrollButtons="auto"
              sx={{
                borderBottom: 1,
                borderColor: "divider",
                "& .MuiTab-root": { textTransform: "none", fontWeight: 500, minWidth: 100 },
              }}
            >
              {envTabs.map((tab) => (
                <Tab key={tab.value} label={tab.label} value={tab.value} />
              ))}
            </Tabs>
          </Paper>

          {mainLoading ? (
            <LinearProgress />
          ) : envTabs.length === 0 ? (
            <Typography color="text.secondary" sx={{ mb: 4 }}>
              No deployment data available.
            </Typography>
          ) : (
            <>
              <Typography variant="h6" fontWeight={700} sx={{ mb: 2 }}>
                {activeDeployment?.name}
                {activeDeployment?.type?.label && (
                  <Typography component="span" variant="body2" color="text.secondary" sx={{ ml: 1 }}>
                    · {activeDeployment.type.label}
                  </Typography>
                )}
              </Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 2, flexWrap: "wrap" }}>
                <CalendarIcon size={18} />
                <Typography variant="body2" color="text.secondary" sx={{ mr: 1 }}>
                  Time Range:
                </Typography>

                {(["1M", "3M", "6M", "12M"] as TimeRange[]).map((r) => (
                  <Button
                    key={r}
                    size="small"
                    onClick={() => setTimeRange(r)}
                    variant={timeRange === r ? "contained" : "outlined"}
                    sx={{ borderRadius: "20px", textTransform: "none", minWidth: 52, fontWeight: timeRange === r ? 600 : 400 }}
                  >
                    {r}
                  </Button>
                ))}

                <Button
                  size="small"
                  onClick={() => {
                    if (timeRange !== "Custom") {
                      setPendingFrom(dateFrom);
                      setPendingTo(dateTo);
                      setTimeRange("Custom");
                    }
                  }}
                  variant={timeRange === "Custom" ? "contained" : "outlined"}
                  sx={{ borderRadius: "20px", textTransform: "none", minWidth: 64 }}
                >
                  Custom
                </Button>

                {timeRange === "Custom" && (
                  <>
                    <TextField
                      type="date"
                      size="small"
                      value={pendingFrom}
                      onChange={(e) => setPendingFrom(e.target.value)}
                      slotProps={{ inputLabel: { shrink: true }, htmlInput: { min: MIN_ALLOWED_DATE, max: todayStr() } }}
                      sx={{ width: 150, "& .MuiOutlinedInput-root": { borderRadius: "8px" } }}
                    />
                    <Typography variant="body2" color="text.secondary">
                      to
                    </Typography>
                    <TextField
                      type="date"
                      size="small"
                      value={pendingTo}
                      onChange={(e) => setPendingTo(e.target.value)}
                      slotProps={{ inputLabel: { shrink: true }, htmlInput: { min: MIN_ALLOWED_DATE, max: todayStr() } }}
                      sx={{ width: 150, "& .MuiOutlinedInput-root": { borderRadius: "8px" } }}
                    />
                    <Button
                      size="small"
                      variant="contained"
                      onClick={() => {
                        setCustomFrom(pendingFrom);
                        setCustomTo(pendingTo);
                      }}
                      disabled={
                        !pendingFrom ||
                        !pendingTo ||
                        isNaN(new Date(pendingFrom).getTime()) ||
                        isNaN(new Date(pendingTo).getTime()) ||
                        new Date(pendingFrom) >= new Date(pendingTo) ||
                        new Date(pendingTo).getTime() - new Date(pendingFrom).getTime() > MAX_RANGE_DAYS * 86_400_000
                      }
                      sx={{ borderRadius: "8px", textTransform: "none", px: 2 }}
                    >
                      Apply
                    </Button>
                    <Button
                      size="small"
                      variant="outlined"
                      onClick={() => setTimeRange("1M")}
                      sx={{ borderRadius: "8px", textTransform: "none" }}
                    >
                      Cancel
                    </Button>
                    <Box sx={{ ml: "auto" }}>
                      <Typography variant="caption" color="text.secondary">
                        Custom Range: {customFrom} → {customTo}
                      </Typography>
                    </Box>
                    {new Date(pendingFrom) >= new Date(pendingTo) && (
                      <Alert severity="error" sx={{ mt: 1, width: "100%", borderRadius: "8px" }}>
                        Start date must be before end date.
                      </Alert>
                    )}
                    {new Date(pendingFrom) < new Date(pendingTo) &&
                      new Date(pendingTo).getTime() - new Date(pendingFrom).getTime() > MAX_RANGE_DAYS * 86_400_000 && (
                        <Alert severity="warning" sx={{ mt: 1, width: "100%", borderRadius: "8px" }}>
                          This dashboard supports a maximum date range of {MAX_RANGE_DAYS} days. Please adjust your
                          selection.
                        </Alert>
                      )}
                  </>
                )}
              </Box>

              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Product breakdown for this deployment
              </Typography>

              {prodBreakdownLoading ? (
                <LinearProgress />
              ) : productBreakdown.length === 0 ? (
                <Typography color="text.secondary">No product data available for this deployment.</Typography>
              ) : (
                productBreakdown.map((p) => (
                  <ProductBreakdownRow
                    key={p.id}
                    {...p}
                    deploymentId={activeDepId}
                    coreMetrics={prodMetricsStats.get(p.id)}
                    usageCounts={prodUsagesStats.get(p.id)}
                    statsLoading={prodMetricsStatsLoading || prodUsagesStatsLoading}
                    instancesData={prodInstances.get(p.id)}
                    instancesLoading={prodInstancesLoading}
                    expanded={expandedProductIds.has(p.id)}
                    onToggle={() => handleProductToggle(p.id)}
                  />
                ))
              )}
            </>
          )}
        </>
      )}
    </Box>
  );
}
