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
import { GitPullRequest } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useNavTransition } from "@hooks/useNavTransition";
import { useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";
import type { BeChangeRequestDetail } from "@api/backend/types";
import {
  changeRequestStateColor,
  changeRequestStateLabel,
} from "@features/csm-operations/utils/changeRequests";
import RefreshButton from "@components/RefreshButton";

const LINKED_CHANGE_REQUESTS_COLUMNS = ["Change request", "State", "Target environment"];

interface LinkedChangeRequestRef {
  id: string;
  number: string;
  /** Subject, or `null` when the record has none — never `""`. The row label
   * filters falsy parts out, so both render as just the number. */
  name: string | null;
}

interface LinkedChangeRequestsWidgetProps {
  /** The change requests raised from this service request, from the case
   * detail response's `linkedChangeRequests` (id/number/name only). */
  changeRequests: LinkedChangeRequestRef[] | undefined;
}

/**
 * Change requests raised from this service request — the reverse of the
 * change request's own "originating service request" link. One-to-many:
 * promoting the same change through multiple environments (dev, pre-prod,
 * production) produces one change request per environment, all pointing back
 * at this service request, so this always renders as a list.
 *
 * `linkedChangeRequests` only carries id/number/name; state and target
 * environment are fetched per row via `GET /change-requests/{id}` (fanned out
 * with `useQueries`, mirroring `useResolvedInlineImageHtml`), so each row
 * renders its number immediately and fills in the rest as its own fetch
 * resolves — a slow or failed row never blocks or hides the others.
 */
export function LinkedChangeRequestsWidget({
  changeRequests,
}: LinkedChangeRequestsWidgetProps): JSX.Element {
  const api = useBackendApi();
  const navigate = useNavTransition();
  const queryClient = useQueryClient();
  const refs = changeRequests ?? [];

  const queries = useQueries({
    queries: refs.map((ref) => ({
      // Same query key useGetChangeRequest uses, so a row here and a visit to
      // the change request's own detail page share one cache entry.
      queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, ref.id],
      queryFn: (): Promise<BeChangeRequestDetail | null> =>
        api.get<BeChangeRequestDetail>(
          `/change-requests/${encodeURIComponent(ref.id)}`,
        ),
      staleTime: 30_000,
    })),
  });
  const isFetching = queries.some((q) => q.isFetching);
  const refreshRows = (): void => {
    for (const ref of refs) {
      void queryClient.invalidateQueries({
        queryKey: [ApiQueryKeys.CHANGE_REQUEST_DETAILS, ref.id],
      });
    }
  };

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
          <GitPullRequest size={16} />
          <Typography variant="subtitle2">Linked change requests</Typography>
          <Chip size="small" variant="outlined" label={`${refs.length} total`} />
        </Box>
        <RefreshButton
          onRefresh={refreshRows}
          isFetching={isFetching}
          label="Refresh linked change requests"
        />
      </Box>

      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              {LINKED_CHANGE_REQUESTS_COLUMNS.map((col) => (
                <TableCell key={col}>{col}</TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {refs.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={LINKED_CHANGE_REQUESTS_COLUMNS.length}
                  align="center"
                  sx={{ py: 3 }}
                >
                  <Typography variant="body2" color="text.secondary">
                    No change requests have been raised from this service
                    request yet.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              refs.map((ref, i) => {
                const detailQuery = queries[i];
                const detail = detailQuery?.data;
                // A failed or still-in-flight row falls back to the
                // placeholder/dash below rather than dropping the row or
                // blanking the whole widget.
                const isRowLoading = detailQuery?.isLoading ?? false;

                return (
                  <TableRow
                    key={ref.id}
                    hover
                    onClick={() =>
                      navigate(`/operations/change-requests/${encodeURIComponent(ref.id)}`)
                    }
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        navigate(
                          `/operations/change-requests/${encodeURIComponent(ref.id)}`,
                        );
                      }
                    }}
                    tabIndex={0}
                    role="button"
                    aria-label={`View change request ${ref.number}`}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell sx={{ maxWidth: 360 }}>
                      {/* `name` is an empty string on records with no subject,
                       * so it can't be joined unconditionally. */}
                      <Typography
                        variant="body2"
                        noWrap
                        title={[ref.number, ref.name].filter(Boolean).join(" — ")}
                      >
                        {[ref.number, ref.name].filter(Boolean).join(" — ")}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      {isRowLoading ? (
                        <Skeleton variant="text" width={80} />
                      ) : detail ? (
                        <Chip
                          size="small"
                          color={changeRequestStateColor(detail.state)}
                          label={changeRequestStateLabel(detail.state)}
                        />
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell>
                      {isRowLoading ? (
                        <Skeleton variant="text" width={80} />
                      ) : (
                        (detail?.deployment?.name ?? "—")
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

export default LinkedChangeRequestsWidget;
