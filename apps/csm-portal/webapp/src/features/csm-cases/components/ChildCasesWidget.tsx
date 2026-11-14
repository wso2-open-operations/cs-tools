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

import AssigneeName from "@components/AssigneeName";
import {
  Box,
  Card,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { GitFork } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchChildCases } from "@features/csm-cases/api/useSearchChildCases";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";
import RefreshButton from "@components/RefreshButton";

const CHILD_CASES_COLUMNS = ["Case", "Severity", "State", "Assignee"];

interface ChildCasesWidgetProps {
  /** UUID of the case whose children (`parentId` pointing here) are listed. */
  caseId: string;
}

/**
 * Child cases of this case — cases whose `parentId` points here (the
 * hierarchical major-case/child-case relationship). List-with-link pattern,
 * modeled on {@link TasksWidget}; queries the existing cross-project search
 * (`POST /cases/search { filters: { parentId } }`) rather than a dedicated
 * endpoint.
 */
export function ChildCasesWidget({ caseId }: ChildCasesWidgetProps): JSX.Element {
  const {
    data,
    isLoading,
    isError,
    refetch,
    isFetching,
    dataUpdatedAt,
  } = useSearchChildCases(caseId);
  const navigate = useNavTransition();

  const cases = data?.cases ?? [];
  const total = data?.total ?? cases.length;
  // So Back on the child case's own page returns here instead of falling
  // through to that case's hardcoded list route (see CsmCaseDetailPage's
  // `resolvedBackPath`, which prefers `location.state.from` when present).
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
          <GitFork size={16} />
          <Typography variant="subtitle2">
            Child cases{!isLoading && !isError && total > 0 ? ` (${total})` : ""}
          </Typography>
        </Box>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh child cases"
        />
      </Box>

      {isError ? (
        <Typography variant="body2" color="error">
          Could not load child cases for this case.
        </Typography>
      ) : (
        <TableContainer>
          {/* Deliberately NOT table-layout:fixed — that takes each column's
              width literally, and a narrow one (e.g. state's old "1%" hack)
              just collapses instead of sizing to content, bleeding its text
              into the next column. Auto layout sizes Severity/State/Assignee
              to their actual content and only the Case/Assignee `Typography`
              below (via `maxWidth` + `noWrap`) truncate. */}
          <Table size="small" sx={{ width: "100%" }}>
            <TableHead>
              <TableRow>
                <TableCell>Case</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell sx={{ whiteSpace: "nowrap" }}>State</TableCell>
                <TableCell>Assignee</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading ? (
                [0, 1].map((i) => (
                  <TableRow key={i}>
                    {CHILD_CASES_COLUMNS.map((col) => (
                      <TableCell key={col}>
                        <Skeleton variant="text" />
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : cases.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={CHILD_CASES_COLUMNS.length} align="center">
                    <Typography variant="body2" color="text.secondary">
                      No child cases linked to this case.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                cases.map((c) => {
                  const caseLabel = `${c.caseNumber ?? c.id} — ${c.subject}`;
                  const casePath = `/cases/${encodeURIComponent(c.id)}`;
                  return (
                    <TableRow
                      key={c.id}
                      hover
                      onClick={() => navigate(casePath, { state: { from: backPath } })}
                      sx={{ cursor: "pointer" }}
                    >
                      <TableCell sx={{ maxWidth: 0, width: "40%" }}>
                        {/* A real link, not a `role="button"` override on the
                            row — that strips the row's implicit ARIA role,
                            leaving its cells with no valid parent role. The
                            row's own onClick above is just a mouse
                            convenience on top of this. */}
                        <Typography
                          component={RouterLink}
                          to={casePath}
                          state={{ from: backPath }}
                          variant="body2"
                          noWrap
                          title={caseLabel}
                          sx={{ color: "inherit", textDecoration: "none", display: "block" }}
                        >
                          {caseLabel}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <SeverityChip severity={c.severity} />
                      </TableCell>
                      <TableCell sx={{ whiteSpace: "nowrap" }}>
                        <StateChip state={c.state} />
                      </TableCell>
                      <TableCell sx={{ maxWidth: 0, width: "25%" }}>
                        <Typography variant="body2" noWrap title={c.assigneeName ?? "—"}>
                          <AssigneeName name={c.assigneeName} />
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

export default ChildCasesWidget;
