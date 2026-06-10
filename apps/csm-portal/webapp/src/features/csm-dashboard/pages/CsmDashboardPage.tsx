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

import { Box, Card, Chip, Typography } from "@wso2/oxygen-ui";
import { useEffect, useRef, useState, type JSX } from "react";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import MyQueueSection from "@features/csm-dashboard/components/MyQueueSection";
import SlaAtRiskSection from "@features/csm-dashboard/components/SlaAtRiskSection";
import CustomerSummarySection from "@features/csm-dashboard/components/CustomerSummarySection";
import RecentActivitySection from "@features/csm-dashboard/components/RecentActivitySection";
import CaseCountsMatrix from "@features/csm-dashboard/components/CaseCountsMatrix";
import { useGetCsmDashboard } from "@features/csm-dashboard/api/useGetCsmDashboard";
import {
  DASHBOARD_OPTIONS,
  type DashboardKey,
  type DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Top-level CSM dashboard. Hosts the engineer-overview dashboard (queue +
 * SLA + customers + activity) plus four placeholder dashboards selectable
 * from the top-right dropdown (Operations, IAM CS, Security, Team
 * performance). Per DashboardsAndReportsProposal.md the real tab+widget
 * model lives behind the entity-service reports DSL; these placeholders
 * are mocks for UX iteration only.
 */
export default function CsmDashboardPage(): JSX.Element {
  // ABT scoping is not implemented yet, so default to (and stay on)
  // all-customers; the My ABT / All customers toggle is disabled in the header.
  const [scope, setScope] = useState<DashboardScope>("all_customers");
  const [dashboardKey, setDashboardKey] = useState<DashboardKey>("engineer");
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
        dashboardKey={dashboardKey}
        onDashboardChange={setDashboardKey}
      />
      {dashboardKey === "engineer" ? (
        <>
          <CaseCountsMatrix />
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
        </>
      ) : (
        <DashboardPlaceholder dashboardKey={dashboardKey} />
      )}
    </Box>
  );
}

interface DashboardPlaceholderProps {
  dashboardKey: DashboardKey;
}

function DashboardPlaceholder({ dashboardKey }: DashboardPlaceholderProps): JSX.Element {
  const option = DASHBOARD_OPTIONS.find((o) => o.key === dashboardKey);
  if (!option) return <></>;

  // Mock KPI tiles per dashboard. Numbers are pinned (no real query); the
  // shape matches the v1 widget set in DashboardsAndReportsProposal.md.
  const tiles = TILE_SETS[dashboardKey] ?? [];

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Card variant="outlined" sx={{ p: 2.5 }}>
        <Typography variant="h6">{option.name}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          {option.description}
        </Typography>
        <Box sx={{ display: "flex", gap: 0.5, mt: 1 }}>
          <Chip size="small" label="Mock" color="warning" variant="outlined" />
          <Chip size="small" label="No widgets persisted" variant="outlined" />
        </Box>
      </Card>
      <Box
        sx={{
          display: "grid",
          gap: 1.5,
          gridTemplateColumns: {
            xs: "repeat(2, minmax(0, 1fr))",
            sm: "repeat(3, minmax(0, 1fr))",
            md: "repeat(4, minmax(0, 1fr))",
            lg: "repeat(5, minmax(0, 1fr))",
          },
        }}
      >
        {tiles.map((t) => (
          <Card key={t.label} variant="outlined" sx={{ p: 1.75 }}>
            <Typography variant="caption" color="text.secondary">
              {t.label}
            </Typography>
            <Typography
              variant="h5"
              sx={{
                color:
                  t.color === "warning"
                    ? "warning.main"
                    : t.color === "danger"
                      ? "error.main"
                      : t.color === "success"
                        ? "success.main"
                        : "text.primary",
                mt: 0.5,
              }}
            >
              {t.value}
            </Typography>
            {t.sub && (
              <Typography variant="caption" color="text.secondary">
                {t.sub}
              </Typography>
            )}
          </Card>
        ))}
      </Box>
    </Box>
  );
}

type TileColor = "neutral" | "info" | "success" | "warning" | "danger";
interface Tile {
  label: string;
  value: string;
  sub?: string;
  color: TileColor;
}

const TILE_SETS: Record<DashboardKey, Tile[]> = {
  engineer: [],
  operations: [
    { label: "Open cases", value: "287", color: "neutral" },
    { label: "Created today", value: "34", sub: "+12% vs 7d avg", color: "info" },
    { label: "Resolved today", value: "29", sub: "+3% vs 7d avg", color: "success" },
    { label: "Solution proposed", value: "41", color: "neutral" },
    { label: "Awaiting info", value: "62", color: "neutral" },
    { label: "P0/P1 open", value: "9", color: "danger" },
    { label: "P0/P1 breached", value: "2", color: "danger" },
    { label: "Escalations open", value: "11", color: "warning" },
    { label: "SLA breach 24h", value: "4", color: "warning" },
    { label: "Time-card pending approval", value: "18", color: "neutral" },
  ],
  iam: [
    { label: "IS cases open", value: "53", color: "neutral" },
    { label: "Asgardeo cases open", value: "41", color: "neutral" },
    { label: "IS P0/P1 open", value: "3", color: "danger" },
    { label: "Top product: IS 7.1.0", value: "22", sub: "Open cases", color: "info" },
    { label: "Auth-failure clusters", value: "5", color: "warning" },
    { label: "Top account: Bank of Georgia", value: "9", sub: "Open cases", color: "info" },
    { label: "Avg ack time", value: "22 m", sub: "Target 30 m", color: "success" },
    { label: "Avg resolution (P2)", value: "8.4 h", sub: "Target 24 h", color: "success" },
    { label: "Customer satisfaction", value: "4.4 / 5", sub: "Last 30d", color: "success" },
    { label: "Vuln links to active cases", value: "7", color: "warning" },
  ],
  security: [
    { label: "Critical vulns", value: "4", color: "danger" },
    { label: "High vulns", value: "18", color: "warning" },
    { label: "Medium vulns", value: "62", color: "neutral" },
    { label: "Patches released 30d", value: "11", color: "success" },
    { label: "SRA cases open", value: "6", color: "warning" },
    { label: "Avg disclosure SLA", value: "12 d", sub: "Target 14 d", color: "success" },
    { label: "Customers with critical exposure", value: "9", color: "danger" },
    { label: "Affected products", value: "5", color: "neutral" },
    { label: "Pending CVE assignments", value: "3", color: "warning" },
    { label: "Open advisories", value: "27", color: "neutral" },
  ],
  team_performance: [
    { label: "Cases per engineer (7d avg)", value: "5.2", color: "neutral" },
    { label: "First-response within SLA", value: "94%", sub: "Last 30d", color: "success" },
    { label: "Resolution within SLA", value: "89%", sub: "Last 30d", color: "success" },
    { label: "On-call coverage gaps", value: "1", sub: "Bijira SRE — Sun 03:00", color: "warning" },
    { label: "Top performer (cases closed)", value: "Priya N.", sub: "42 last 30d", color: "info" },
    { label: "Most reassigned engineer", value: "Asanka R.", sub: "8 outbound", color: "warning" },
    { label: "Time-card submission rate", value: "97%", color: "success" },
    { label: "Time-card approval lag", value: "1.3 d", color: "neutral" },
    { label: "Median ack time", value: "18 m", sub: "Across all P0–P3", color: "success" },
    { label: "Median resolution (P2)", value: "9.6 h", color: "success" },
  ],
};
