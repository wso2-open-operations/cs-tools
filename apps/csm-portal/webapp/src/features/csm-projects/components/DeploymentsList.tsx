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
  Chip,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";
import type {
  CsmDeploymentEnvironment,
  CsmDeploymentProduct,
  CsmDeploymentRow,
} from "@features/csm-projects/types/csmProjects";
import RelativeTime from "@components/RelativeTime";

interface DeploymentsListProps {
  deployments: CsmDeploymentRow[];
}

function environmentColor(
  env: CsmDeploymentEnvironment,
): "error" | "warning" | "info" | "default" {
  switch (env) {
    case "prod":
      return "error";
    case "staging":
    case "uat":
      return "warning";
    case "qa":
    case "stress":
      return "info";
    default:
      return "default";
  }
}

function supportColor(
  status: CsmDeploymentProduct["supportStatus"],
): "success" | "warning" | "error" | "default" {
  switch (status) {
    case "available":
      return "success";
    case "extended":
      return "warning";
    case "deprecated":
    case "discontinued":
      return "error";
    default:
      return "default";
  }
}

export default function DeploymentsList({
  deployments,
}: DeploymentsListProps): JSX.Element {
  if (deployments.length === 0) {
    return (
      <Paper sx={{ p: 3, display: "flex", flexDirection: "column", gap: 0.5 }}>
        <Typography variant="subtitle2">No deployments</Typography>
        <Typography variant="body2" color="text.secondary">
          No environments are registered for this project yet.
        </Typography>
      </Paper>
    );
  }

  return (
    <Paper sx={{ overflow: "hidden" }}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>Name</TableCell>
            <TableCell>Environment</TableCell>
            <TableCell>Region</TableCell>
            <TableCell>Products & versions</TableCell>
            <TableCell>Last updated</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {deployments.map((d) => (
            <TableRow key={d.id} hover>
              <TableCell sx={{ fontWeight: 500 }}>{d.name}</TableCell>
              <TableCell>
                <Chip
                  size="small"
                  color={environmentColor(d.environment)}
                  variant="outlined"
                  label={d.environment}
                />
              </TableCell>
              <TableCell>
                <Typography variant="body2" color="text.secondary">
                  {d.region}
                </Typography>
              </TableCell>
              <TableCell>
                <Box
                  sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}
                >
                  {d.products.map((p, i) => (
                    <Box
                      key={i}
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1,
                        flexWrap: "wrap",
                      }}
                    >
                      <Typography variant="body2" sx={{ fontWeight: 500 }}>
                        {p.product}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {p.version} · {p.updateLevel}
                      </Typography>
                      <Chip
                        size="small"
                        color={supportColor(p.supportStatus)}
                        variant="outlined"
                        label={p.supportStatus}
                        sx={{ height: 18, fontSize: 10 }}
                      />
                    </Box>
                  ))}
                </Box>
              </TableCell>
              <TableCell>
                <Typography variant="body2" color="text.secondary">
                  <RelativeTime iso={d.lastUpdatedAt} />
                </Typography>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Paper>
  );
}
