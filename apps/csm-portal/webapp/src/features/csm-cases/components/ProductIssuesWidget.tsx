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
  Link,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Button,
} from "@wso2/oxygen-ui";
import { GitBranch, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import RefreshButton from "@components/RefreshButton";
import RelativeTime from "@components/RelativeTime";
import { isHttpUrl } from "@utils/isHttpUrl";
import { useCaseProductWorkItems } from "@features/csm-cases/api/useCaseProductWorkItems";
import { ProductIssueDetailDialog } from "@features/csm-cases/components/ProductIssueDetailDialog";
import {
  productIssueStatusColor,
  productIssueStatusLabel,
  productLinkStateLabel,
} from "@features/csm-cases/utils/productWorkItems";

const PRODUCT_ISSUES_COLUMNS = ["Issue", "Title", "Status", "Last activity"];

interface ProductIssuesWidgetProps {
  /** UUID of the case whose linked product issues are listed. */
  caseId: string;
}

/**
 * Read-only list of the product issues (GitHub issues raised from the case,
 * tracked as work items) linked to this case; a row opens
 * {@link ProductIssueDetailDialog}. The backing routes exist only when the
 * backend's `GIT_INTERNALS_ENABLED` is on. With it off they answer 404 (the
 * hook yields `null`), and this widget renders nothing at all — no frame, no
 * error — so the case page is unchanged. A 403 (not an internal caller) is
 * treated the same way; any other failure shows an error state.
 */
export function ProductIssuesWidget({ caseId }: ProductIssuesWidgetProps): JSX.Element | null {
  const { data, isLoading, isError, error, refetch, isFetching, dataUpdatedAt } =
    useCaseProductWorkItems(caseId);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  if (data === null) return null;
  if (isError && (error as { status?: number } | null)?.status === 403) return null;

  const items = data ?? [];

  return (
    <>
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
            <GitBranch size={16} />
            <Typography variant="subtitle2">
              Product issues{items.length > 0 ? ` (${items.length})` : ""}
            </Typography>
          </Box>
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label="Refresh product issues"
          />
        </Box>

        {isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Skeleton variant="text" width="60%" />
            <Skeleton variant="rounded" height={48} />
          </Box>
        ) : isError ? (
          <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1, py: 2 }}>
            <Typography variant="body2" color="error">
              Could not load product issues.
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
        ) : (
          <TableContainer>
            <Table size="small" sx={{ width: "100%" }}>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ whiteSpace: "nowrap" }}>Issue</TableCell>
                  <TableCell>Title</TableCell>
                  <TableCell sx={{ whiteSpace: "nowrap" }}>Status</TableCell>
                  <TableCell sx={{ whiteSpace: "nowrap" }}>Last activity</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {items.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={PRODUCT_ISSUES_COLUMNS.length} align="center">
                      <Typography variant="body2" color="text.secondary">
                        No product issues linked to this case.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  items.map((item) => {
                    const linkNote = productLinkStateLabel(item.linkState);
                    const url = item.github.htmlUrl;
                    return (
                      <TableRow
                        key={item.id}
                        hover
                        onClick={() => setSelectedId(item.id)}
                        onKeyDown={(e) => {
                          if (e.target !== e.currentTarget) return;
                          if (e.key === "Enter" || e.key === " ") {
                            e.preventDefault();
                            setSelectedId(item.id);
                          }
                        }}
                        tabIndex={0}
                        role="button"
                        aria-label={`View product issue ${item.number}`}
                        sx={{
                          cursor: "pointer",
                          "&:focus-visible": {
                            outline: "2px solid",
                            outlineColor: "primary.main",
                            outlineOffset: -2,
                          },
                        }}
                      >
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {url && isHttpUrl(url) ? (
                            <Link
                              href={url}
                              target="_blank"
                              rel="noopener noreferrer"
                              onClick={(e) => e.stopPropagation()}
                              variant="body2"
                              aria-label={`${item.number} on GitHub`}
                            >
                              {item.number}
                            </Link>
                          ) : (
                            <Typography variant="body2">{item.number}</Typography>
                          )}
                        </TableCell>
                        <TableCell sx={{ maxWidth: 0, width: "50%" }}>
                          <Typography variant="body2" noWrap title={item.title}>
                            {item.title}
                          </Typography>
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, alignItems: "flex-start" }}>
                            <Chip
                              size="small"
                              variant="outlined"
                              color={productIssueStatusColor(item)}
                              label={productIssueStatusLabel(item)}
                            />
                            {linkNote && (
                              <Typography variant="caption" color="text.secondary">
                                {linkNote}
                              </Typography>
                            )}
                          </Box>
                        </TableCell>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {item.lastActivityOn ? <RelativeTime iso={item.lastActivityOn} /> : "—"}
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

      {selectedId && (
        <ProductIssueDetailDialog workItemId={selectedId} onClose={() => setSelectedId(null)} />
      )}
    </>
  );
}

export default ProductIssuesWidget;
