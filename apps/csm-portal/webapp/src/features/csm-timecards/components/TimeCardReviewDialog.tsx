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
import { Check, X } from "@wso2/oxygen-ui-icons-react";
import RelativeTime from "@components/RelativeTime";
import {
  billableLabel,
  breakdownSummary,
  LEAD_COMMENT_MAX,
  TIME_CARD_ACTIVITY_LABEL,
} from "@features/csm-timecards/constants/timeCardConstants";
import type {
  CsmTimeCard,
  TimeCardDecisionInput,
} from "@features/csm-timecards/types/timeCards";

interface TimeCardReviewDialogProps {
  card: CsmTimeCard;
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
 * Team-lead review of a pending time card. Shows the engineer, the case, the
 * activity breakdown and the work log, and captures an optional Lead's comment
 * before accepting or rejecting. Reject requires nothing extra but a reason is
 * encouraged — the comment is sent with either decision.
 */
export default function TimeCardReviewDialog({
  card,
  isDeciding,
  onClose,
  onDecide,
}: TimeCardReviewDialogProps): JSX.Element {
  const [leadComment, setLeadComment] = useState("");

  const decide = (state: "approved" | "rejected"): void =>
    onDecide({ cardId: card.id, state, leadComment });

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        Review time card · {card.caseNumber} · {card.totalHours.toFixed(2)}h
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
            <Field label="Date" value={card.date} />
            <Field label="Category" value={card.category} />
            <Field label="Issue complexity" value={card.issueComplexity} />
            <Field label="Billable" value={billableLabel(card.billable)} />
          </Box>

          <Field label="Time breakdown" value={breakdownSummary(card.breakdown)} />

          <Box>
            <Typography variant="caption" color="text.secondary">
              Work log
            </Typography>
            <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
              {card.workLogComment}
            </Typography>
          </Box>

          {card.activity.length > 0 && (
            <Box>
              <Typography variant="caption" color="text.secondary">
                Activity
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, mt: 0.25 }}>
                {card.activity.map((a, i) => (
                  <Typography key={i} variant="caption" color="text.secondary">
                    <Box component="span" sx={{ fontWeight: 600, color: "text.primary" }}>
                      {TIME_CARD_ACTIVITY_LABEL[a.action]}
                    </Box>{" "}
                    by {a.by} · <RelativeTime iso={a.at} />
                    {a.note ? ` — ${a.note}` : ""}
                  </Typography>
                ))}
              </Box>
            </Box>
          )}

          <TextField
            label="Lead's comment (optional)"
            multiline
            minRows={2}
            value={leadComment}
            onChange={(e) =>
              setLeadComment(e.target.value.slice(0, LEAD_COMMENT_MAX))
            }
            helperText={`${LEAD_COMMENT_MAX - leadComment.length} characters left`}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isDeciding}>
          Cancel
        </Button>
        <Button
          color="error"
          variant="outlined"
          startIcon={<X size={16} />}
          onClick={() => decide("rejected")}
          disabled={isDeciding}
        >
          Reject
        </Button>
        <Button
          color="success"
          variant="contained"
          startIcon={<Check size={16} />}
          onClick={() => decide("approved")}
          disabled={isDeciding}
        >
          Accept
        </Button>
      </DialogActions>
    </Dialog>
  );
}
