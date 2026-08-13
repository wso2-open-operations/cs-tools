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
  IconButton,
  Skeleton,
  TablePagination,
  Tooltip,
  Typography,
  useTheme,
} from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { Link as RouterLink, useLocation } from "react-router";
import ConversationStateChip from "@components/ConversationStateChip";
import RelativeTime from "@components/RelativeTime";
import UserRefLink from "@components/UserRefLink";
import QueryErrorState from "@components/QueryErrorState";
import ConversationPreviewDrawer from "@features/csm-projects/components/ConversationPreviewDrawer";
import ConversationsFilterBar from "@features/csm-projects/components/ConversationsFilterBar";
import { useSearchConversations } from "@features/csm-projects/api/useSearchConversations";
import { DEFAULT_CONVERSATION_FILTERS, type ConversationsFilters } from "@features/csm-projects/utils/conversationState";
import { useNavTransition } from "@hooks/useNavTransition";
import type { BeConversationView } from "@api/backend/types";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, DEFAULT_ROWS_PER_PAGE, 50];

const HEADER_CELLS = ["Number", "Initiator", "Messages", "State", "Created"];
// Trailing `auto` track (unlabeled in the header, like CasesList's own
// quick-preview column) holds the per-row Preview action.
const GRID_TEMPLATE_COLUMNS =
  "minmax(120px, 1fr) minmax(160px, 1.5fr) minmax(90px, 0.6fr) minmax(110px, 0.8fr) minmax(120px, 1fr) auto";

interface ConversationsTabProps {
  projectId: string;
}

/**
 * Lists a project's chat sessions (`POST /conversations/search`), most
 * recently active first, with a filter bar (state / search / "my
 * conversations") and a quick-preview drawer — mirroring `CasesList`'s
 * CSS-grid row pattern: the whole row navigates to the conversation's
 * dedicated page (`/conversations/:id`), while a trailing eye-icon column
 * opens `ConversationPreviewDrawer` without navigating (`stopPropagation`).
 */
