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

import { useMemo, useState, type JSX } from "react";
import {
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Skeleton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ClipboardCheck, Clock, Eye, Pencil, Plus, Trash2 } from "@wso2/oxygen-ui-icons-react";
import RelativeDate from "@components/RelativeDate";
import {
  useCaseTimeCards,
  useDecideTimeCard,
  useDeleteTimeCard,
} from "@features/csm-timecards/api/useTimeCards";
import { useCurrentEngineer } from "@features/csm-timecards/api/useTimeSheets";
import { useIsTeamLead } from "@features/csm-timecards/hooks/useIsTeamLead";
import { billableLabel } from "@features/csm-timecards/constants/timeCardConstants";
import { decisionSummary } from "@features/csm-timecards/utils/timeCardDecision";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import TimeCardDetailsDialog from "@features/csm-timecards/components/TimeCardDetailsDialog";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import TimeCardReviewDialog from "@features/csm-timecards/components/TimeCardReviewDialog";
import TimeCardTruncatedNotice from "@features/csm-timecards/components/TimeCardTruncatedNotice";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";
import RefreshButton from "@components/RefreshButton";

interface CaseTimeCardsPanelProps {
  caseId: string;
  /** Opens the log-time dialog (owned by the page so the action bar can trigger it). */
  onLogTime: () => void;
  /** Opens the edit dialog for one of this panel's own cards (owned by the
   * page, same as `onLogTime` — both open the same `LogTimeCardDialog`
   * instance, just in different modes). */
  onEditTimeCard: (card: CsmTimeCard) => void;
}

// Every column is left-aligned for a consistent scan line down the table,
// except "Actions" -- a row of icon buttons rather than text, which reads
// better centered under its own column. Mirrors CallRequestsTable's
// HEADER_CELLS/GRID convention so the two tabs read consistently.
const HEADER_CELLS: string[] = [
  "Preview",
  "Engineer",
  "State",
  "Minutes",
  "Billable",
  "Logged",
  "Actions",
];

const GRID =
  "minmax(56px, 0.4fr) minmax(140px, 1.1fr) minmax(140px, 1.1fr) minmax(80px, 0.6fr) minmax(90px, 0.6fr) minmax(110px, 0.8fr) minmax(120px, 0.9fr)";

/**
 * The body of a case's "Time tracking" tab: the time cards logged on this
 * case, with a running total and per-entry status. A team lead can review
 * (accept or reject) any submitted entry inline. Available even after the
 * case is closed — time is often logged after the fact.
 */
