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

import { useEffect, useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import type { CsmTimeCard } from "@src/types";
import { formatMinutes } from "@utils/timecard";

// Mirrors the ServiceNow form's lead-comment cap.
const LEAD_COMMENT_MAX = 500;

interface TimeCardReviewDialogProps {
  open: boolean;
  card: CsmTimeCard | null;
  decision: "approved" | "rejected";
  submitting: boolean;
  onClose: () => void;
  onConfirm: (leadComment: string | undefined) => void;
}

export function TimeCardReviewDialog({
  open,
  card,
  decision,
  submitting,
  onClose,
  onConfirm,
}: TimeCardReviewDialogProps) {
  const [comment, setComment] = useState("");

  // Reset the field each time the dialog opens for a fresh card/decision.
  useEffect(() => {
    if (open) setComment("");
  }, [open, card?.id, decision]);

  const isReject = decision === "rejected";
  // A rejection must tell the engineer why; an approval comment is optional.
  const commentInvalid = isReject && comment.trim().length === 0;

  return (
    <Dialog
      open={open}
      onClose={submitting ? undefined : onClose}
      fullWidth
      maxWidth="xs"
      // Force an opaque surface — the Acrylic theme's `background.paper` is translucent.
      slotProps={{ paper: { sx: { backgroundImage: "none", backgroundColor: "background.default" } } }}
    >
      <DialogTitle>{isReject ? "Reject time card" : "Approve time card"}</DialogTitle>
      <DialogContent>
        <Stack gap={2} pt={0.5}>
          {card && (
            <Typography variant="body2" color="text.secondary">
              {card.caseNumber} · {card.userName} · {formatMinutes(card.totalMinutes)}
            </Typography>
          )}
          <TextField
            label={isReject ? "Reason (required)" : "Comment (optional)"}
            value={comment}
            onChange={(e) => setComment(e.target.value.slice(0, LEAD_COMMENT_MAX))}
            multiline
            minRows={2}
            fullWidth
            required={isReject}
            error={commentInvalid}
            helperText={`${comment.length}/${LEAD_COMMENT_MAX}`}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color={isReject ? "error" : "success"}
          disabled={submitting || commentInvalid}
          onClick={() => onConfirm(comment.trim() || undefined)}
        >
          {isReject ? "Reject" : "Approve"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