export default function ConversationsTab({ projectId }: ConversationsTabProps): JSX.Element {
  const theme = useTheme();
  const location = useLocation();
  const navigate = useNavTransition();

  // Guarded set during render (React's documented pattern for adjusting state
  // from a changed prop, not an effect) — resets pagination/filters/preview
  // when the surrounding page switches to a different project's Work items
  // tab, rather than carrying over the previous project's state into this one.
  const [previousProjectId, setPreviousProjectId] = useState(projectId);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [filters, setFilters] = useState<ConversationsFilters>(DEFAULT_CONVERSATION_FILTERS);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [previewRow, setPreviewRow] = useState<BeConversationView | null>(null);

  if (projectId !== previousProjectId) {
    setPreviousProjectId(projectId);
    setPage(0);
    setFilters(DEFAULT_CONVERSATION_FILTERS);
    setPreviewRow(null);
  }

  const { data, isLoading, isError, error } = useSearchConversations(
    projectId,
    { page, rowsPerPage },
    {
      states: filters.states,
      searchQuery: filters.search,
      createdByMe: filters.createdByMe,
      number: filters.number,
      createdBy: filters.createdBy,
    },
  );
  const conversations = data?.conversations ?? [];
  const total = data?.total ?? 0;

  const handleFiltersChange = (next: ConversationsFilters): void => {
    setFilters(next);
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <ConversationsFilterBar
        filters={filters}
        onChange={handleFiltersChange}
        onReset={() => handleFiltersChange(DEFAULT_CONVERSATION_FILTERS)}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen((v) => !v)}
      />

      <Box
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          overflow: "hidden",
          display: "grid",
          gridTemplateColumns: GRID_TEMPLATE_COLUMNS,
          columnGap: 2,
        }}
      >
        {/* Header */}
        <Box
          sx={{
            gridColumn: "1 / -1",
            display: "grid",
            gridTemplateColumns: "subgrid",
            columnGap: 2,
            alignItems: "center",
            px: 2,
            py: 1.25,
            bgcolor: "action.hover",
            borderBottom: 1,
            borderColor: "divider",
          }}
        >
          {HEADER_CELLS.map((label) => (
            <Typography
              key={label}
              variant="caption"
              color="text.secondary"
              sx={{ fontWeight: 600, textAlign: "left" }}
            >
              {label}
            </Typography>
          ))}
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600, textAlign: "left" }}>
            Preview
          </Typography>
        </Box>

        {/* Rows */}
        {isLoading &&
          Array.from({ length: 3 }).map((_, i) => (
            <Box
              key={i}
              sx={{
                gridColumn: "1 / -1",
                px: 2,
                py: 1.25,
                borderBottom: 1,
                borderColor: "divider",
                "&:last-of-type": { borderBottom: 0 },
              }}
            >
              <Skeleton variant="rounded" height={28} />
            </Box>
          ))}

        {!isLoading && isError && (
          <Box sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
            <QueryErrorState
              message={
                error instanceof Error && error.message.trim()
                  ? error.message
                  : "Failed to load conversations."
              }
              error={error}
            />
          </Box>
        )}

        {!isLoading && !isError && conversations.length === 0 && (
          <Box sx={{ gridColumn: "1 / -1", px: 2, py: 4, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              No chat sessions found for this project.
            </Typography>
          </Box>
        )}

        {!isLoading &&
          !isError &&
          conversations.map((c, i) => {
            const rowKey = c.id ?? `${c.createdOn}-${i}`;
            const detailPath = c.id ? `/conversations/${c.id}` : undefined;
            const initiator = c.createdBy;
            const rowLabel = c.number || "this chat session";

            return (
              <Box
                key={rowKey}
                onClick={() => {
                  if (!detailPath) return;
                  navigate(detailPath, {
                    state: { conversation: c, from: `${location.pathname}${location.search}` },
                  });
                }}
                sx={{
                  gridColumn: "1 / -1",
                  display: "grid",
                  gridTemplateColumns: "subgrid",
                  columnGap: 2,
                  alignItems: "center",
                  px: 2,
                  py: 1.25,
                  borderBottom: 1,
                  borderColor: "divider",
                  cursor: detailPath ? "pointer" : "default",
                  "&:hover": detailPath ? { bgcolor: "action.hover" } : undefined,
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                {detailPath ? (
                  <Box
                    component={RouterLink}
                    to={detailPath}
                    state={{ conversation: c, from: `${location.pathname}${location.search}` }}
                    aria-label={`Open ${rowLabel}`}
                    onClick={(e) => e.stopPropagation()}
                    sx={{
                      minWidth: 0,
                      color: "inherit",
                      textDecoration: "none",
                      display: "block",
                      "&:focus-visible": {
                        outline: `2px solid ${theme.palette.primary.main}`,
                        outlineOffset: 2,
                        borderRadius: 0.5,
                      },
                    }}
                  >
                    <Typography
                      variant="body2"
                      noWrap
                      title={c.number || undefined}
                      sx={{ fontFamily: "monospace", fontWeight: 600 }}
                    >
                      {c.number || "—"}
                    </Typography>
                  </Box>
                ) : (
                  <Typography variant="body2" noWrap sx={{ fontFamily: "monospace", fontWeight: 600 }}>
                    {c.number || "—"}
                  </Typography>
                )}
                <Box sx={{ minWidth: 0 }}>
                  {initiator ? (
                    <UserRefLink
                      name={initiator.name || initiator.email || "—"}
                      email={initiator.email}
                      userId={initiator.id}
                      underlineOnHover={false}
                    />
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      —
                    </Typography>
                  )}
                </Box>
                <Typography variant="body2">{c.messageCount}</Typography>
                <Box sx={{ justifySelf: "start" }}>
                  {c.state ? (
                    <ConversationStateChip state={c.state} variant="outlined" />
                  ) : (
                    <Typography variant="body2" color="text.secondary">
                      —
                    </Typography>
                  )}
                </Box>
                <Typography variant="caption" color="text.secondary" noWrap>
                  <RelativeTime iso={c.createdOn} />
                </Typography>
                {/* Quick preview. `stopPropagation` keeps the click from also
                    bubbling up into the row's own onClick (which would
                    navigate to the full page instead of just previewing). */}
                <Tooltip title={`Quick preview ${rowLabel}`}>
                  <IconButton
                    size="small"
                    aria-label={`Quick preview ${rowLabel}`}
                    onClick={(e) => {
                      e.stopPropagation();
                      setPreviewRow(c);
                    }}
                    sx={{ justifySelf: "center" }}
                  >
                    <Eye size={16} />
                  </IconButton>
                </Tooltip>
              </Box>
            );
          })}
      </Box>

      <TablePagination
        component="div"
        count={total}
        page={page}
        onPageChange={(_, newPage) => setPage(newPage)}
        rowsPerPage={rowsPerPage}
        onRowsPerPageChange={(e) => {
          setRowsPerPage(parseInt(e.target.value, 10));
          setPage(0);
        }}
        rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
        labelRowsPerPage="Conversations per page"
      />

      <ConversationPreviewDrawer conversation={previewRow} onClose={() => setPreviewRow(null)} />
    </Box>
  );
}
