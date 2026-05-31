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
  Card,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import AdminTabs from "@features/csm-admin/components/AdminTabs";
import type { JSX } from "react";

/** Reports — BRD §Reports. */
interface ReportRow {
  id: string;
  name: string;
  category: "CRE workspace" | "Operational" | "Customer" | "Executive";
  cadence: "Daily" | "Weekly" | "Monthly" | "On demand";
  recipients: string[];
  lastRun: string;
  active: boolean;
}

const REPORTS: ReportRow[] = [
  { id: "rpt.cre_workspace", name: "CRE Engineer workspace", category: "CRE workspace", cadence: "Daily",   recipients: ["Each CRE Engineer"],          lastRun: "2026-05-31 06:00", active: true },
  { id: "rpt.sla_at_risk",  name: "SLA at-risk daily digest", category: "Operational",   cadence: "Daily",   recipients: ["CRE Leads", "SRE Leads"],     lastRun: "2026-05-31 06:00", active: true },
  { id: "rpt.weekly_team",  name: "Weekly team performance",  category: "Operational",   cadence: "Weekly",  recipients: ["CRE Leads", "Engineering Mgmt"], lastRun: "2026-05-26 06:00", active: true },
  { id: "rpt.customer_health",   name: "Customer health score",          category: "Customer",    cadence: "Weekly",  recipients: ["Account Managers"],            lastRun: "2026-05-26 06:00", active: true },
  { id: "rpt.escalations",  name: "Escalations summary",       category: "Operational",   cadence: "Weekly",  recipients: ["Operational Leads"],          lastRun: "2026-05-26 06:00", active: true },
  { id: "rpt.security_posture",  name: "Security posture",      category: "Executive",     cadence: "Monthly", recipients: ["Leadership"],                  lastRun: "2026-05-01 06:00", active: true },
  { id: "rpt.exec_health",  name: "Executive operational health", category: "Executive",   cadence: "Monthly", recipients: ["Leadership"],                  lastRun: "2026-05-01 06:00", active: true },
  { id: "rpt.engagement_pipeline",name: "Engagement pipeline",  category: "Customer",     cadence: "Monthly", recipients: ["FDE Leads", "Account Managers"], lastRun: "2026-05-01 06:00", active: true },
  { id: "rpt.adhoc_breach",      name: "SLA breach drill-down",   category: "Operational",  cadence: "On demand", recipients: ["—"],                              lastRun: "—",                  active: true },
];

function catColor(c: ReportRow["category"]): "primary" | "info" | "success" | "warning" {
  switch (c) {
    case "CRE workspace": return "primary";
    case "Operational": return "info";
    case "Customer": return "success";
    case "Executive": return "warning";
  }
}

export default function CsmAdminReportsPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AdminTabs />
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Report</TableCell>
                <TableCell>Category</TableCell>
                <TableCell>Cadence</TableCell>
                <TableCell>Recipients</TableCell>
                <TableCell>Last run</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {REPORTS.map((r) => (
                <TableRow key={r.id} hover>
                  <TableCell>{r.name}</TableCell>
                  <TableCell>
                    <Chip size="small" label={r.category} color={catColor(r.category)} variant="outlined" />
                  </TableCell>
                  <TableCell>
                    <Chip size="small" label={r.cadence} variant="outlined" />
                  </TableCell>
                  <TableCell>{r.recipients.join(", ")}</TableCell>
                  <TableCell>{r.lastRun}</TableCell>
                  <TableCell>
                    {r.active ? (
                      <Chip size="small" label="Active" color="success" variant="outlined" />
                    ) : (
                      <Chip size="small" label="Paused" color="default" variant="outlined" />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Configure scheduled report distribution and on-demand drill-downs. Per-report editor lands with the reports backend.
      </Typography>
    </Box>
  );
}
