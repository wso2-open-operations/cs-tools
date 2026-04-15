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
  Box,
  Button,
  Typography,
  TextField,
  InputAdornment,
} from "@wso2/oxygen-ui";
import { Calendar, ChartColumn } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import { useParams } from "react-router";
import TabBar from "@components/tab-bar/TabBar";
import UsageOverviewPanel from "@features/project-details/components/usage-metrics/UsageOverviewPanel";
import UsageEnvironmentProductsPanel from "@features/project-details/components/usage-metrics/UsageEnvironmentProductsPanel";
import { USAGE_TIME_RANGE_LABELS } from "@features/usage-metrics/constants/usageMetricsConstants";
import { UsageTimeRange } from "@features/project-details/types/usage";
import type { TabOption } from "@components/tab-bar/TabBar";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";

/** Compute ISO date strings for the selected preset relative to today. */
function resolveDateRange(preset: UsageTimeRange): {
  startDate: string;
  endDate: string;
} {
  const end = new Date();
  const start = new Date(end);
  if (preset === UsageTimeRange.THREE_MONTHS) {
    start.setMonth(start.getMonth() - 3);
  } else if (preset === UsageTimeRange.SIX_MONTHS) {
    start.setMonth(start.getMonth() - 6);
  } else {
    start.setFullYear(start.getFullYear() - 1);
  }
  return {
    startDate: start.toISOString().slice(0, 10),
    endDate: end.toISOString().slice(0, 10),
  };
}

/**
 * Usage & Metrics area: time range, inner environment tabs, overview and product drill-downs.
 *
 * @returns {JSX.Element} Full usage metrics experience for project details.
 */