export default function CaseTimeCardsPanel({
  caseId,
  onLogTime,
  onEditTimeCard,
}: CaseTimeCardsPanelProps): JSX.Element {
  const { data, isLoading, isError, refetch, isFetching, dataUpdatedAt } =
    useCaseTimeCards(caseId);
  const isTeamLead = useIsTeamLead();
  const me = useCurrentEngineer();
  const decide = useDecideTimeCard();
  const deleteTimeCard = useDeleteTimeCard();
  const { showError } = useErrorBanner();
  const [reviewCard, setReviewCard] = useState<CsmTimeCard | null>(null);
  const [pendingDelete, setPendingDelete] = useState<CsmTimeCard | null>(null);
  const [detailCard, setDetailCard] = useState<CsmTimeCard | null>(null);

  const cards = useMemo(() => data?.cards ?? [], [data]);
  const total = useMemo(
    () => cards.reduce((s, c) => s + c.totalMinutes, 0),
    [cards],
  );

  return (
    <Card variant="outlined" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      {/* Header */}
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
          <Clock size={16} />
          <Typography variant="subtitle2">Time tracked</Typography>
          {!isLoading && !isError && (
            <Chip
              size="small"
              variant="outlined"
              label={`${total} min · ${cards.length} ${cards.length === 1 ? "entry" : "entries"}`}
            />
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
          <RefreshButton
            onRefresh={() => void refetch()}
            isFetching={isFetching}
            updatedAt={dataUpdatedAt}
            label="Refresh time cards"
          />
          <Button
            size="small"
            variant="contained"
            startIcon={<Plus size={14} />}
            onClick={onLogTime}
            sx={{ textTransform: "none" }}
          >
            Log time
          </Button>
        </Box>
      </Box>

      {!isError && data?.truncated && (
        <TimeCardTruncatedNotice hint="Some entries on this case may not be shown." />
      )}

      {/* Content */}
      {isLoading && (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} variant="rounded" height={64} />
          ))}
        </Box>
      )}

      {isError && (
        <Typography variant="body2" color="error" sx={{ py: 2 }}>
          Could not load time cards.
        </Typography>
      )}

      {!isLoading && !isError && cards.length === 0 && (
        <Box sx={{ py: 3, textAlign: "center" }}>
          <Typography variant="body2" color="text.secondary">
            No time logged on this case yet.
          </Typography>
        </Box>
      )}

      {!isLoading && !isError && cards.length > 0 && (
        <Box
          sx={{
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            overflowX: "auto",
            overflowY: "hidden",
            display: "grid",
            gridTemplateColumns: GRID,
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
                sx={{ fontWeight: 600, textAlign: label === "Actions" ? "center" : "left" }}
              >
                {label}
              </Typography>
            ))}
          </Box>

          {/* Rows */}
          {cards.map((c) => {
            const decision = decisionSummary(c);
            const canEdit = c.state === "submitted" && !!me.id && c.userId === me.id;
            // Never shown on your own card: the backend 403s a self-decide
            // regardless of approver status, so a card you submitted
            // yourself can never actually be reviewed by you. Also gated on
            // being in the card's own approver list -- this panel shows
            // every submitted card on the case, not just ones assigned to
            // the signed-in lead, and the backend 403s a decision from
            // anyone not in that list (confirmed live).
            const canReview =
              isTeamLead &&
              c.state === "submitted" &&
              !!me.id &&
              c.userId !== me.id &&
              !!c.approvers?.some((a) => a.id === me.id);

            return (
              <Box
                key={c.id}
                sx={{
                  gridColumn: "1 / -1",
                  display: "grid",
                  gridTemplateColumns: "subgrid",
                  columnGap: 2,
                  alignItems: "start",
                  px: 2,
                  py: 1.25,
                  borderBottom: 1,
                  borderColor: "divider",
                  "&:last-of-type": { borderBottom: 0 },
                }}
              >
                {/* Preview: view-detail eye icon, its own leading column so
                    it reads as "inspect this row" rather than one of the
                    row's actions. Read-only, so shown on every row
                    regardless of state or ownership -- unlike Edit/Review
                    below, there's no authorization concern here. */}
                <Box sx={{ justifySelf: "start" }}>
                  <IconButton
                    size="small"
                    aria-label={`View details for ${c.userName}'s entry`}
                    aria-pressed={detailCard?.id === c.id}
                    data-testid={`timecard-view-${c.id}`}
                    onClick={() =>
                      setDetailCard((prev) => (prev?.id === c.id ? null : c))
                    }
                  >
                    <Eye size={16} />
                  </IconButton>
                </Box>

                <Typography variant="body2" noWrap title={c.userName}>
                  {c.userName}
                </Typography>

                {/* State: chip + decision summary, when present. */}
                <Box sx={{ justifySelf: "start", minWidth: 0 }}>
                  <TimeCardStatusChip state={c.state} />
                  {decision && (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      noWrap
                      title={decision}
                      sx={{ display: "block", mt: 0.5 }}
                    >
                      {decision}
                    </Typography>
                  )}
                </Box>

                <Typography variant="body2">{c.totalMinutes} min</Typography>

                <Typography variant="body2">{billableLabel(c.billable)}</Typography>

                <Typography variant="caption" color="text.secondary" noWrap>
                  <RelativeDate value={c.workDate} />
                </Typography>

                {/* Actions: Edit/Delete for your own still-submitted card,
                    Review for a team lead. The view-detail eye icon has its
                    own leading "Preview" column instead. Centered under the
                    "Actions" header, matching CallRequestsTable. */}
                <Box sx={{ minWidth: 0, textAlign: "center" }}>
                  <Box
                    sx={{
                      display: "flex",
                      gap: 0.5,
                      alignItems: "center",
                      justifyContent: "center",
                      flexWrap: "wrap",
                    }}
                  >
                    {canEdit && (
                      <>
                        <Tooltip title="Edit">
                          <IconButton
                            size="small"
                            aria-label="Edit time card"
                            onClick={() => onEditTimeCard(c)}
                          >
                            <Pencil size={16} />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Delete">
                          <IconButton
                            size="small"
                            color="error"
                            aria-label="Delete time card"
                            onClick={() => setPendingDelete(c)}
                          >
                            <Trash2 size={16} />
                          </IconButton>
                        </Tooltip>
                      </>
                    )}
                    {canReview && (
                      <Tooltip title="Review">
                        <IconButton
                          size="small"
                          color="primary"
                          aria-label="Review time card"
                          onClick={() => setReviewCard(c)}
                        >
                          <ClipboardCheck size={16} />
                        </IconButton>
                      </Tooltip>
                    )}
                  </Box>
                </Box>
              </Box>
            );
          })}
        </Box>
      )}

      {detailCard && (
        <TimeCardDetailsDialog card={detailCard} onClose={() => setDetailCard(null)} />
      )}

      {reviewCard && (
        <TimeCardReviewDialog
          card={reviewCard}
          isDeciding={decide.isPending}
          onClose={() => setReviewCard(null)}
          onDecide={(decision) =>
            decide.mutate(decision, {
              onSuccess: () => setReviewCard(null),
              onError: (err) => {
                // The backend 403s when the signed-in user isn't authorized
                // to decide this specific card — surface its own message
                // rather than failing silently (see CsmTimeCardsPage.tsx for
                // the same pattern and the confirmed-live evidence).
                const msg =
                  err instanceof BackendApiError && err.status < 500 && err.message
                    ? err.message
                    : "Could not submit your decision. Please try again.";
                showError(msg, err);
              },
            })
          }
        />
      )}

      <Dialog
        open={!!pendingDelete}
        onClose={() => {
          if (!deleteTimeCard.isPending) setPendingDelete(null);
        }}
        maxWidth="xs"
        fullWidth
      >
        <DialogTitle>Delete time card?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            Permanently delete this {pendingDelete?.totalMinutes} min entry? This can&apos;t be
            undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            color="inherit"
            onClick={() => setPendingDelete(null)}
            disabled={deleteTimeCard.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            color="error"
            disabled={deleteTimeCard.isPending}
            onClick={() => {
              if (!pendingDelete) return;
              const target = pendingDelete;
              deleteTimeCard.mutate(target.id, {
                onSuccess: () => setPendingDelete(null),
                onError: (err) => {
                  setPendingDelete(null);
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not delete this time card.";
                  showError(msg, err);
                },
              });
            }}
          >
            {deleteTimeCard.isPending ? "Deleting…" : "Delete"}
          </Button>
        </DialogActions>
      </Dialog>
    </Card>
  );
}
