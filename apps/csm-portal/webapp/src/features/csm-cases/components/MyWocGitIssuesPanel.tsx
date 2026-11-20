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
  Button,
  Card,
  Link,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@wso2/oxygen-ui";
import { GitBranch, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { Link as RouterLink } from "react-router";
import RefreshButton from "@components/RefreshButton";
import RelativeTime from "@components/RelativeTime";
import { isHttpUrl } from "@utils/isHttpUrl";
import { formatAbsoluteForUser } from "@utils/dateTime";
import { useMyWocProductIssues } from "@features/csm-cases/api/useMyWocProductIssues";
import {
  NonStandardColumnHint,
  ProductIssueSlaChip,
} from "@features/csm-cases/components/ProductIssueSlaStanding";

const COLUMNS = ["Issue", "Title", "Case", "Waiting since", "SLA", "Last activity"];

/**
 * "My WOC Git issues": the product issues waiting on CS that the signed-in user
 * opened or whose case is assigned to them, oldest wait first. Shown above the
 * Support cases list. Renders nothing when the product-work-items routes answer
 * 404 (feature off) or 403 (not an internal caller), like the case page's
 * Product issues card.
 */
export function MyWocGitIssuesPanel(): JSX.Element | null {
  const { data, isLoading, isError, refetch, isFetching, dataUpdatedAt } = useMyWocProductIssues();

  if (data === null) return null;

  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  return (
    <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
          <GitBranch size={16} />
          <Typography variant="subtitle2">
            My WOC Git issues{total > 0 ? ` (${total})` : ""}
          </Typography>
        </Box>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh my WOC Git issues"
        />
      </Box>

      {isLoading ? (
        <Skeleton variant="rounded" height={48} />
      ) : isError ? (
        <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1, py: 2 }}>
          <Typography variant="body2" color="error">
            Could not load your Git issues.
          </Typography>
          <Button
            size="small"
            variant="outlined"
            startIcon={<RefreshCw size={14} />}
            onClick={() => void refetch()}
            sx={{ textTransform: "none" }}
          >
            Retry
          </Button>
        </Box>
      ) : items.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No Git issues are waiting on you.
        </Typography>
      ) : (
        <TableContainer>
          <Table size="small" sx={{ width: "100%" }}>
            <TableHead>
              <TableRow>
                {COLUMNS.map((c) => (
                  <TableCell key={c} sx={{ whiteSpace: "nowrap" }}>
                    {c}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {items.map((item) => {
                const url = item.github.htmlUrl;
                const since = item.sla?.pausedSince;
                return (
                  <TableRow key={item.id} hover>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {url && isHttpUrl(url) ? (
                        <Link
                          href={url}
                          target="_blank"
                          rel="noopener noreferrer"
                          variant="body2"
                          aria-label={`${item.number} on GitHub`}
                        >
                          {item.number}
                        </Link>
                      ) : (
                        <Typography variant="body2">{item.number}</Typography>
                      )}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 0, width: "40%" }}>
                      <Typography variant="body2" noWrap title={item.title}>
                        {item.title}
                      </Typography>
                      <NonStandardColumnHint item={item} />
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {item.case ? (
                        <Link
                          component={RouterLink}
                          to={`/cases/${encodeURIComponent(item.case.id)}`}
                          variant="body2"
                          aria-label={`Case ${item.case.number}`}
                        >
                          {item.case.number}
                        </Link>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {since ? (formatAbsoluteForUser(since) ?? "—") : "—"}
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      <ProductIssueSlaChip sla={item.sla} />
                    </TableCell>
                    <TableCell sx={{ whiteSpace: "nowrap" }}>
                      {item.lastActivityOn ? <RelativeTime iso={item.lastActivityOn} /> : "—"}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Card>
  );
}

export default MyWocGitIssuesPanel;
