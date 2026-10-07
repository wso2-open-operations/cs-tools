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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField,
} from "@wso2/oxygen-ui";
import { useRef, useState, type JSX } from "react";
import { changeRequestTransitionLabel } from "@features/csm-operations/utils/changeRequests";

interface TransitionCopy {
  title: string;
  body: string;
  confirmLabel: string;
  /** Destructive off-ramps confirm in the error colour. */
  confirmColor: "error";
}

/**
 * Per-target copy for the two destructive transitions. It says only what the
 * action DOES, never why it is taken: the same dialog opens from Review and from
 * Customer Review, with a customer's answer given, pending or never asked, so a
 * sentence about "the review failing" would be false history in an audited record
 * whenever no review was held. The reason typed below is where the why goes.
 * Cancel: the approvals still waiting are withdrawn (their rows become Cancelled),
 * but the ones already given stay on the record, so they are not "lost".
 */
const TRANSITION_COPY: Record<string, TransitionCopy> = {
  rollback: {
    title: "Roll back this change?",
    body:
      "This moves the change request into Rollback, recording that the implemented change is being reversed. Rollback is final and can't be undone from here.",
    confirmLabel: "Roll back",
    confirmColor: "error",
  },
  canceled: {
    title: "Cancel this change request?",
    body:
      "This ends the change request as canceled. It can't be reopened from here. Approvals still waiting are withdrawn; the ones already given stay on its record.",
    confirmLabel: "Cancel change",
    confirmColor: "error",
  },
};

function copyFor(target: string): TransitionCopy {
  return (
    TRANSITION_COPY[target] ?? {
      title: `${changeRequestTransitionLabel(target)}?`,
      body: "This change to the record can't be undone from here.",
      confirmLabel: changeRequestTransitionLabel(target),
      confirmColor: "error",
    }
  );
}

interface ChangeRequestTransitionReasonDialogProps {
  /** Target lifecycle state being confirmed, e.g. `rollback` or `canceled`. */
  target: string;
  /** True while the reason comment and/or the state change are in flight. */
  isSubmitting: boolean;
  /**
   * User-facing message for the most recent failed attempt, if any. Rendered
   * inline so the engineer sees exactly how far the attempt got — in
   * particular whether the reason was already recorded and must not be
   * retyped.
   */
  error?: string | null;
  /**
   * True once the reason has been recorded as an internal note. The field locks and
   * a retry re-sends only the state change, so retrying after a failed patch
   * can't post the same reason twice.
   */
  reasonRecorded?: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => void;
}

/**
 * Confirmation for a change-request transition that needs a stated reason:
 * the destructive `rollback` / `canceled` (effectively irreversible). The
 * confirm action stays disabled until the Reason field has content.
 *
 * The reason is *not* part of the patch body — the change-request PATCH
 * contract has no reason or comment field. The caller records it as an
 * internal note on the change request (not visible to the customer) and only
 * then patches the state; see `CsmChangeRequestDetailPage`. This dialog only
 * collects it.
 */
export default function ChangeRequestTransitionReasonDialog({
  target,
  isSubmitting,
  error,
  reasonRecorded,
  onClose,
  onConfirm,
}: ChangeRequestTransitionReasonDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  const reasonRef = useRef<HTMLTextAreaElement | null>(null);
  const { title, body, confirmLabel, confirmColor } = copyFor(target);
  const canSubmit = reason.trim().length > 0 && !isSubmitting;

  return (
    <Dialog
      open
      onClose={() => {
        if (!isSubmitting) onClose();
      }}
      maxWidth="xs"
      fullWidth
      aria-labelledby="cr-transition-reason-title"
      aria-describedby="cr-transition-reason-body"
      // Focus the field once the dialog has finished opening, not with `autoFocus`
      // on it: the dialog opens in the same tick as the "Change state" menu
      // closes, and that menu's own focus restore wins over `autoFocus`, leaving
      // focus on the dialog's root and anything typed going nowhere.
      slotProps={{ transition: { onEntered: () => reasonRef.current?.focus() } }}
    >
      <DialogTitle id="cr-transition-reason-title">{title}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <DialogContentText id="cr-transition-reason-body" variant="body2">
            {body}
          </DialogContentText>
          <TextField
            label="Reason"
            required
            inputRef={reasonRef}
            multiline
            minRows={3}
            fullWidth
            size="small"
            value={reason}
            disabled={isSubmitting || reasonRecorded}
            onChange={(e) => setReason(e.target.value)}
            helperText={
              reasonRecorded
                ? "Already recorded as an internal note — retrying will only change the state."
                : "Recorded as an internal note (not visible to the customer) before the state changes."
            }
          />
        </Box>
      </DialogContent>
      <DialogActions>
        {/* Not "Close": next to a confirm that closes the change request (or cancels it) the word reads as the action. */}
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Go back
        </Button>
        <Button
          variant="contained"
          color={confirmColor}
          onClick={() => onConfirm(reason.trim())}
          disabled={!canSubmit}
          loading={isSubmitting}
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