export default function UsageAndMetricsTabContent(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const [timeRange, setTimeRange] = useState<UsageTimeRange>(
    UsageTimeRange.THREE_MONTHS,
  );
  const [innerTab, setInnerTab] = useState<string>("um-overview");
  const [expandedEnvironmentIds, setExpandedEnvironmentIds] = useState<
    Set<string>
  >(() => new Set());
  const [expandedProductIds, setExpandedProductIds] = useState<Set<string>>(
    () => new Set(),
  );

  const [customStart, setCustomStart] = useState<string>("");
  const [customEnd, setCustomEnd] = useState<string>("");
  const [appliedCustomStart, setAppliedCustomStart] = useState<string>("");
  const [appliedCustomEnd, setAppliedCustomEnd] = useState<string>("");

  const timeLabel = USAGE_TIME_RANGE_LABELS[timeRange];
  const dateRange = useMemo(() => resolveDateRange(timeRange), [timeRange]);

  const { data: deploymentsData } = usePostProjectDeploymentsSearchAll(
    projectId ?? "",
  );

  // Build dynamic tabs from deployments returned by the API — one tab per deployment, labelled by dep.name.
  const innerTabs = useMemo((): TabOption[] => {
    const overviewTab: TabOption = {
      id: "um-overview",
      label: "Overview",
      icon: ChartColumn,
    };
    if (!deploymentsData || deploymentsData.length === 0) {
      return [overviewTab];
    }
    const envTabs: TabOption[] = deploymentsData.map((dep) => ({
      id: `um-dep-${dep.id}`,
      label: dep.name,
    }));
    return [overviewTab, ...envTabs];
  }, [deploymentsData]);

  // Derive the deployment ID for the selected tab (null = overview).
  const activeDeploymentId = useMemo((): string | null => {
    if (innerTab === "um-overview") return null;
    if (innerTab.startsWith("um-dep-")) return innerTab.slice("um-dep-".length);
    return null;
  }, [innerTab]);

  const toggleEnvironment = useCallback((id: string) => {
    setExpandedEnvironmentIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const toggleProduct = useCallback((productId: string) => {
    setExpandedProductIds((prev) => {
      const next = new Set(prev);
      if (next.has(productId)) {
        next.delete(productId);
      } else {
        next.add(productId);
      }
      return next;
    });
  }, []);

  const handleApplyCustom = () => {
    setAppliedCustomStart(customStart);
    setAppliedCustomEnd(customEnd);
  };

  const handleCancelCustom = () => {
    setCustomStart(appliedCustomStart);
    setCustomEnd(appliedCustomEnd);
    setTimeRange(UsageTimeRange.THREE_MONTHS); // Revert to default or last valid maybe
  };

  const timeRangeSelector = (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        mb: 1,
        mt: innerTab !== "um-overview" ? 1 : 0,
        overflowX: "auto",
        pb: 0.5,
      }}
    >
      <Box
        sx={{ display: "flex", alignItems: "center", gap: 1.5, flexShrink: 0 }}
      >
        <Calendar size={18} />
        <Typography variant="body2" sx={{ fontWeight: 600 }}>
          Time Range:
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          {(
            [
              UsageTimeRange.THREE_MONTHS,
              UsageTimeRange.SIX_MONTHS,
              UsageTimeRange.TWELVE_MONTHS,
            ] as UsageTimeRange[]
          ).map((preset) => {
            const selected = timeRange === preset;
            return (
              <Button
                key={preset}
                size="small"
                variant={selected ? "contained" : "outlined"}
                color={selected ? "warning" : "inherit"}
                onClick={() => {
                  setTimeRange(preset);
                  setAppliedCustomStart("");
                  setAppliedCustomEnd("");
                }}
                sx={{ textTransform: "none", minWidth: 48 }}
              >
                {preset === "3m" ? "3M" : preset === "6m" ? "6M" : "12M"}
              </Button>
            );
          })}
          <Button
            size="small"
            variant={
              timeRange === UsageTimeRange.CUSTOM ? "contained" : "outlined"
            }
            color={timeRange === UsageTimeRange.CUSTOM ? "warning" : "inherit"}
            onClick={() => setTimeRange(UsageTimeRange.CUSTOM)}
            sx={{ textTransform: "none", minWidth: 48 }}
          >
            Custom
          </Button>

          {timeRange === UsageTimeRange.CUSTOM && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 2, ml: 1 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <TextField
                  type="date"
                  size="small"
                  value={customStart}
                  onChange={(e) => setCustomStart(e.target.value)}
                  sx={{ minWidth: 160 }}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <Calendar size={16} />
                      </InputAdornment>
                    ),
                  }}
                />
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mx: 0.5 }}
                >
                  to
                </Typography>
                <TextField
                  type="date"
                  size="small"
                  value={customEnd}
                  onChange={(e) => setCustomEnd(e.target.value)}
                  sx={{ minWidth: 160 }}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <Calendar size={16} />
                      </InputAdornment>
                    ),
                  }}
                />
              </Box>
              <Box sx={{ display: "flex", gap: 1 }}>
                <Button
                  size="small"
                  variant="contained"
                  color="warning"
                  onClick={handleApplyCustom}
                  disabled={!customStart || !customEnd}
                >
                  Apply
                </Button>
                <Button
                  size="small"
                  variant="outlined"
                  color="inherit"
                  onClick={handleCancelCustom}
                >
                  Cancel
                </Button>
              </Box>
            </Box>
          )}
        </Box>
      </Box>

      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ minWidth: 100, textAlign: "right", flexShrink: 0 }}
      >
        {timeRange === "custom"
          ? appliedCustomStart && appliedCustomEnd
            ? `${appliedCustomStart} to ${appliedCustomEnd}`
            : "Select custom range"
          : timeLabel}
      </Typography>
    </Box>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <TabBar
        tabs={innerTabs}
        activeTab={innerTab}
        onTabChange={setInnerTab}
        sx={{ mb: 1 }}
      />

      {innerTab !== "um-overview" && timeRangeSelector}

      {innerTab === "um-overview" && (
        <UsageOverviewPanel
          projectId={projectId}
          dateRange={dateRange}
          expandedEnvironmentIds={expandedEnvironmentIds}
          onToggleEnvironment={toggleEnvironment}
          timeRangeSelector={timeRangeSelector}
        />
      )}

      {activeDeploymentId != null && (
        <UsageEnvironmentProductsPanel
          deploymentId={activeDeploymentId}
          projectId={projectId}
          dateRange={dateRange}
          expandedProductIds={expandedProductIds}
          onToggleProduct={toggleProduct}
        />
      )}
    </Box>
  );
}
