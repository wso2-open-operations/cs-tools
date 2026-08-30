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

import { Box, Button, Divider, IconButton, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Check, X } from "@wso2/oxygen-ui-icons-react";
import { useRef, type JSX, type ReactNode } from "react";
import FloatingSlidePanel from "@components/FloatingSlidePanel";
import RelativeDate from "@components/RelativeDate";
import CasePreviewContent from "@features/csm-cases/components/CasePreviewContent";
import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";
import { QUICK_PREVIEW_EYE_SELECTOR } from "@features/csm-cases/utils/quickPreviewEye";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import { billableLabel } from "@features/csm-timecards/constants/timeCardConstants";
import { decisionSummary } from "@features/csm-timecards/utils/timeCardDecision";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { useCloseOnOutsideClick } from "@hooks/useCloseOnOutsideClick";
import type { TimecardAction } from "@features/csm-timecards/utils/timeSheetState";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

interface TimeCardCasePreviewDrawerProps {
  /** The card being previewed. `null` keeps the drawer mounted-but-closed, so
   * its close transition can play instead of unmounting mid-animation. */
  card: CsmTimeCard | null;
  /** This card's own eligible actions (see `cardActions`) — decides which of
   * the Approve/Reject buttons render, exactly like the row-level ones in
   * `TimeCardsTable`. Ignored (no buttons render) while `card` is `null`. */
  actions: TimecardAction[];
  onClose: () => void;
  /** Approve or reject the previewed card. The caller is expected to both
   * open the same confirm step the row-level buttons use (see
   * `CsmTimeCardsPage`'s `handleCardAction`/`TimeCardReviewDialog`) *and*
   * close this drawer — closing here rather than after the decision
   * resolves avoids this panel sitting open showing the card's now-stale
   * pre-decision state underneath the confirm dialog. */
  onDecide: (action: Extract<TimecardAction, "approve" | "reject">) => void;
}

