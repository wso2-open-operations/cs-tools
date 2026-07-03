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
import { useState, type JSX } from "react";
import AbtDashboardHeader from "@features/csm-dashboard/components/AbtDashboardHeader";
import CaseCompositionCharts from "@features/csm-dashboard/components/CaseCompositionCharts";
import CaseCountsMatrix from "@features/csm-dashboard/components/CaseCountsMatrix";
import MyAssignedCases from "@features/csm-dashboard/components/MyAssignedCases";
import { useGetCsmDashboard } from "@features/csm-dashboard/api/useGetCsmDashboard";
import {
  DASHBOARD_OPTIONS,
  type DashboardKey,
  type DashboardScope,
} from "@features/csm-dashboard/types/abtDashboard";

/**
 * Top-level CSM dashboard. Currently locked to the Engineer dashboard and
 * showing only the "Cases by severity and state" matrix: the queue / SLA /
 * customers / activity widgets are hidden, and the dashboard switcher dropdown
 * (Operations, IAM CS, Security, Team performance) is disabled in the header
 * because those are mock placeholders. Re-enable via DASHBOARD_SWITCHER_ENABLED
 * in AbtDashboardHeader and restore the hidden sections here once the real
 * tab+widget model (DashboardsAndReportsProposal.md, entity-service reports
 * DSL) lands. The placeholder dashboards below are kept for that restore.
 */
export default function CsmDashboardPage(): JSX.Element {
  // ABT scoping is not implemented yet, so default to (and stay on)
  // all-customers; the My ABT / All customers toggle is disabled in the header.
  const [scope, setScope] = useState<DashboardScope>("all_customers");
  // Locked to the Engineer dashboard: the switcher is disabled in the header
  // (the other dashboards are mock placeholders), so this never changes today.
  const [dashboardKey, setDashboardKey] = useState<DashboardKey>("engineer");
  // Only the engineer-overview header consumes this now; the queue/SLA/customer/
  // activity widgets are hidden, leaving the standalone severity-by-state matrix
  // (CaseCountsMatrix, which loads from its own source) as the only widget.
  const { data, isError } = useGetCsmDashboard(scope);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AbtDashboardHeader
        engineer={data?.engineer}
        scope={scope}
        onScopeChange={setScope}
        dashboardKey={dashboardKey}
        onDashboardChange={setDashboardKey}
        isError={isError}
      />
      {dashboardKey === "engineer" ? (
        <>
          <MyAssignedCases />
          <CaseCountsMatrix />
          <CaseCompositionCharts />
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
