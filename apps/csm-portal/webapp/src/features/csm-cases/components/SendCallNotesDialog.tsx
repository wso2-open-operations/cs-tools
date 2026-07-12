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
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeCallRequestView } from "@api/backend/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface SendCallNotesDialogProps {
  callRequest: BeCallRequestView | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (input: {
    notes: string;
    plan?: string;
    attendees?: string;
    actionItems?: string;
    actualDuration?: number;
  }) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function SendCallNotesDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: SendCallNotesDialogProps): JSX.Element {
  const [notes, setNotes] = useState("");
  const [actionItems, setActionItems] = useState("");
  const [plan, setPlan] = useState("");
  const [attendees, setAttendees] = useState("");
  const [actualDuration, setActualDuration] = useState("");

  // Reset the form whenever the target call request changes (render-time
  // state adjustment, not an effect -- see React docs on this pattern).
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setNotes("");
    setActionItems("");
    setPlan("");
    setAttendees("");
    setActualDuration(callRequest?.durationMin ? String(callRequest.durationMin) : "");
  }

  const handleClose = () => {
    setNotes("");
    setActionItems("");
    setPlan("");
    setAttendees("");
    setActualDuration("");
    onClose();
  };

  const durationNum =
    actualDuration.trim() === "" ? undefined : parseInt(actualDuration, 10);
  const durationValid =
    durationNum === undefined || (!isNaN(durationNum) && durationNum > 0);

  const canSubmit = notes.trim().length > 0 && durationValid;

  const handleSubmit = () => {
    if (!canSubmit) return;
    onSubmit({
      notes: notes.trim(),
      ...(plan.trim() ? { plan: plan.trim() } : {}),
      ...(attendees.trim() ? { attendees: attendees.trim() } : {}),
      ...(actionItems.trim() ? { actionItems: actionItems.trim() } : {}),
      ...(durationNum !== undefined ? { actualDuration: durationNum } : {}),
    });
  };

  return (
    <Dialog open={!!callRequest} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Send call notes</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          <Typography variant="body2" color="text.secondary">
            Submitting notes concludes this call request.
          </Typography>
          <TextField
            label="Notes"
            multiline
            minRows={3}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            fullWidth
            required
            disabled={submitting}
            placeholder="Summary of what was discussed..."
          />
          <TextField
            label="Action items (optional)"
            multiline
            minRows={2}
            value={actionItems}
            onChange={(e) => setActionItems(e.target.value)}
            fullWidth
            disabled={submitting}
          />
          <TextField
            label="Plan (optional)"
            multiline
            minRows={2}
            value={plan}
            onChange={(e) => setPlan(e.target.value)}
            fullWidth
            disabled={submitting}
          />
          <TextField
            label="Attendees (optional)"
            value={attendees}
            onChange={(e) => setAttendees(e.target.value)}
            fullWidth
            disabled={submitting}
            placeholder="Comma-separated names or emails"
          />
          <TextField
            label="Actual duration (minutes, optional)"
            type="number"
            value={actualDuration}
            onChange={(e) => setActualDuration(e.target.value)}
            fullWidth
            disabled={submitting}
            error={actualDuration !== "" && !durationValid}
            inputProps={{ min: 1 }}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={handleClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={!canSubmit || submitting}
          loading={submitting}
        >
          Send notes
        </Button>
      </DialogActions>
    </Dialog>
  );
}
