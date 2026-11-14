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
import { stripStaffMark } from "@utils/staffName";
import {
  Box,
  Button,
  Card,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Link as LinkIcon, Plus } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { Link as RouterLink } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import {
  useSearchChildCases,
  type ChildCaseRow,
} from "@features/csm-cases/api/useSearchChildCases";
import StateChip from "@components/StateChip";
import RefreshButton from "@components/RefreshButton";

const LINKED_SERVICE_REQUESTS_COLUMNS = ["Case", "State", "Assignee"];

export interface LinkedServiceRequestRef {
  id: string;
  number: string;
  name: string;
}

interface LinkedServiceRequestsWidgetProps {
  /** UUID of the case whose linked service requests are listed. */
  caseId: string;
  /**
   * The authoritative set of linked service requests, straight off the case
   * detail response — always shown in full, even if a row can't be enriched
   * below (see `useSearchChildCases` cap note).
   */
  linkedServiceRequests: LinkedServiceRequestRef[] | undefined;
  onCreateServiceRequest: () => void;
  /** Disables "Create service request" once the case is closed — matches
   * the read-only rule the rest of the case detail page applies (comment
   * composer, attachment upload, tag add/remove, "Link to another case"). */
  createDisabled?: boolean;
}

/**
 * Linked service requests — same underlying relationship `ChildCasesWidget`
 * queries (`parentId` pointing at this case), just restricted to the ids the
 * case-detail response already names as service requests. Calling
 * `useSearchChildCases(caseId)` here too (rather than a bespoke query) means
 * React Query dedupes the network request against `ChildCasesWidget`, which
 * renders alongside this on the same tab.
 *
 * Only `id`/`number`/`name` are guaranteed for each linked request (that's
 * all the case-detail payload carries); state/assignee come from the shared
 * children search and are enriched in once that resolves. A ref whose id
 * isn't found there (e.g. beyond the children search's page cap) still
 * renders with its known number/name — never silently dropped, just shown
 * with "—" for the columns that couldn't be enriched.
 */
export function LinkedServiceRequestsWidget({
  caseId,
  linkedServiceRequests,
  onCreateServiceRequest,
  createDisabled = false,
}: LinkedServiceRequestsWidgetProps): JSX.Element {
  const {
    data,
    isLoading,
    isError,
    refetch,
    isFetching,
    dataUpdatedAt,
  } = useSearchChildCases(caseId);
  const navigate = useNavTransition();

  const enrichedById = useMemo(() => {
    const map = new Map<string, ChildCaseRow>();
    for (const row of data?.cases ?? []) {
      map.set(row.id, row);
    }
    return map;
  }, [data]);

  const refs = linkedServiceRequests ?? [];
  // isError means enrichment is unavailable, not pending — fall straight to
  // "—" rather than a skeleton that would spin forever.
  const enriching = isLoading && !isError;
  // So Back on the linked case's own page returns here instead of falling
  // through to that case's hardcoded list route (see CsmCaseDetailPage's
  // `resolvedBackPath`, which prefers `location.state.from` when present).
  const backPath = `/cases/${encodeURIComponent(caseId)}`;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1,
          flexWrap: "wrap",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <LinkIcon size={16} />
          <Typography variant="subtitle2">
            Linked service requests{refs.length > 0 ? ` (${refs.length})` : ""}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label="Refresh linked service requests"
          />
          <Tooltip title={createDisabled ? "This case is closed — it's read-only." : ""}>
            <Box component="span">
              <Button
                size="small"
                variant="outlined"
                startIcon={<Plus size={14} />}
                onClick={onCreateServiceRequest}
                disabled={createDisabled}
              >
                Create service request
              </Button>
            </Box>
          </Tooltip>
        </Box>
      </Box>

      <TableContainer>
        {/* Deliberately NOT table-layout:fixed — that takes each column's
            width literally, and a narrow one (e.g. state's old "1%" hack)
            just collapses instead of sizing to content, bleeding its text
            into the next column. Auto layout sizes State to its actual
            content and only the Case/Assignee `Typography` below (via
            `maxWidth` + `noWrap`) truncate. */}
        <Table size="small" sx={{ width: "100%" }}>
          <TableHead>
            <TableRow>
              <TableCell>Case</TableCell>
              <TableCell sx={{ whiteSpace: "nowrap" }}>State</TableCell>
              <TableCell>Assignee</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {refs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={LINKED_SERVICE_REQUESTS_COLUMNS.length} align="center">
                  <Typography variant="body2" color="text.secondary">
                    No service requests linked to this case.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              refs.map((sr) => {
                const enriched = enrichedById.get(sr.id);
                const caseLabel = `${sr.number} — ${sr.name}`;
                const assigneeLabel = enriched ? enriched.assigneeName ?? "—" : "—";
                const casePath = `/cases/${encodeURIComponent(sr.id)}`;
                return (
                  <TableRow
                    key={sr.id}
                    hover
                    onClick={() => navigate(casePath, { state: { from: backPath } })}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell sx={{ maxWidth: 0, width: "50%" }}>
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
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {enriched ? (
                        <StateChip state={enriched.state} />
                      ) : enriching ? (
                        <Skeleton variant="text" width={64} />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 0, width: "25%" }}>
                      {enriching ? (
                        <Skeleton variant="text" width={72} />
                      ) : (
                        <Typography variant="body2" noWrap title={stripStaffMark(assigneeLabel)}>
                          <AssigneeName name={enriched?.assigneeName} />
                        </Typography>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Card>
  );
}

export default LinkedServiceRequestsWidget;
