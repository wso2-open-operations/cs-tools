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
import { GitFork } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavTransition } from "@hooks/useNavTransition";
import { useSearchChildCases } from "@features/csm-cases/api/useSearchChildCases";
import SeverityChip from "@components/SeverityChip";
import StateChip from "@components/StateChip";

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
  const { data, isLoading, isError } = useSearchChildCases(caseId);
  const navigate = useNavTransition();

  const cases = data?.cases ?? [];
  const total = data?.total ?? cases.length;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
        <GitFork size={16} />
        <Typography variant="subtitle2">Child cases</Typography>
        {!isLoading && !isError && (
          <Chip size="small" variant="outlined" label={`${total} total`} />
        )}
      </Box>

      {isError ? (
        <Typography variant="body2" color="error">
          Could not load child cases for this case.
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow>
                {CHILD_CASES_COLUMNS.map((col) => (
                  <TableCell key={col}>{col}</TableCell>
                ))}
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
                  <TableCell
                    colSpan={CHILD_CASES_COLUMNS.length}
                    align="center"
                    sx={{ py: 3 }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      No child cases linked to this case.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                cases.map((c) => (
                  <TableRow
                    key={c.id}
                    hover
                    onClick={() => navigate(`/cases/${encodeURIComponent(c.id)}`)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        navigate(`/cases/${encodeURIComponent(c.id)}`);
                      }
                    }}
                    tabIndex={0}
                    role="button"
                    aria-label={`View case ${c.caseNumber ?? c.id}`}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>{c.caseNumber ?? c.id} — {c.subject}</TableCell>
                    <TableCell>
                      <SeverityChip severity={c.severity} />
                    </TableCell>
                    <TableCell>
                      <StateChip state={c.state} />
                    </TableCell>
                    <TableCell>{c.assigneeName ?? "—"}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Card>
  );
}

export default ChildCasesWidget;
