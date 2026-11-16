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
import { ListChecks } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { incidentRelatedTabPath } from "@features/csm-operations/utils/incidents";
import { useSearchIncidentTasks } from "@features/csm-operations/api/useSearchIncidentTasks";
import RefreshButton from "@components/RefreshButton";
import { useCaseRouteOverride } from "@context/case-tabs/CaseRouteOverrideContext";

const INCIDENT_TASKS_COLUMNS = ["Task", "State", "Assignment group", "Assigned to"];

interface IncidentTasksWidgetProps {
  /** UUID of the incident whose tasks are listed. */
  incidentId: string;
}

/**
 * The incident's tasks, on its Related tab. Each row opens the task's detail
 * page, with Back returning to this tab. Same card/table pattern as the case
 * page's `LinkedIncidentsListWidget`.
 */
export function IncidentTasksWidget({ incidentId }: IncidentTasksWidgetProps): JSX.Element {
  const { data, isLoading, isError, refetch, isFetching, dataUpdatedAt } =
    useSearchIncidentTasks(incidentId);
  const navigate = useNavTransition();
  const backPath = incidentRelatedTabPath(incidentId);
  // The incident page's own state (e.g. `{ from: <incidents list>, parentState }`),
  // read where that page reads it (its case tab's, when it is open in one), and
  // handed to the task as `parentState` so Back from the task keeps the
  // incident's own Back.
  const routedState: unknown = useLocation().state;
  const routeOverride = useCaseRouteOverride();
  const taskState = {
    from: backPath,
    parentState: (routeOverride ? routeOverride.state : routedState) ?? null,
  };

  const tasks = data?.tasks ?? [];
  const total = data?.total ?? tasks.length;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <ListChecks size={16} />
          <Typography variant="subtitle2">
            Incident tasks{!isLoading && !isError && total > 0 ? ` (${total})` : ""}
          </Typography>
        </Box>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh incident tasks"
        />
      </Box>

      {isError ? (
        <Typography variant="body2" color="error">
          Could not load the tasks for this incident.
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small" sx={{ width: "100%" }}>
            <TableHead>
              <TableRow>
                {INCIDENT_TASKS_COLUMNS.map((col) => (
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
                    {INCIDENT_TASKS_COLUMNS.map((col) => (
                      <TableCell key={col}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : tasks.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={INCIDENT_TASKS_COLUMNS.length} align="center">
                    <Typography variant="body2" color="text.secondary">
                      No tasks for this incident.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                tasks.map((t) => {
                  const label = `${t.number ?? t.id} — ${t.subject}`;
                  const taskPath = `/operations/incident-tasks/${encodeURIComponent(t.id)}`;
                  return (
                    <TableRow
                      key={t.id}
                      hover
                      onClick={() => navigate(taskPath, { state: taskState })}
                      sx={{ cursor: "pointer" }}
                    >
                      <TableCell sx={{ maxWidth: 0, width: "45%" }}>
                        {/* A real link, not a `role="button"` override on the
                            row — see `ChildCasesWidget`'s equivalent note. */}
                        <Typography
                          component={RouterLink}
                          to={taskPath}
                          state={taskState}
                          variant="body2"
                          noWrap
                          title={label}
                          sx={{ color: "inherit", textDecoration: "none", display: "block" }}
                        >
                          {label}
                        </Typography>
                      </TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>
                        {t.stateLabel ? <Chip size="small" label={t.stateLabel} /> : "—"}
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" noWrap title={t.assignmentGroupName}>
                          {t.assignmentGroupName ?? "—"}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Typography variant="body2" noWrap title={t.assignedToName}>
                          {t.assignedToName ?? "—"}
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

export default IncidentTasksWidget;
