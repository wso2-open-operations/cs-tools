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

/** SLA / Policy Management — BRD §SLA / Policy Management. */
interface SlaRow {
  id: string;
  subscriptionType: string;
  tier: "Platinum" | "Gold" | "Silver" | "Bronze";
  severity: "P0" | "P1" | "P2" | "P3" | "P4";
  responseMins: number;
  resolutionMins: number;
  twilioAlert: boolean;
  active: boolean;
}

const SLAS: SlaRow[] = [
  { id: "sla.plat.p0", subscriptionType: "Subscription",          tier: "Platinum", severity: "P0", responseMins: 15,   resolutionMins: 240,   twilioAlert: true,  active: true },
  { id: "sla.plat.p1", subscriptionType: "Subscription",          tier: "Platinum", severity: "P1", responseMins: 30,   resolutionMins: 480,   twilioAlert: true,  active: true },
  { id: "sla.plat.p2", subscriptionType: "Subscription",          tier: "Platinum", severity: "P2", responseMins: 60,   resolutionMins: 1440,  twilioAlert: true,  active: true },
  { id: "sla.plat.p3", subscriptionType: "Subscription",          tier: "Platinum", severity: "P3", responseMins: 240,  resolutionMins: 2880,  twilioAlert: false, active: true },
  { id: "sla.gold.p0", subscriptionType: "Subscription",          tier: "Gold",     severity: "P0", responseMins: 30,   resolutionMins: 480,   twilioAlert: true,  active: true },
  { id: "sla.gold.p1", subscriptionType: "Subscription",          tier: "Gold",     severity: "P1", responseMins: 60,   resolutionMins: 720,   twilioAlert: true,  active: true },
  { id: "sla.gold.p2", subscriptionType: "Subscription",          tier: "Gold",     severity: "P2", responseMins: 240,  resolutionMins: 2880,  twilioAlert: false, active: true },
  { id: "sla.silver.p0", subscriptionType: "Subscription",        tier: "Silver",   severity: "P0", responseMins: 60,   resolutionMins: 720,   twilioAlert: true,  active: true },
  { id: "sla.silver.p1", subscriptionType: "Subscription",        tier: "Silver",   severity: "P1", responseMins: 240,  resolutionMins: 1440,  twilioAlert: false, active: true },
  { id: "sla.mc.p0",   subscriptionType: "Managed cloud",          tier: "Platinum", severity: "P0", responseMins: 10,   resolutionMins: 120,   twilioAlert: true,  active: true },
  { id: "sla.mc.p1",   subscriptionType: "Managed cloud",          tier: "Platinum", severity: "P1", responseMins: 30,   resolutionMins: 360,   twilioAlert: true,  active: true },
  { id: "sla.dev.p2",  subscriptionType: "Development support",    tier: "Bronze",   severity: "P2", responseMins: 480,  resolutionMins: 0,     twilioAlert: false, active: true },
  { id: "sla.eval.p3", subscriptionType: "Evaluation subscription",tier: "Bronze",   severity: "P3", responseMins: 1440, resolutionMins: 0,     twilioAlert: false, active: true },
];

function fmtMins(m: number): string {
  if (m === 0) return "Best effort";
  if (m < 60) return `${m} min`;
  const h = m / 60;
  if (h < 24) return `${h} h`;
  return `${(h / 24).toFixed(1)} d`;
}

function sevColor(s: SlaRow["severity"]): "error" | "warning" | "info" | "success" | "default" {
  switch (s) {
    case "P0": return "error";
    case "P1": return "error";
    case "P2": return "warning";
    case "P3": return "info";
    case "P4": return "success";
  }
}

export default function CsmAdminSlaPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Subscription type</TableCell>
                <TableCell>Tier</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Initial response</TableCell>
                <TableCell>Target resolution</TableCell>
                <TableCell>Twilio alert</TableCell>
                <TableCell>Status</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {SLAS.map((s) => (
                <TableRow key={s.id} hover>
                  <TableCell>{s.subscriptionType}</TableCell>
                  <TableCell>
                    <Chip size="small" label={s.tier} variant="outlined" />
                  </TableCell>
                  <TableCell>
                    <Chip size="small" label={s.severity} color={sevColor(s.severity)} variant="outlined" />
                  </TableCell>
                  <TableCell>{fmtMins(s.responseMins)}</TableCell>
                  <TableCell>{fmtMins(s.resolutionMins)}</TableCell>
                  <TableCell>
                    {s.twilioAlert ? (
                      <Chip size="small" label="On" color="warning" variant="outlined" />
                    ) : (
                      <Typography variant="body2" color="text.secondary">—</Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    {s.active ? (
                      <Chip size="small" label="Active" color="success" variant="outlined" />
                    ) : (
                      <Chip size="small" label="Inactive" color="default" variant="outlined" />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Per-tier and per-severity SLA policies. Editing, audit log, and per-customer overrides wire up with the SLA backend.
      </Typography>
    </Box>
  );
}
