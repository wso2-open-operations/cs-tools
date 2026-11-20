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
  Chip,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Link,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { RefreshCw, X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useProductWorkItem } from "@features/csm-cases/api/useProductWorkItem";
import {
  productHistorySourceLabel,
  productHistoryStatusLabel,
  productIssueStatusColor,
  productIssueStatusLabel,
  productLinkStateLabel,
} from "@features/csm-cases/utils/productWorkItems";
import { formatAbsoluteForUser } from "@utils/dateTime";
import { isHttpUrl } from "@utils/isHttpUrl";

interface ProductIssueDetailDialogProps {
  workItemId: string;
  onClose: () => void;
}

/**
 * Read-only detail of one product issue: its status, the conversation and the
 * status history. Comment bodies are untrusted GitHub markdown, so they are
 * rendered as plain text (line breaks kept via `white-space: pre-wrap`) —
 * never as HTML and never through a markdown renderer.
 */
export function ProductIssueDetailDialog({
  workItemId,
  onClose,
}: ProductIssueDetailDialogProps): JSX.Element {
  const { data: item, isLoading, isError, refetch } = useProductWorkItem(workItemId);
  const url = item?.github.htmlUrl;
  const linkNote = item ? productLinkStateLabel(item.linkState) : null;
  const conversation = item?.conversation ?? [];
  const history = item?.statusHistory ?? [];

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle
        sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}
      >
        <Typography variant="subtitle1" component="span">
          Product issue{item ? ` ${item.number}` : ""}
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <X size={16} />
        </IconButton>
      </DialogTitle>
      <DialogContent dividers sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {isLoading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Skeleton variant="text" width="60%" />
            <Skeleton variant="rounded" height={80} />
          </Box>
        )}

        {isError && (
          <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1, py: 3 }}>
            <Typography variant="body2" color="error">
              Could not load this product issue.
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
        )}

        {!isLoading && !isError && item === null && (
          <Typography variant="body2" color="text.secondary">
            This product issue was not found.
          </Typography>
        )}

        {!isLoading && !isError && item && (
          <>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
              <Typography variant="subtitle2">{item.title}</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
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
                {url && isHttpUrl(url) && (
                  <Link
                    href={url}
                    target="_blank"
                    rel="noopener noreferrer"
                    variant="body2"
                    aria-label={`${item.number} on GitHub`}
                  >
                    {item.github.owner}/{item.github.repository}#{item.github.issueNumber}
                  </Link>
                )}
                {item.priority && <Chip size="small" variant="outlined" label={item.priority} />}
                {item.category && <Chip size="small" variant="outlined" label={item.category} />}
              </Box>
            </Box>

            <Divider />

            <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
              <Typography variant="subtitle2">Conversation</Typography>
              {item.conversationTruncated && (
                <Typography variant="caption" color="text.secondary">
                  Older comments are not shown.
                </Typography>
              )}
              {conversation.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No comments yet.
                </Typography>
              ) : (
                conversation.map((comment) => (
                  <Box
                    key={comment.id}
                    sx={{
                      border: 1,
                      borderColor: "divider",
                      borderRadius: 1,
                      p: 1.5,
                      display: "flex",
                      flexDirection: "column",
                      gap: 0.5,
                    }}
                  >
                    <Box sx={{ display: "flex", gap: 1, alignItems: "baseline", flexWrap: "wrap" }}>
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>
                        {comment.author}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {formatAbsoluteForUser(comment.createdOn) ?? "—"}
                        {comment.origin ? ` · ${comment.origin === "GITHUB" ? "GitHub" : "Portal"}` : ""}
                      </Typography>
                    </Box>
                    <Typography
                      variant="body2"
                      sx={{ whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}
                    >
                      {comment.body}
                    </Typography>
                  </Box>
                ))
              )}
            </Box>

            <Divider />

            <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
              <Typography variant="subtitle2">Status history</Typography>
              {history.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No status changes recorded.
                </Typography>
              ) : (
                <Box component="ul" sx={{ m: 0, pl: 2.5, display: "flex", flexDirection: "column", gap: 0.5 }}>
                  {history.map((ev) => (
                    <Typography component="li" variant="body2" key={ev.id}>
                      {ev.fromStatus ? productHistoryStatusLabel(ev.fromStatus) : "—"}
                      {" → "}
                      {productHistoryStatusLabel(ev.toStatus)}
                      <Typography component="span" variant="caption" color="text.secondary">
                        {" · "}
                        {formatAbsoluteForUser(ev.occurredOn) ?? "—"}
                        {" · "}
                        {productHistorySourceLabel(ev.source)}
                        {ev.actor ? ` · ${ev.actor}` : ""}
                      </Typography>
                    </Typography>
                  ))}
                </Box>
              )}
            </Box>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

export default ProductIssueDetailDialog;
