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
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { GitBranch } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentRelatedTabPath,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import { useSearchChildIncidents } from "@features/csm-operations/api/useSearchChildIncidents";
import RefreshButton from "@components/RefreshButton";

const CHILD_INCIDENTS_COLUMNS = ["Incident", "Priority", "State", "Assigned to"];

interface ChildIncidentsWidgetProps {
  /** UUID of the incident whose child incidents are listed. */
  parentIncidentId: string;
}

/**
 * The incidents whose parent incident is this one, on its Related tab.
 * Resolving this incident resolves the open ones, so their state is shown.
 * Each row opens the child, with Back returning to this tab. Same card/table
 * pattern as `IncidentTasksWidget`.
 */
export function ChildIncidentsWidget({ parentIncidentId }: ChildIncidentsWidgetProps): JSX.Element {
  const { data, isLoading, isError, refetch, isFetching, dataUpdatedAt } =
    useSearchChildIncidents(parentIncidentId);
  const navigate = useNavTransition();
  const backPath = incidentRelatedTabPath(parentIncidentId);

  const incidents = data?.incidents ?? [];
  const total = data?.total ?? incidents.length;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <GitBranch size={16} />
          <Typography variant="subtitle2">
            Child incidents{!isLoading && !isError && total > 0 ? ` (${total})` : ""}
          </Typography>
        </Box>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh child incidents"
        />
      </Box>

      {isError ? (
        <Typography variant="body2" color="error">
          Could not load the child incidents of this incident.
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small" sx={{ width: "100%" }}>
            <TableHead>
              <TableRow>
                {CHILD_INCIDENTS_COLUMNS.map((col) => (
                  <TableCell key={col} sx={{ whiteSpace: "nowrap" }}>
                    {col}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                [0, 1].map((i) => (
                  <TableRow key={i}>
                    {CHILD_INCIDENTS_COLUMNS.map((col) => (
                      <TableCell key={col}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : incidents.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={CHILD_INCIDENTS_COLUMNS.length} align="center">
                    <Typography variant="body2" color="text.secondary">
                      No child incidents.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                incidents.map((i) => {
                  const label = `${i.number ?? i.id} — ${i.subject}`;
                  const incidentPath = `/operations/incidents/${encodeURIComponent(i.id)}`;
                  return (
                    <TableRow
                      key={i.id}
                      hover
                      onClick={() => navigate(incidentPath, { state: { from: backPath } })}
                      sx={{ cursor: "pointer" }}
                    >
                      <TableCell sx={{ maxWidth: 0, width: "45%" }}>
                        {/* A real link, not a `role="button"` override on the
                            row — see `ChildCasesWidget`'s equivalent note. */}
                        <Typography
                          component={RouterLink}
                          to={incidentPath}
                          state={{ from: backPath }}
                          variant="body2"
                          noWrap
                          title={label}
                          sx={{ color: "inherit", textDecoration: "none", display: "block" }}
                        >
                          {label}
                        </Typography>
                      </TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={incidentPriorityColor(i.priority)}
                          label={incidentPriorityLabel(i.priority)}
                        />
                      </TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>
                        <Chip
                          size="small"
                          variant="outlined"
                          color={incidentStateColor(i.state)}
                          label={incidentStateLabel(i.state)}
                        />
                      </TableCell>
                      <TableCell sx={{ maxWidth: 220 }}>
                        <Typography variant="body2" noWrap title={i.assigneeName}>
                          {i.assigneeName ?? "—"}
                        </Typography>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Card>
  );
}

export default ChildIncidentsWidget;
