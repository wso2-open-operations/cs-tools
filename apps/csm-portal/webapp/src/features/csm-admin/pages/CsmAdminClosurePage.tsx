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
import type { JSX } from "react";

/** Subscription Closure Management — BRD §Subscription Closure Management. */
interface ClosureRow {
  id: string;
  account: string;
  project: string;
  subscriptionType: string;
  effectiveDate: string;
  stage: "Notified" | "Knowledge transfer" | "Data export" | "Pending sign-off" | "Closed";
  openCases: number;
  blockers?: string;
}

const ROWS: ClosureRow[] = [
  { id: "cl.1", account: "Idaho Dept of Health And Welfare (IDHW)", project: "IDHW-IS-PROD",   subscriptionType: "Subscription",            effectiveDate: "2026-06-30", stage: "Notified", openCases: 3 },
  { id: "cl.2", account: "ixtel technologies",                       project: "IXTEL-APIM-PROD",  subscriptionType: "Development support",     effectiveDate: "2026-06-15", stage: "Knowledge transfer", openCases: 1 },
  { id: "cl.3", account: "Currency Cloud",                           project: "CC-MI-PROD",       subscriptionType: "Subscription",            effectiveDate: "2026-07-15", stage: "Data export", openCases: 0 },
  { id: "cl.4", account: "Sandstone Technology",                     project: "SST-IS-DEV",       subscriptionType: "Development support",     effectiveDate: "2026-05-31", stage: "Pending sign-off", openCases: 0, blockers: "Final invoice pending" },
  { id: "cl.5", account: "HTM Personenvervoer N.V.",                 project: "HTM-SP-PROD",      subscriptionType: "Subscription",            effectiveDate: "2026-04-30", stage: "Closed", openCases: 0 },
  { id: "cl.6", account: "Standard Chartered Bank Singapore",        project: "SCBS-APIM-PROD",   subscriptionType: "Managed cloud subscription", effectiveDate: "2026-09-30", stage: "Notified", openCases: 7, blockers: "5 open S2 cases must close before subscription end" },
];

function stageColor(s: ClosureRow["stage"]): "primary" | "info" | "warning" | "success" | "default" {
  switch (s) {
    case "Notified": return "info";
    case "Knowledge transfer": return "primary";
    case "Data export": return "primary";
    case "Pending sign-off": return "warning";
    case "Closed": return "success";
  }
}

export default function CsmAdminClosurePage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Account</TableCell>
                <TableCell>Project</TableCell>
                <TableCell>Subscription type</TableCell>
                <TableCell>Effective</TableCell>
                <TableCell>Stage</TableCell>
                <TableCell align="right">Open cases</TableCell>
                <TableCell>Blockers</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {ROWS.map((r) => (
                <TableRow key={r.id} hover>
                  <TableCell>{r.account}</TableCell>
                  <TableCell>{r.project}</TableCell>
                  <TableCell>{r.subscriptionType}</TableCell>
                  <TableCell>{r.effectiveDate}</TableCell>
                  <TableCell>
                    <Chip size="small" label={r.stage} color={stageColor(r.stage)} variant="outlined" />
                  </TableCell>
                  <TableCell align="right">{r.openCases}</TableCell>
                  <TableCell>
                    {r.blockers ? (
                      <Typography variant="caption" color="warning.main">{r.blockers}</Typography>
                    ) : (
                      <Typography variant="caption" color="text.secondary">—</Typography>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Tracks the lifecycle of subscription closures end-to-end. Notifications, KT handover, and data export flows wire up with the closure backend.
      </Typography>
    </Box>
  );
}
