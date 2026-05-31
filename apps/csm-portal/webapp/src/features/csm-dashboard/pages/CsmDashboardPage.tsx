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
import { useState, type JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useEffect, useRef } from "react";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import MyQueueSection from "@features/csm-dashboard/components/MyQueueSection";
import SlaAtRiskSection from "@features/csm-dashboard/components/SlaAtRiskSection";
import CustomerSummarySection from "@features/csm-dashboard/components/CustomerSummarySection";
import RecentActivitySection from "@features/csm-dashboard/components/RecentActivitySection";
import { useGetCsmDashboard } from "@features/csm-dashboard/api/useGetCsmDashboard";
import type { DashboardScope } from "@features/csm-dashboard/types/abtDashboard";

/**
 * Top-level CSM ABT dashboard. Engineer-scoped, not project-scoped.
 *
 * Sections (placeholders until OQ#CSM-D is resolved):
 *   1. My queue
 *   2. SLA at risk
 *   3. Customers in scope
 *   4. Recent activity
 */
export default function CsmDashboardPage(): JSX.Element {
  const [scope, setScope] = useState<DashboardScope>("my_abt");
  const { data, isLoading, isError } = useGetCsmDashboard(scope);
  const { showError } = useErrorBanner();
  const hasShownErrorRef = useRef(false);

  useEffect(() => {
    if (isError && !hasShownErrorRef.current) {
      hasShownErrorRef.current = true;
      showError("Could not load dashboard.");
    }
    if (!isError) hasShownErrorRef.current = false;
  }, [isError, showError]);

  const scopeLabel =
    scope === "my_abt"
      ? data?.engineer?.abtName
        ? `In ${data.engineer.abtName}`
        : "ABT scope"
      : "All customers";

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AbtDashboardHeader
        engineer={data?.engineer}
        scope={scope}
        onScopeChange={setScope}
      />
      <Box
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: {
            xs: "1fr",
            md: "repeat(2, minmax(0, 1fr))",
          },
        }}
      >
        <MyQueueSection queue={data?.queue} isLoading={isLoading} />
        <SlaAtRiskSection cases={data?.slaAtRisk} isLoading={isLoading} />
      </Box>
      <Box
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: {
            xs: "1fr",
            md: "minmax(0, 7fr) minmax(0, 5fr)",
          },
        }}
      >
        <CustomerSummarySection
          customers={data?.customers}
          isLoading={isLoading}
          scopeLabel={scopeLabel}
        />
        <RecentActivitySection
          activity={data?.recentActivity}
          isLoading={isLoading}
        />
      </Box>
    </Box>
  );
}
