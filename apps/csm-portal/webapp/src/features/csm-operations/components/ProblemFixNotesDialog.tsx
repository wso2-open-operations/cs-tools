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
import { useState, type JSX } from "react";

interface ProblemFixNotesDialogProps {
  isSubmitting: boolean;
  /** User-facing message for the most recent failed attempt, if any. */
  error?: string | null;
  onClose: () => void;
  onConfirm: (fields: { causeNotes: string; fixNotes: string }) => void;
}

/**
 * Optional-notes dialog for the `fix` transition (Root Cause Analysis ->
 * Fix in Progress). Unlike `IncidentResolutionDialog`'s `resolutionCode`/
 * `resolutionNotes` (ServiceNow-required, confirmed live: those 500 without
 * it), `causeNotes`/`fixNotes` are genuinely optional on this transition per
 * `CHANGES-problem-update.md` §3 — so this dialog can be skipped rather than
 * blocking the transition on empty fields, same "confirm without forcing
 * input" shape as `ChangeRequestTransitionReasonDialog` but without the
 * required-field gate that dialog has for its two destructive transitions.
 */
export default function ProblemFixNotesDialog({
  isSubmitting,
  error,
  onClose,
  onConfirm,
}: ProblemFixNotesDialogProps): JSX.Element {
  const [causeNotes, setCauseNotes] = useState("");
  const [fixNotes, setFixNotes] = useState("");

  return (
    <Dialog
      open
      onClose={() => {
        if (!isSubmitting) onClose();
      }}
      maxWidth="xs"
      fullWidth
      aria-labelledby="problem-fix-notes-title"
    >
      <DialogTitle id="problem-fix-notes-title">Move to Fix In Progress</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <DialogContentText variant="body2">
            Cause and fix notes are optional here — you can add them now or
            edit them later from the problem's overview.
          </DialogContentText>
          <TextField
            label="Cause notes"
            multiline
            minRows={2}
            fullWidth
            size="small"
            value={causeNotes}
            disabled={isSubmitting}
            onChange={(e) => setCauseNotes(e.target.value)}
            helperText="Root cause / detailed RCA."
          />
          <TextField
            label="Fix notes"
            multiline
            minRows={2}
            fullWidth
            size="small"
            value={fixNotes}
            disabled={isSubmitting}
            onChange={(e) => setFixNotes(e.target.value)}
            helperText="Description of the permanent fix being applied."
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onConfirm({ causeNotes: causeNotes.trim(), fixNotes: fixNotes.trim() })}
          disabled={isSubmitting}
          loading={isSubmitting}
        >
          Move to Fix In Progress
        </Button>
      </DialogActions>
    </Dialog>
  );
}
