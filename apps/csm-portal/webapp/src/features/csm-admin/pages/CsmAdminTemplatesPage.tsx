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
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { isMockMode } from "@api/backend/client";
import AdminTabEmpty from "@features/csm-admin/components/AdminTabEmpty";

/** Response Templates — BRD §Response Templates. */
interface TemplateRow {
  id: string;
  name: string;
  scope: "Case reply" | "Closure" | "Acknowledgement" | "Escalation" | "Customer email";
  category: string;
  language: string;
  lastUpdated: string;
  author: string;
}

const TEMPLATES: TemplateRow[] = [
  { id: "tpl.ack_s1", name: "S1 acknowledgement", scope: "Acknowledgement", category: "Severity P1", language: "EN", lastUpdated: "2026-04-18", author: "Priya N." },
  { id: "tpl.ack_s0", name: "S0 acknowledgement + escalation notice", scope: "Acknowledgement", category: "Severity P0", language: "EN", lastUpdated: "2026-05-03", author: "Sajith Ekanayaka" },
  { id: "tpl.req_info", name: "Awaiting customer info — generic", scope: "Case reply", category: "Information request", language: "EN", lastUpdated: "2026-03-29", author: "Priya N." },
  { id: "tpl.req_logs", name: "Awaiting logs / heap dump", scope: "Case reply", category: "Information request", language: "EN", lastUpdated: "2026-02-11", author: "Sajith Ekanayaka" },
  { id: "tpl.sol_proposed", name: "Solution proposed — verify and accept", scope: "Case reply", category: "Resolution", language: "EN", lastUpdated: "2026-04-22", author: "Sajith Ekanayaka" },
  { id: "tpl.closure_resolved", name: "Closure — resolved", scope: "Closure", category: "Closure", language: "EN", lastUpdated: "2026-04-22", author: "Sajith Ekanayaka" },
  { id: "tpl.closure_no_response", name: "Closure — no customer response", scope: "Closure", category: "Closure", language: "EN", lastUpdated: "2026-03-04", author: "Priya N." },
  { id: "tpl.escalation_tl", name: "Escalation to Team Lead", scope: "Escalation", category: "Internal", language: "EN", lastUpdated: "2026-04-01", author: "Nadeesha S." },
  { id: "tpl.escalation_rnd", name: "Escalation to R&D", scope: "Escalation", category: "Internal", language: "EN", lastUpdated: "2026-04-01", author: "Nadeesha S." },
  { id: "tpl.cust_eol", name: "Product EOL announcement", scope: "Customer email", category: "Announcement", language: "EN", lastUpdated: "2026-01-15", author: "Nadeesha S." },
];

function scopeColor(s: TemplateRow["scope"]): "primary" | "info" | "success" | "warning" | "default" {
  switch (s) {
    case "Case reply": return "primary";
    case "Acknowledgement": return "info";
    case "Closure": return "success";
    case "Escalation": return "warning";
    case "Customer email": return "default";
  }
}

export default function CsmAdminTemplatesPage(): JSX.Element {
  if (!isMockMode()) return <AdminTabEmpty resource="response templates" />;
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Template</TableCell>
                <TableCell>Scope</TableCell>
                <TableCell>Category</TableCell>
                <TableCell>Language</TableCell>
                <TableCell>Author</TableCell>
                <TableCell>Last updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {TEMPLATES.map((t) => (
                <TableRow key={t.id} hover>
                  <TableCell>{t.name}</TableCell>
                  <TableCell>
                    <Chip size="small" label={t.scope} color={scopeColor(t.scope)} variant="outlined" />
                  </TableCell>
                  <TableCell>{t.category}</TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={0.5}>
                      <Chip size="small" label={t.language} variant="outlined" />
                    </Stack>
                  </TableCell>
                  <TableCell>{t.author}</TableCell>
                  <TableCell>{t.lastUpdated}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Templates surface in the case comment editor as quick-insert blocks. Editor + version history wire up once the response-template endpoints land.
      </Typography>
    </Box>
  );
}
