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
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  TextField,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useState, type ChangeEvent, type JSX } from "react";

const REASON_MAX_LENGTH = 1000;

export interface RejectSolutionDialogProps {
  open: boolean;
  isPending: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => void;
}

/**
 * Confirm dialog for the "Reject Solution" action. Unlike CaseStateConfirmDialog
 * (used by every other case action), this collects an optional reason so support
 * engineers see why a solution was rejected instead of the case silently
 * bouncing back to "Waiting On WSO2" with no context.
 *
 * The reason is optional: the customer can submit blank, but there is no
 * separate "Skip" action — Cancel aborts the whole action (no comment, no
 * state change), Confirm always submits (with or without text).
 *
 * @param {RejectSolutionDialogProps} props - Dialog control props.
 * @returns {JSX.Element} The reject solution dialog.
 */
export default function RejectSolutionDialog({
  open,
  isPending,
  onClose,
  onConfirm,
}: RejectSolutionDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  // Tracks the previous `open` value so we can reset the reason text exactly
  // once per close, during render rather than in an effect (React's documented
  // "adjusting state when a prop changes" pattern) — covers every close path:
  // Cancel, a successful submit, or a failed submit the caller chose to close.
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (!open) setReason("");
  }

  const handleClose = (): void => {
    if (isPending) return;
    onClose();
  };

  const handleReasonChange = (
    e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>,
  ): void => {
    setReason(e.target.value.slice(0, REASON_MAX_LENGTH));
  };

  const handleConfirm = (): void => {
    if (isPending) return;
    onConfirm(reason.trim());
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="reject-solution-dialog-title"
    >
      <DialogTitle
        id="reject-solution-dialog-title"
        sx={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}
      >
        Reject Solution
        <IconButton size="small" onClick={handleClose} aria-label="close" disabled={isPending}>
          <X size={18} />
        </IconButton>
      </DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          Are you sure you want to reject the proposed solution? Let us know
          what&apos;s still missing so we can help further.
        </DialogContentText>
        <TextField
          id="reject-solution-reason"
          label="Reason (optional)"
          placeholder="Tell us why the proposed solution didn't resolve your issue..."
          value={reason}
          onChange={handleReasonChange}
          fullWidth
          multiline
          rows={4}
          disabled={isPending}
          inputProps={{
            "aria-label": "Reason for rejecting the solution",
            maxLength: REASON_MAX_LENGTH,
          }}
          helperText={`${reason.length}/${REASON_MAX_LENGTH}`}
          FormHelperTextProps={{ sx: { textAlign: "right", m: 0, mt: 0.5 } }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={handleConfirm}
          disabled={isPending}
          startIcon={
            isPending ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {isPending ? "Rejecting…" : "Reject Solution"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
