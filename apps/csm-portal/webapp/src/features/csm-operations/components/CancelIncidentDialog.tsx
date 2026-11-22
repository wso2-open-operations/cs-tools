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
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface CancelIncidentDialogProps {
  /** Shown in the body so the engineer can see which incident they're ending. */
  incidentNumber?: string | null;
  /**
   * How many incidents the confirmation covers (bulk Edit). Above 1 the copy
   * and the confirm button switch to the plural; `incidentNumber` is ignored.
   */
  incidentCount?: number;
  /** True while the `PATCH { state: CANCELLED }` is in flight. */
  isSubmitting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Confirmation before moving an incident to `CANCELLED`. Cancelled is terminal
 * (no transition out, comments disabled), so a single misclick must not end
 * the incident on its own. Every path to Cancelled goes through it: the
 * "Change state" menu, the Edit dialog's save and the list's bulk Edit.
 */
export default function CancelIncidentDialog({
  incidentNumber,
  incidentCount = 1,
  isSubmitting,
  onClose,
  onConfirm,
}: CancelIncidentDialogProps): JSX.Element {
  const plural = incidentCount > 1;
  const subject = plural
    ? `${incidentCount} incidents will be moved to Cancelled.`
    : incidentNumber
      ? `${incidentNumber} will be moved to Cancelled.`
      : "This incident will be moved to Cancelled.";
  return (
    <Dialog
      open
      onClose={() => {
        if (!isSubmitting) onClose();
      }}
      maxWidth="xs"
      fullWidth
      aria-labelledby="cancel-incident-title"
      aria-describedby="cancel-incident-body"
    >
      <DialogTitle id="cancel-incident-title">
        {plural ? `Cancel ${incidentCount} incidents?` : "Cancel this incident?"}
      </DialogTitle>
      <DialogContent dividers>
        <DialogContentText id="cancel-incident-body" variant="body2">
          {subject} Cancelled is final: a cancelled incident can&apos;t be reopened or commented on.
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        {/* Not "Cancel": next to a confirm that cancels the incident the word reads as the action. */}
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Go back
        </Button>
        <Button variant="contained" color="error" onClick={onConfirm} loading={isSubmitting}>
          {plural ? "Cancel incidents" : "Cancel incident"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
