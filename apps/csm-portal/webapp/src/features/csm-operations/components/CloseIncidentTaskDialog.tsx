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
  DialogTitle,
  FormControlLabel,
  Radio,
  RadioGroup,
  TextField,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeIncidentTaskState } from "@api/backend/types";

export type ClosedTaskState = Extract<
  BeIncidentTaskState,
  "CLOSED_COMPLETE" | "CLOSED_INCOMPLETE" | "CLOSED_SKIPPED"
>;

const OUTCOMES: { value: ClosedTaskState; label: string }[] = [
  { value: "CLOSED_COMPLETE", label: "Closed Complete" },
  { value: "CLOSED_INCOMPLETE", label: "Closed Incomplete" },
  { value: "CLOSED_SKIPPED", label: "Closed Skipped" },
];

interface CloseIncidentTaskDialogProps {
  taskNumber: string;
  isSubmitting: boolean;
  /** User-facing message for the most recent failed attempt, if any. */
  error?: string | null;
  /** The outcome pre-selected when the dialog opens (Closed Complete if not given). */
  initialState?: ClosedTaskState;
  onClose: () => void;
  onConfirm: (fields: { state: ClosedTaskState; closeNotes: string }) => void;
}

/**
 * Closes an incident task with one of the three closed states
 * ServiceNow's task model has, plus optional close notes. Closing every
 * task is what lets the parent incident be closed.
 */
export default function CloseIncidentTaskDialog({
  taskNumber,
  isSubmitting,
  error,
  initialState,
  onClose,
  onConfirm,
}: CloseIncidentTaskDialogProps): JSX.Element {
  const [state, setState] = useState<ClosedTaskState>(initialState ?? "CLOSED_COMPLETE");
  const [closeNotes, setCloseNotes] = useState("");

  return (
    <Dialog
      open
      onClose={() => {
        if (!isSubmitting) onClose();
      }}
      maxWidth="xs"
      fullWidth
      aria-labelledby="close-incident-task-title"
    >
      <DialogTitle id="close-incident-task-title">Close {taskNumber}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && <Alert severity="error">{error}</Alert>}
          <RadioGroup
            aria-label="Outcome"
            value={state}
            onChange={(e) => setState(e.target.value as ClosedTaskState)}
          >
            {OUTCOMES.map((o) => (
              <FormControlLabel
                key={o.value}
                value={o.value}
                control={<Radio size="small" />}
                label={o.label}
                disabled={isSubmitting}
              />
            ))}
          </RadioGroup>
          <TextField
            label="Close notes"
            multiline
            minRows={2}
            fullWidth
            size="small"
            value={closeNotes}
            disabled={isSubmitting}
            onChange={(e) => setCloseNotes(e.target.value)}
            helperText="Optional."
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onConfirm({ state, closeNotes: closeNotes.trim() })}
          disabled={isSubmitting}
          loading={isSubmitting}
        >
          Close task
        </Button>
      </DialogActions>
    </Dialog>
  );
}
