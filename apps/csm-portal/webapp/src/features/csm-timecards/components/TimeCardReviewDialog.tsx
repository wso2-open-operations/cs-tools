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
  LEAD_COMMENT_MAX,
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
 * Team-lead review of a submitted time card, then accept or reject with an
 * optional comment. Only shows what the backend actually returns on read —
 * category, issue complexity, work-log comment and the hour breakdown are
 * accepted on create but never echoed back, so they aren't shown here.
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
            <Field label="Project" value={card.projectName} />
            <Field label="Billable" value={billableLabel(card.billable)} />
            <Box>
              <Typography variant="caption" color="text.secondary">
                Logged
              </Typography>
              <Typography variant="body2">
                <RelativeTime iso={card.createdOn} />
              </Typography>
            </Box>
          </Box>

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
