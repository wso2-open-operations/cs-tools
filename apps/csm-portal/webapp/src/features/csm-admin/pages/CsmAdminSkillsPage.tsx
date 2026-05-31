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
  LinearProgress,
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

/** Skill Management — BRD §Skill management. Feeds case routing. */
interface SkillRow {
  id: string;
  name: string;
  productLine: string;
  engineerCount: number;
  expertCount: number;
  coverageGap?: string;
}

const SKILLS: SkillRow[] = [
  { id: "skl.apim", name: "API Manager", productLine: "Integration", engineerCount: 14, expertCount: 4 },
  { id: "skl.mi", name: "Micro Integrator (MI)", productLine: "Integration", engineerCount: 12, expertCount: 3 },
  { id: "skl.is", name: "Identity Server (IS)", productLine: "Identity", engineerCount: 10, expertCount: 3 },
  { id: "skl.asgardeo", name: "Asgardeo SaaS", productLine: "Identity SaaS", engineerCount: 6, expertCount: 2 },
  { id: "skl.choreo", name: "Choreo", productLine: "Platform SaaS", engineerCount: 9, expertCount: 3 },
  { id: "skl.bcentral", name: "Ballerina Central", productLine: "Platform SaaS", engineerCount: 3, expertCount: 1, coverageGap: "Only 1 expert — single point of failure" },
  { id: "skl.bijira", name: "Bijira", productLine: "Platform SaaS", engineerCount: 5, expertCount: 2 },
  { id: "skl.devant", name: "Devant", productLine: "Platform SaaS", engineerCount: 4, expertCount: 1, coverageGap: "Only 1 expert" },
  { id: "skl.sp", name: "Stream Processor (legacy)", productLine: "Integration", engineerCount: 2, expertCount: 1, coverageGap: "EOL — keep for legacy customers" },
  { id: "skl.greg", name: "Governance Registry (legacy)", productLine: "Integration", engineerCount: 2, expertCount: 1, coverageGap: "EOL — keep for legacy customers" },
];

export default function CsmAdminSkillsPage(): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <AdminTabs />
      <Card variant="outlined">
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Skill</TableCell>
                <TableCell>Product line</TableCell>
                <TableCell align="right">Engineers</TableCell>
                <TableCell align="right">Experts</TableCell>
                <TableCell>Coverage</TableCell>
                <TableCell>Notes</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {SKILLS.map((s) => {
                const expertRatio = s.engineerCount === 0 ? 0 : Math.round((s.expertCount / s.engineerCount) * 100);
                return (
                  <TableRow key={s.id} hover>
                    <TableCell>{s.name}</TableCell>
                    <TableCell>
                      <Chip size="small" label={s.productLine} variant="outlined" />
                    </TableCell>
                    <TableCell align="right">{s.engineerCount}</TableCell>
                    <TableCell align="right">{s.expertCount}</TableCell>
                    <TableCell sx={{ width: 200 }}>
                      <Stack spacing={0.5}>
                        <LinearProgress
                          variant="determinate"
                          value={Math.min(100, expertRatio)}
                          color={expertRatio < 25 ? "warning" : "success"}
                        />
                        <Typography variant="caption" color="text.secondary">{expertRatio}% expert</Typography>
                      </Stack>
                    </TableCell>
                    <TableCell>
                      {s.coverageGap ? (
                        <Typography variant="caption" color="warning.main">{s.coverageGap}</Typography>
                      ) : (
                        <Typography variant="caption" color="text.secondary">—</Typography>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>
      <Typography variant="caption" color="text.secondary">
        Skill catalog drives case auto-assignment and on-call selection. Per-engineer proficiency editor lands with the skill-management endpoints.
      </Typography>
    </Box>
  );
}
