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

import { Box, Button, ToggleButton, ToggleButtonGroup } from "@wso2/oxygen-ui";
import { Upload } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";
import type { JSX } from "react";
import { useCallback, useMemo, useState } from "react";
import { DataSource } from "@features/project-details/types/usage";
import {
  USAGE_METRICS_DATA_SOURCE_API_CALL,
  USAGE_METRICS_DATA_SOURCE_FILE_UPLOAD,
  USAGE_METRICS_DATA_SOURCE_LABEL,
  USAGE_METRICS_DEPLOYMENT_TAB_PREFIX,
} from "@features/usage-metrics/constants/usageMetricsConstants";
import { useParams } from "react-router";
import TabBar from "@components/tab-bar/TabBar";
import UsageEnvironmentProductsPanel from "@features/usage-metrics/components/UsageEnvironmentProductsPanel";
import UsageMetricsTimeRangeSelector from "@features/usage-metrics/components/UsageMetricsTimeRangeSelector";
import DeploymentUsageUploadDialog from "@features/usage-metrics/components/DeploymentUsageUploadDialog";
import { UsageTimeRange } from "@features/project-details/types/usage";
import {
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
  const [innerTab, setInnerTab] = useState<string>("");
  const [expandedProductIds, setExpandedProductIds] = useState<Set<string>>(
    () => new Set(),
  );

  const [customStart, setCustomStart] = useState<string>("");
  const [customEnd, setCustomEnd] = useState<string>("");
  const [appliedCustomStart, setAppliedCustomStart] = useState<string>("");
  const [appliedCustomEnd, setAppliedCustomEnd] = useState<string>("");
  const [uploadOpen, setUploadOpen] = useState<boolean>(false);
  const [dataSource] = useState<DataSource>(DataSource.API_CALL);

  const dateRange = useMemo(
    () => resolveUsagePresetDateRange(timeRange),
    [timeRange],
  );

  const { data: deploymentsData } = usePostProjectDeploymentsSearchAll(
    projectId ?? "",
  );

  const deploymentTabs = useMemo(
    () =>
      (deploymentsData ?? []).map((dep) => ({
        id: `${USAGE_METRICS_DEPLOYMENT_TAB_PREFIX}${dep.id}`,
        label: dep.name,
        instanceCount: dep.instanceCount ?? 0,
        productCount: dep.productCount ?? 0,
      })),
    [deploymentsData],
  );

  // Default to the first deployment that has products, fallback to first tab.
  const defaultTab = useMemo(() => {
    const withProducts = deploymentTabs.find((t) => t.productCount > 0);
    return (withProducts ?? deploymentTabs[0])?.id ?? "";
  }, [deploymentTabs]);

  const activeTab = innerTab || defaultTab;

  const activeDeploymentId = useMemo(
    () => getActiveUsageDeploymentId(activeTab),
    [activeTab],
  );

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

  const dataSourceToggle: ReactNode = (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexShrink: 0 }}>
      <Box
        component="span"
        sx={{ fontSize: "0.75rem", color: "text.secondary", whiteSpace: "nowrap" }}
      >
        {USAGE_METRICS_DATA_SOURCE_LABEL}
      </Box>
      <ToggleButtonGroup
        size="small"
        exclusive
        value={dataSource}
        aria-label="data source filter"
        sx={{ pointerEvents: "none" }}
      >
        <ToggleButton value={DataSource.API_CALL} aria-label={USAGE_METRICS_DATA_SOURCE_API_CALL}>
          {USAGE_METRICS_DATA_SOURCE_API_CALL}
        </ToggleButton>
        <ToggleButton value={DataSource.FILE_UPLOAD} aria-label={USAGE_METRICS_DATA_SOURCE_FILE_UPLOAD}>
          {USAGE_METRICS_DATA_SOURCE_FILE_UPLOAD}
        </ToggleButton>
      </ToggleButtonGroup>
    </Box>
  );

  const uploadButton: ReactNode = (
    <Button
      variant="outlined"
      size="small"
      startIcon={<Upload size={16} />}
      onClick={() => setUploadOpen(true)}
      sx={{ flexShrink: 0 }}
    >
      Upload
    </Button>
  );

  const timeRangeSelector = (
    <UsageMetricsTimeRangeSelector
      innerTab={activeTab}
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
      rightAction={
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexShrink: 0 }}>
          {dataSourceToggle}
          {uploadButton}
        </Box>
      }
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
            overflowX: "auto",
            overflowY: "hidden",
            scrollbarWidth: "none",
            "&::-webkit-scrollbar": { display: "none" },
          }}
        >
          <Box sx={{ width: "max-content" }}>
            <TabBar
              tabs={deploymentTabs}
              activeTab={activeTab}
              onTabChange={setInnerTab}
              keepButtonWidth={true}
              compact={true}
              sx={{ mb: 0, border: "none", boxShadow: "none" }}
            />
          </Box>
        </Box>
      </Box>

      <DeploymentUsageUploadDialog
        open={uploadOpen}
        onClose={() => setUploadOpen(false)}
      />

      {timeRangeSelector}

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
