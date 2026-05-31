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
import AdminTabs from "@features/csm-admin/components/AdminTabs";
import type { JSX } from "react";

/**
 * Mocked Teams view. Per BRD §Team Management — teams group engineers by
 * functional area + lead and feed assignment routing rules.
 *
 * Sampled from production SN sys_user_group rows (the ones flagged as
 * type=1cb8ab9bff500200158bffffffffff62, the SN "team" type). Real lead/skill
 * assignment lives in the backend; this UI is for shape review only.
 */
interface TeamRow {
  id: string;
  name: string;
  function: "CRE" | "SRE" | "FDE" | "Operational" | "Leadership";
  leadName: string;
  memberCount: number;
  primarySkills: string[];
  onCallEnabled: boolean;
}

const TEAMS: TeamRow[] = [
  { id: "team.cre_atlas", name: "CRE — Atlas", function: "CRE", leadName: "Sajith Ekanayaka", memberCount: 8, primarySkills: ["API Manager", "IS"], onCallEnabled: true },
  { id: "team.cre_apollo", name: "CRE — Apollo", function: "CRE", leadName: "Priya N.", memberCount: 6, primarySkills: ["Identity", "OIDC"], onCallEnabled: true },
  { id: "team.sre_apollo", name: "Apollo SRE", function: "SRE", leadName: "Maya R.", memberCount: 5, primarySkills: ["Choreo", "K8s"], onCallEnabled: true },
  { id: "team.sre_artemis", name: "Artemis SRE", function: "SRE", leadName: "Kasun H.", memberCount: 4, primarySkills: ["Asgardeo", "Identity SaaS"], onCallEnabled: true },
  { id: "team.sre_bcentral", name: "B-Central SRE", function: "SRE", leadName: "Kasun H.", memberCount: 3, primarySkills: ["Ballerina"], onCallEnabled: false },
  { id: "team.sre_choreo", name: "Choreo SRE", function: "SRE", leadName: "Maya R.", memberCount: 7, primarySkills: ["Choreo", "K8s"], onCallEnabled: true },
  { id: "team.fde_integration", name: "FDE — Integration", function: "FDE", leadName: "Tharindu A.", memberCount: 5, primarySkills: ["MI", "ESB", "API Manager"], onCallEnabled: false },
  { id: "team.ops_leadership", name: "Operational Leadership", function: "Operational", leadName: "Nadeesha S.", memberCount: 4, primarySkills: ["Cross-team"], onCallEnabled: false },
  { id: "team.exec_leadership", name: "Business Leadership", function: "Leadership", leadName: "Nadeesha S.", memberCount: 3, primarySkills: ["Executive escalation"], onCallEnabled: false },
];

function functionColor(fn: TeamRow["function"]): "primary" | "info" | "success" | "warning" | "default" {
  switch (fn) {
    case "CRE": return "primary";
    case "SRE": return "info";
    case "FDE": return "success";
    case "Operational": return "warning";
    case "Leadership": return "default";
  }
}

export default function CsmAdminTeamsPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AdminTabs />
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Team</TableCell>
                <TableCell>Function</TableCell>
                <TableCell>Lead</TableCell>
                <TableCell align="right">Members</TableCell>
                <TableCell>Primary skills</TableCell>
                <TableCell>On-call</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {TEAMS.map((t) => (
                <TableRow key={t.id} hover>
                  <TableCell>{t.name}</TableCell>
                  <TableCell>
                    <Chip size="small" label={t.function} color={functionColor(t.function)} variant="outlined" />
                  </TableCell>
                  <TableCell>{t.leadName}</TableCell>
                  <TableCell align="right">{t.memberCount}</TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={0.5} sx={{ flexWrap: "wrap", gap: 0.5 }}>
                      {t.primarySkills.map((s) => (
                        <Chip key={s} size="small" label={s} variant="outlined" />
                      ))}
                    </Stack>
                  </TableCell>
                  <TableCell>
                    {t.onCallEnabled ? (
                      <Chip size="small" label="Enabled" color="success" variant="outlined" />
                    ) : (
                      <Typography variant="body2" color="text.secondary">—</Typography>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Mocked from SN sys_user_group production rows. Routing rules and on-call schedules wire up once the team-management endpoints land.
      </Typography>
    </Box>
  );
}