function Field({ label, value }: { label: string; value: ReactNode }): JSX.Element {
  return (
    <Box sx={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      {typeof value === "string" ? <Typography variant="body2">{value}</Typography> : value}
    </Box>
  );
}

function WorkLogComment({ html }: { html?: string }): JSX.Element | null {
  if (!html || isBlankHtml(html)) return null;
  const safeHtml = sanitizeRichTextHtml(html);
  return (
    <Field
      label="Engineer's comment"
      value={
        <Box
          sx={{
            fontSize: "0.875rem",
            lineHeight: 1.5,
            wordBreak: "break-word",
            // Newly generated comments no longer carry a per-run
            // `white-space: pre-wrap` inline style (digiops-cs#2933) —
            // declared once here instead. Older comments carry their own
            // inline style and are unaffected either way.
            whiteSpace: "pre-wrap",
            "& p": { my: 0.5 },
            "& p:first-of-type": { mt: 0 },
            "& p:last-child": { mb: 0 },
            "& ul, & ol": { my: 0.5, pl: 3 },
            "& a": { color: "primary.main" },
            "& img": { maxWidth: "100%", height: "auto" },
          }}
          dangerouslySetInnerHTML={{ __html: safeHtml }}
        />
      }
    />
  );
}

/**
 * "Quick review" panel opened from a time card's own view action: the card's
 * details up top (same fields `TimeCardDetailModal` used to show, before this
 * replaced it as the Approvals-tab entry point), the linked case's own
 * `CasePreviewContent` underneath (fetched fresh via `useGetCsmCaseDetail` —
 * a `CsmTimeCard` only carries `caseId`/`caseNumber`, not the case's own
 * subject/state/severity/comments), and Approve/Reject at the bottom when
 * eligible. Exists so a reviewer working through a stack of submitted cards
 * can see enough case context to decide without leaving this panel for the
 * full case page — the reason this exists at all (per the product
 * discussion this was built from): making 20-30 individual approvals a day
 * involve one fewer navigation each.
 */
export default function TimeCardCasePreviewDrawer({
  card,
  actions,
  onClose,
  onDecide,
}: TimeCardCasePreviewDrawerProps): JSX.Element {
  const { data: caseDetail, isLoading, isError } = useGetCsmCaseDetail(card?.caseId);
  const decision = card ? decisionSummary(card) : null;
  const contentRef = useRef<HTMLDivElement | null>(null);
  useCloseOnOutsideClick(!!card, contentRef, QUICK_PREVIEW_EYE_SELECTOR, onClose);

  return (
    // `FloatingSlidePanel` (not `Drawer`) -- a `Drawer` is `Modal`-backed,
    // which enforces a focus trap and marks the rest of the page
    // `aria-hidden` for as long as it's open, regardless of `hideBackdrop`
    // or pointer-events tricks (those only ever affected mouse clicks, not
    // `Modal`'s own accessibility isolation). This panel has no backdrop
    // and no modal behavior at all, so the rest of the page stays fully
    // interactive for every input method -- not just the mouse -- letting
    // a click on a different row's quick-preview eye land normally while
    // this preview is already open. `useCloseOnOutsideClick` below replaces
    // the click-to-close behavior a `Drawer`'s backdrop would otherwise
    // give, minus the eye buttons (their own onClick already decides the
    // next state).
    <FloatingSlidePanel open={!!card} ariaLabel="Time card preview">
      {card && (
        <Box
          ref={contentRef}
          sx={{ display: "flex", flexDirection: "column", height: "100%", overflowY: "auto" }}
        >
          <Box sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
            <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 1 }}>
              <Typography variant="subtitle2" color="text.secondary">
                Time card
              </Typography>
              <IconButton size="small" onClick={onClose} aria-label="Close preview">
                <X size={18} />
              </IconButton>
            </Box>

            <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 2 }}>
              <Field label="Engineer" value={card.userName} />
              <Field label="Project" value={card.projectName} />
              <Field label="Billable" value={billableLabel(card.billable)} />
              <Field
                label="Logged"
                value={
                  <Typography variant="body2">
                    <RelativeDate value={card.workDate} />
                  </Typography>
                }
              />
              <Field label="Minutes" value={`${card.totalMinutes} min`} />
              <Field
                label="State"
                value={
                  <Box sx={{ mt: 0.5 }}>
                    <TimeCardStatusChip state={card.state} />
                  </Box>
                }
              />
            </Box>

            <WorkLogComment html={card.workLogComment} />

            {card.state === "submitted" && card.approvers && card.approvers.length > 0 && (
              <Field
                label="Approvers"
                value={card.approvers.map((approver) => approver.name).join(", ")}
              />
            )}

            {decision && (
              <Typography variant="body2" sx={{ whiteSpace: "normal", wordBreak: "break-word" }}>
                {decision}
              </Typography>
            )}

            {actions.some((a) => a === "approve" || a === "reject") && (
              <Box sx={{ display: "flex", gap: 1 }}>
                {actions.includes("reject") && (
                  <Button
                    color="error"
                    variant="outlined"
                    size="small"
                    startIcon={<X size={16} />}
                    onClick={() => onDecide("reject")}
                  >
                    Reject
                  </Button>
                )}
                {actions.includes("approve") && (
                  <Button
                    color="success"
                    variant="outlined"
                    size="small"
                    startIcon={<Check size={16} />}
                    onClick={() => onDecide("approve")}
                  >
                    Approve
                  </Button>
                )}
              </Box>
            )}
          </Box>

          <Divider />

          {isLoading ? (
            <Stack gap={1.5} sx={{ p: 2.5 }}>
              <Skeleton variant="rounded" height={28} width="60%" />
              <Skeleton variant="rounded" height={64} />
              <Skeleton variant="rounded" height={64} />
            </Stack>
          ) : isError || !caseDetail ? (
            <Typography variant="body2" color="error" sx={{ p: 2.5 }}>
              Could not load the case.
            </Typography>
          ) : (
            <CasePreviewContent row={caseDetail} onClose={onClose} hideCloseButton />
          )}
        </Box>
      )}
    </FloatingSlidePanel>
  );
}
