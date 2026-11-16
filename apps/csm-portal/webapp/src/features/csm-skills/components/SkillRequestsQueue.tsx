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
  Alert,
  Box,
  Button,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import RelativeTime from "@components/RelativeTime";
import { useSkillRequestQueue } from "@features/csm-skills/api/useSkillManagement";
import LevelChip from "@features/csm-skills/components/LevelChip";
import ReviewSkillRequestDialog from "@features/csm-skills/components/ReviewSkillRequestDialog";
import { personName, REQUEST_STATUS_COLOR, REQUEST_STATUS_LABEL } from "@features/csm-skills/utils/skillRequests";
import type { SkillRequest, SkillRequestStatus } from "@features/csm-skills/types";

const ROWS_PER_PAGE_OPTIONS = [10, 25, 50];
const VIEWS: Record<"pending" | "decided", SkillRequestStatus[]> = {
  pending: ["PENDING"],
  decided: ["APPROVED", "REJECTED", "WITHDRAWN"],
};

/**
 * Administration › Requests: the skill request queue. Pending requests come
 * oldest first; each opens the review dialog. Decided ones show the outcome.
 */
export default function SkillRequestsQueue(): JSX.Element {
  const [view, setView] = useState<"pending" | "decided">("pending");
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(ROWS_PER_PAGE_OPTIONS[1]);
  const [reviewing, setReviewing] = useState<SkillRequest | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const result = useSkillRequestQueue({ statuses: VIEWS[view], limit: rowsPerPage, offset: page * rowsPerPage });
  const rows = result.data?.requests ?? [];

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={view}
          aria-label="Which requests"
          onChange={(_, v: "pending" | "decided" | null) => {
            if (v) {
              setView(v);
              setPage(0);
            }
          }}
        >
          <ToggleButton value="pending">Pending</ToggleButton>
          <ToggleButton value="decided">Decided</ToggleButton>
        </ToggleButtonGroup>
        <Box sx={{ flexGrow: 1 }} />
        <RefreshButton
          onRefresh={() => void result.refetch()}
          isFetching={result.isFetching}
          updatedAt={result.dataUpdatedAt}
          label="Refresh skill requests"
        />
      </Box>

      {notice && (
        <Alert severity="success" role="status" onClose={() => setNotice(null)}>
          {notice}
        </Alert>
      )}

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" aria-label="Skill requests" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell>Requested skill</TableCell>
                <TableCell>Category</TableCell>
                <TableCell>Requested by</TableCell>
                <TableCell>Level</TableCell>
                <TableCell>{view === "pending" ? "Waiting since" : "Decided"}</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody sx={result.isFetching && !result.isLoading ? { opacity: 0.6 } : undefined}>
              {result.isLoading ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell colSpan={7}>
                      <Skeleton variant="rounded" height={18} />
                    </TableCell>
                  </TableRow>
                ))
              ) : result.isError ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <QueryErrorState message={result.error.message || "Failed to load skill requests."} error={result.error} onRetry={() => void result.refetch()} />
                  </TableCell>
                </TableRow>
              ) : rows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      {view === "pending" ? "No requests are waiting for review." : "No decided requests yet."}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                rows.map((r) => (
                  <TableRow key={r.id} hover>
                    <TableCell sx={{ maxWidth: 280 }}>
                      <Typography variant="body2">{r.requestedName}</Typography>
                      {r.description && (
                        <Typography variant="caption" color="text.secondary" noWrap component="div">
                          {r.description}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 220 }}>
                      {r.category?.categoryPath ?? r.category?.name ?? (r.suggestedCategory ? `Suggested: ${r.suggestedCategory}` : "—")}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 220 }}>{personName(r.requestedBy)}</TableCell>
                    <TableCell>
                      <LevelChip level={r.requestedLevel} />
                    </TableCell>
                    <TableCell>
                      <RelativeTime iso={view === "pending" ? r.createdOn : (r.reviewedOn ?? r.createdOn)} />
                    </TableCell>
                    <TableCell>
                      <Chip size="small" variant="outlined" color={REQUEST_STATUS_COLOR[r.status]} label={REQUEST_STATUS_LABEL[r.status]} />
                      {r.status !== "PENDING" && r.reviewedBy && (
                        <Typography variant="caption" color="text.secondary" component="div">
                          by {personName(r.reviewedBy)}
                          {r.resolvedSkill ? ` → ${r.resolvedSkill.name}` : ""}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell align="right">
                      {r.status === "PENDING" && (
                        <Button size="small" variant="outlined" onClick={() => setReviewing(r)} aria-label={`Review the request for ${r.requestedName}`}>
                          Review
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={result.data?.total ?? 0}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => {
            setRowsPerPage(parseInt(e.target.value, 10));
            setPage(0);
          }}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>

      {reviewing && (
        <ReviewSkillRequestDialog
          request={reviewing}
          onClose={(done) => {
            setReviewing(null);
            if (done) setNotice(done);
          }}
        />
      )}
    </Box>
  );
}
