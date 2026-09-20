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

import { useState, type JSX } from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import RelativeDate from "@components/RelativeDate";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import {
  billableLabel,
  LEAD_COMMENT_MAX,
} from "@features/csm-timecards/constants/timeCardConstants";
import type { TimecardAction } from "@features/csm-timecards/utils/timeSheetState";
import type {
  CsmTimeCard,
  TimeCardDecisionInput,
} from "@features/csm-timecards/types/timeCards";

interface TimeCardReviewDialogProps {
  card: CsmTimeCard;
  /**
   * The decision already chosen (e.g. the Approve/Reject button clicked on the
   * list) — when set, the dialog only offers that one action, instead of
   * asking the user to pick again from both. Omit for a generic "Review" entry
   * point (see `CaseTimeCardsPanel`'s single Review button) where no decision
   * has been made yet and both options should show.
   */
  action?: TimecardAction;
  /** True while the decision mutation is in flight. */
  isDeciding: boolean;
  onClose: () => void;
  onDecide: (decision: TimeCardDecisionInput) => void;
}

function Field({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="body2">{value}</Typography>
    </Box>
  );
}

/**
 * The engineer's own work-log comment from submission — ServiceNow rich-text
 * HTML, so it's sanitized and rendered as HTML (same policy as a case
 * comment's body). Renders nothing when the card has none (e.g. logged
 * before this field was mapped).
 */
function WorkLogComment({ html }: { html?: string }): JSX.Element | null {
  if (!html || isBlankHtml(html)) return null;
  const safeHtml = sanitizeRichTextHtml(html);
  return (
    <Box>
      <Typography variant="caption" color="text.secondary">
        Engineer's comment
      </Typography>
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
    </Box>
  );
}

/**
 * Team-lead review of a submitted time card, then accept or reject with an
 * optional comment. Only shows what the backend actually returns on read —
 * issue complexity and the per-activity minute breakdown are accepted on
 * create but never echoed back, so they aren't shown here.
 */
export default function TimeCardReviewDialog({
  card,
  action,
  isDeciding,
  onClose,
  onDecide,
}: TimeCardReviewDialogProps): JSX.Element {
  const [leadComment, setLeadComment] = useState("");
  const [rejectBlocked, setRejectBlocked] = useState(false);
  const trimmedComment = leadComment.trim();

  const decide = (state: "approved" | "rejected"): void => {
    // The backend requires a non-empty leadComment when rejecting (there's no
    // other trace a rejection leaves — see CsmTimeCard.rejectionReason), but
    // doesn't say so anywhere in its error response, so this is enforced here
    // rather than surfacing a generic "Invalid request payload." after the fact.
    if (state === "rejected" && trimmedComment === "") {
      setRejectBlocked(true);
      return;
    }
    onDecide({ cardId: card.id, state, leadComment });
  };

  const titlePrefix =
    action === "approve" ? "Accept" : action === "reject" ? "Reject" : "Review";
  const rejectAllowed = action !== "approve";
  const commentMissingForReject = rejectBlocked && trimmedComment === "";

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {titlePrefix} time card · {card.caseNumber} · {card.totalMinutes} min
      </DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: 1.5,
            }}
          >
            <Field label="Engineer" value={card.userName} />
            <Field label="Project" value={card.projectName} />
            <Field label="Billable" value={billableLabel(card.billable)} />
            <Box>
              <Typography variant="caption" color="text.secondary">
                Logged
              </Typography>
              <Typography variant="body2">
                <RelativeDate value={card.workDate} />
              </Typography>
            </Box>
          </Box>

          <WorkLogComment html={card.workLogComment} />

          <TextField
            label={
              rejectAllowed
                ? "Lead's comment (required to reject)"
                : "Lead's comment (optional)"
            }
            multiline
            minRows={2}
            value={leadComment}
            onChange={(e) => {
              setLeadComment(e.target.value.slice(0, LEAD_COMMENT_MAX));
              setRejectBlocked(false);
            }}
            error={commentMissingForReject}
            helperText={
              commentMissingForReject
                ? "A comment is required to reject a time card."
                : `${LEAD_COMMENT_MAX - leadComment.length} characters left`
            }
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isDeciding}>
          Cancel
        </Button>
        {action !== "approve" && (
          <Button
            color="error"
            variant="outlined"
            onClick={() => decide("rejected")}
            disabled={isDeciding}
          >
            Reject
          </Button>
        )}
        {action !== "reject" && (
          <Button
            color="primary"
            variant="outlined"
            onClick={() => decide("approved")}
            disabled={isDeciding}
          >
            Accept
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
