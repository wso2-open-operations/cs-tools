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

import type { JSX, ReactNode } from "react";
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from "@wso2/oxygen-ui";
import RelativeDate from "@components/RelativeDate";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import { ACTIVITY_BUCKETS, billableLabel } from "@features/csm-timecards/constants/timeCardConstants";
import { decisionSummary } from "@features/csm-timecards/utils/timeCardDecision";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

interface TimeCardDetailsDialogProps {
  card: CsmTimeCard;
  onClose: () => void;
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

/**
 * The engineer's own work-log comment from submission — ServiceNow rich-text
 * HTML, so it's sanitized and rendered as HTML (same policy as a case
 * comment's body, and the same pattern as `TimeCardReviewDialog`). Renders
 * nothing when the card has none (e.g. logged before this field was mapped).
 */
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
 * Per-activity minute breakdown, in the fixed `ACTIVITY_BUCKETS` display
 * order — absent on a card logged before this field was mapped (see
 * `CsmTimeCard.breakdown`), so renders nothing rather than a zeroed-out list.
 */
function Breakdown({ breakdown }: { breakdown?: CsmTimeCard["breakdown"] }): JSX.Element | null {
  if (!breakdown) return null;
  return (
    <Field
      label="Breakdown"
      value={
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "1fr auto",
            columnGap: 2,
            rowGap: 0.25,
          }}
        >
          {ACTIVITY_BUCKETS.map((bucket) => (
            <Box key={bucket.key} sx={{ display: "contents" }}>
              <Typography variant="body2">{bucket.label}</Typography>
              <Typography variant="body2" sx={{ textAlign: "right" }}>
                {breakdown[bucket.key]} min
              </Typography>
            </Box>
          ))}
        </Box>
      }
    />
  );
}

/**
 * Read-only details view of a single time card, opened from its row's "View
 * details" action in `CaseTimeCardsPanel`. Unlike `TimeCardCasePreviewDrawer`
 * (the Approvals-tab equivalent), this doesn't also fetch and render the
 * linked case — the panel this opens from is already embedded in that same
 * case's own detail page, so re-showing it here would be redundant. Available
 * on every card regardless of state or ownership: it's read-only, so there's
 * no authorization concern the way there is for Edit/Review.
 */
export default function TimeCardDetailsDialog({
  card,
  onClose,
}: TimeCardDetailsDialogProps): JSX.Element {
  const decision = decisionSummary(card);

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Time card · {card.caseNumber} · {card.totalMinutes} min
      </DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
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
            <Field label="Total time" value={`${card.totalMinutes} min`} />
            <Field
              label="State"
              value={
                <Box sx={{ mt: 0.5 }}>
                  <TimeCardStatusChip state={card.state} />
                </Box>
              }
            />
            {card.issueComplexity && (
              <Field label="Issue complexity" value={card.issueComplexity} />
            )}
          </Box>

          <WorkLogComment html={card.workLogComment} />

          <Breakdown breakdown={card.breakdown} />

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
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
}
