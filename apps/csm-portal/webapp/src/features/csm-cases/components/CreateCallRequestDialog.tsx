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
import { Plus } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

/** Parse an ISO datetime local-value from an <input type="datetime-local"> back to UTC ISO. */
function localInputToUtcIso(localValue: string): string {
  // datetime-local gives "YYYY-MM-DDTHH:mm" which is treated as local time by Date.
  const d = new Date(localValue);
  return d.toISOString();
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CreateDialogProps {
  open: boolean;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (reason: string, utcTimes: string[], durationInMinutes: number) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CreateCallRequestDialog({
  open,
  submitting,
  error,
  onClose,
  onSubmit,
}: CreateDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  const [duration, setDuration] = useState("30");
  // Each entry is a datetime-local string from the input.
  const [timeslots, setTimeslots] = useState<string[]>([""]);

  const handleClose = () => {
    setReason("");
    setDuration("30");
    setTimeslots([""]);
    onClose();
  };

  const addTimeslot = () => setTimeslots((prev) => [...prev, ""]);
  const removeTimeslot = (i: number) =>
    setTimeslots((prev) => prev.filter((_, idx) => idx !== i));
  const updateTimeslot = (i: number, val: string) =>
    setTimeslots((prev) => prev.map((v, idx) => (idx === i ? val : v)));

  const filledSlots = timeslots.filter(Boolean);
  const durationNum = parseInt(duration, 10);
  const canSubmit =
    reason.trim().length > 0 &&
    filledSlots.length > 0 &&
    !isNaN(durationNum) &&
    durationNum >= 1;

  const handleSubmit = () => {
    if (!canSubmit) return;
    onSubmit(
      reason.trim(),
      filledSlots.map(localInputToUtcIso),
      durationNum,
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Request a call</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          <TextField
            label="Reason for the call"
            multiline
            minRows={3}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            fullWidth
            required
            disabled={submitting}
            placeholder="Describe why a call is needed..."
          />
          <TextField
            label="Duration (minutes)"
            type="number"
            value={duration}
            onChange={(e) => setDuration(e.target.value)}
            fullWidth
            required
            disabled={submitting}
            inputProps={{ min: 1 }}
          />
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography variant="caption" color="text.secondary">
              Preferred times (UTC) — add one or more options
            </Typography>
            {timeslots.map((slot, i) => (
              <Box key={i} sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                <TextField
                  type="datetime-local"
                  value={slot}
                  onChange={(e) => updateTimeslot(i, e.target.value)}
                  fullWidth
                  disabled={submitting}
                  size="small"
                />
                {timeslots.length > 1 && (
                  <Button
                    size="small"
                    color="inherit"
                    onClick={() => removeTimeslot(i)}
                    disabled={submitting}
                    sx={{ minWidth: 0, px: 1 }}
                  >
                    Remove
                  </Button>
                )}
              </Box>
            ))}
            <Button
              size="small"
              variant="text"
              startIcon={<Plus size={14} />}
              onClick={addTimeslot}
              disabled={submitting}
              sx={{ alignSelf: "flex-start", textTransform: "none" }}
            >
              Add another time slot
            </Button>
          </Box>
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
          Submit request
        </Button>
      </DialogActions>
    </Dialog>
  );
}
