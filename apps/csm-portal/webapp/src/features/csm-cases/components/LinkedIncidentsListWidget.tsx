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
import { AlertTriangle } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchLinkedIncidents } from "@features/csm-cases/api/useSearchLinkedIncidents";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import RefreshButton from "@components/RefreshButton";

const LINKED_INCIDENTS_COLUMNS = ["Incident", "Priority", "State"];

interface LinkedIncidentsListWidgetProps {
  /** UUID of the case whose child incidents (SN `parent` pointing here) are
   * listed. */
  caseId: string;
}

/**
 * Incidents that are SN-side *children* of this case — incidents whose
 * `parent` field points at this case, the opposite direction from the
 * sibling {@link LinkedIncidentWidget} (which shows this case's own
 * `parentCase` when that parent is an incident). List-with-link pattern,
 * modeled on `ChildCasesWidget`; queries `POST /incidents/search {
 * filters: { parentIds } }` rather than a dedicated endpoint.
 */
export function LinkedIncidentsListWidget({
  caseId,
}: LinkedIncidentsListWidgetProps): JSX.Element {
  const {
    data,
    isLoading,
    isError,
    refetch,
    isFetching,
    dataUpdatedAt,
  } = useSearchLinkedIncidents(caseId);
  const navigate = useNavTransition();

  const incidents = data?.incidents ?? [];
  const total = data?.total ?? incidents.length;
  // So Back on the incident's own page returns here instead of falling
  // through to its hardcoded list route — same pattern as `ChildCasesWidget`.
  const backPath = `/cases/${encodeURIComponent(caseId)}`;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <AlertTriangle size={16} />
          <Typography variant="subtitle2">
            Linked incidents{!isLoading && !isError && total > 0 ? ` (${total})` : ""}
          </Typography>
        </Box>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh linked incidents"
        />
      </Box>

      {isError ? (
        <Typography variant="body2" color="error">
          Could not load linked incidents for this case.
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small" sx={{ width: "100%" }}>
            <TableHead>
              <TableRow>
                <TableCell>Incident</TableCell>
                <TableCell>Priority</TableCell>
                <TableCell sx={{ whiteSpace: "nowrap" }}>State</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                [0, 1].map((i) => (
                  <TableRow key={i}>
                    {LINKED_INCIDENTS_COLUMNS.map((col) => (
                      <TableCell key={col}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : incidents.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={LINKED_INCIDENTS_COLUMNS.length} align="center">
                    <Typography variant="body2" color="text.secondary">
                      No incidents linked to this case.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                incidents.map((i) => {
                  const incidentLabel = `${i.number ?? i.id} — ${i.subject}`;
                  const incidentPath = `/operations/incidents/${encodeURIComponent(i.id)}`;
                  return (
                    <TableRow
                      key={i.id}
                      hover
                      onClick={() =>
                        navigate(incidentPath, { state: { from: backPath } })
                      }
                      sx={{ cursor: "pointer" }}
                    >
                      <TableCell sx={{ maxWidth: 0, width: "55%" }}>
                        {/* A real link, not a `role="button"` override on the
                            row — see `ChildCasesWidget`'s equivalent note. */}
                        <Typography
                          component={RouterLink}
                          to={incidentPath}
                          state={{ from: backPath }}
                          variant="body2"
                          noWrap
                          title={incidentLabel}
                          sx={{ color: "inherit", textDecoration: "none", display: "block" }}
                        >
                          {incidentLabel}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip
                          size="small"
                          color={incidentPriorityColor(i.priority)}
                          label={incidentPriorityLabel(i.priority)}
                        />
                      </TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>
                        <Chip
                          size="small"
                          color={incidentStateColor(i.state)}
                          label={incidentStateLabel(i.state)}
                        />
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

export default LinkedIncidentsListWidget;
