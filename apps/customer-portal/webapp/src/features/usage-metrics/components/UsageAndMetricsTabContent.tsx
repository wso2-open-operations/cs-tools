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

import { Box } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import { useParams } from "react-router";
import TabBar from "@components/tab-bar/TabBar";
import UsageOverviewPanel from "@features/usage-metrics/components/UsageOverviewPanel";
import UsageEnvironmentProductsPanel from "@features/usage-metrics/components/UsageEnvironmentProductsPanel";
import UsageMetricsTimeRangeSelector from "@features/usage-metrics/components/UsageMetricsTimeRangeSelector";
import { UsageMetricsInnerTabId } from "@features/usage-metrics/types/usageMetrics";
import { UsageTimeRange } from "@features/project-details/types/usage";
import {
  buildUsageInnerTabs,
  getActiveUsageDeploymentId,
  resolveUsagePresetDateRange,
} from "@features/usage-metrics/utils/usageMetricsTab";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";

/**
 * Usage & Metrics area: time range, inner environment tabs, overview and product drill-downs.
 *
 * @returns {JSX.Element} Full usage metrics experience for project details.
 */
export default function UsageAndMetricsTabContent(): JSX.Element {
  const { projectId } = useParams<{ projectId: string }>();
  const [timeRange, setTimeRange] = useState<UsageTimeRange>(
    UsageTimeRange.ONE_MONTH,
  );
  const [innerTab, setInnerTab] = useState<string>(
    UsageMetricsInnerTabId.OVERVIEW,
  );
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

  const dateRange = useMemo(
    () => resolveUsagePresetDateRange(timeRange),
    [timeRange],
  );

  const { data: deploymentsData } = usePostProjectDeploymentsSearchAll(
    projectId ?? "",
  );

  const innerTabs = useMemo(
    () => buildUsageInnerTabs(deploymentsData),
    [deploymentsData],
  );

  const overviewTab = useMemo(
    () =>
      innerTabs.find((tab) => tab.id === UsageMetricsInnerTabId.OVERVIEW) ??
      innerTabs[0],
    [innerTabs],
  );

  const deploymentTabs = useMemo(
    () => innerTabs.filter((tab) => tab.id !== overviewTab?.id),
    [innerTabs, overviewTab],
  );

  const activeDeploymentId = useMemo(
    () => getActiveUsageDeploymentId(innerTab),
    [innerTab],
  );

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

  const clearCustomApplied = useCallback(() => {
    setAppliedCustomStart("");
    setAppliedCustomEnd("");
  }, []);

  const handleApplyCustom = () => {
    setAppliedCustomStart(customStart);
    setAppliedCustomEnd(customEnd);
  };

  const handleCancelCustom = () => {
    setCustomStart(appliedCustomStart);
    setCustomEnd(appliedCustomEnd);
    setTimeRange(UsageTimeRange.ONE_MONTH);
  };

  const timeRangeSelector = (
    <UsageMetricsTimeRangeSelector
      innerTab={innerTab}
      timeRange={timeRange}
      onTimeRangeChange={setTimeRange}
      onClearCustomApplied={clearCustomApplied}
      customStart={customStart}
      customEnd={customEnd}
      onCustomStartChange={setCustomStart}
      onCustomEndChange={setCustomEnd}
      onApplyCustom={handleApplyCustom}
      onCancelCustom={handleCancelCustom}
      appliedCustomStart={appliedCustomStart}
      appliedCustomEnd={appliedCustomEnd}
    />
  );

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 2,
        width: "100%",
        maxWidth: "100%",
        minWidth: 0,
        boxSizing: "border-box",
        overflowX: "hidden",
      }}
    >
      <Box
        sx={{
          width: "100%",
          maxWidth: "100%",
          minWidth: 0,
          overflow: "hidden",
          contain: "inline-size",
        }}
      >
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1,
            maxWidth: "100%",
            minWidth: 0,
            overflow: "hidden",
          }}
        >
          {overviewTab && (
            <TabBar
              tabs={[overviewTab]}
              activeTab={innerTab}
              onTabChange={setInnerTab}
              keepButtonWidth={true}
              compact={true}
              sx={{ mb: 0, border: "none", boxShadow: "none", flexShrink: 0, flexGrow: 0 }}
            />
          )}
          <Box
            sx={{
              flex: "1 1 0",
              minWidth: 0,
              maxWidth: "100%",
              overflowX: "auto",
              overflowY: "hidden",
              scrollbarWidth: "none",
              "&::-webkit-scrollbar": { display: "none" },
            }}
          >
            <Box sx={{ width: "max-content" }}>
              <TabBar
                tabs={deploymentTabs}
                activeTab={innerTab}
                onTabChange={setInnerTab}
                keepButtonWidth={true}
                compact={true}
                sx={{ mb: 0, border: "none", boxShadow: "none" }}
              />
            </Box>
          </Box>
        </Box>
      </Box>

      {innerTab !== UsageMetricsInnerTabId.OVERVIEW && timeRangeSelector}

      {innerTab === UsageMetricsInnerTabId.OVERVIEW && (
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
