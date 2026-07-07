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
  FormControlLabel,
  Radio,
  RadioGroup,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeCallRequestView } from "@api/backend/types";
import {
  resolveDisplayTimeZone,
  zonedInputToUtcIso,
  normalizeBackendTimestamp,
  formatBackendTimestampForDisplay,
} from "@utils/dateTime";

// ---------------------------------------------------------------------------
// Constants — mirror the backend's schedule contract.
// ---------------------------------------------------------------------------

const MIN_DURATION_MINUTES = 15;
const MAX_DURATION_MINUTES = 240;

/** Sentinel value for the "enter a custom time" radio option. */
const CUSTOM_TIME_VALUE = "__custom__";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface ScheduleCallDialogProps {
  /** The call request being scheduled or rescheduled; null closes the dialog. */
  callRequest: BeCallRequestView | null;
  /** True when this is a reschedule of an already-scheduled request. */
  isReschedule: boolean;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (input: {
    meetingDate: string;
    durationInMinutes: number;
    assignee?: string;
  }) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function ScheduleCallDialog({
  callRequest,
  isReschedule,
  submitting,
  error,
  onClose,
  onSubmit,
}: ScheduleCallDialogProps): JSX.Element {
  const timeZone = resolveDisplayTimeZone();
  const preferredTimes = callRequest?.preferredTimes ?? [];

  const [selectedTime, setSelectedTime] = useState<string>("");
  const [customTime, setCustomTime] = useState<string>("");
  const [duration, setDuration] = useState("30");
  const [assignee, setAssignee] = useState("");
  const [pastError, setPastError] = useState(false);

  // Reset local state whenever the target call request changes (render-time
  // state adjustment, not an effect -- see React docs on this pattern).
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setSelectedTime(preferredTimes[0] ?? CUSTOM_TIME_VALUE);
    setCustomTime("");
    setDuration(callRequest?.durationMin ? String(callRequest.durationMin) : "30");
    setAssignee(callRequest?.assignee ?? "");
    setPastError(false);
  }

  const handleClose = () => {
    setSelectedTime("");
    setCustomTime("");
    setDuration("30");
    setAssignee("");
    setPastError(false);
    onClose();
  };

  const durationNum = parseInt(duration, 10);
  const durationValid =
    !isNaN(durationNum) &&
    durationNum >= MIN_DURATION_MINUTES &&
    durationNum <= MAX_DURATION_MINUTES;

  const customUtcIso =
    selectedTime === CUSTOM_TIME_VALUE && customTime
      ? zonedInputToUtcIso(customTime, timeZone)
      : null;
  const customTimeValid = selectedTime !== CUSTOM_TIME_VALUE || !!customUtcIso;

  // Custom input is entered in the agent's timezone → convert to UTC ISO.
  // A selected preferred time is a backend wall-clock string (e.g. SN's
  // "MM/DD/YYYY HH:MM:SS") → normalize it to RFC3339 UTC; the API requires ISO.
  const meetingDate =
    selectedTime === CUSTOM_TIME_VALUE
      ? customUtcIso
      : selectedTime
        ? normalizeBackendTimestamp(selectedTime)
        : null;

  const canSubmit = durationValid && customTimeValid && !!meetingDate;

  const handleSubmit = () => {
    if (!canSubmit || !meetingDate) return;
    // Reject a time in the past (immediate feedback; the backing data source
    // enforces a stricter lead time). Time is read in the handler, not render.
    if (new Date(meetingDate).getTime() <= Date.now()) {
      setPastError(true);
      return;
    }
    setPastError(false);
    onSubmit({
      meetingDate,
      durationInMinutes: durationNum,
      ...(assignee.trim() ? { assignee: assignee.trim() } : {}),
    });
  };

  return (
    <Dialog open={!!callRequest} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{isReschedule ? "Reschedule call" : "Schedule call"}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {(error || pastError) && (
            <Typography variant="body2" color="error">
              {error ?? "Meeting time must be in the future."}
            </Typography>
          )}

          {preferredTimes.length > 0 && (
            <Box>
              <Typography variant="body2" sx={{ mb: 0.5 }}>
                Choose a time
              </Typography>
              <RadioGroup
                value={selectedTime}
                onChange={(e) => {
                  setSelectedTime(e.target.value);
                  setPastError(false);
                }}
              >
                {preferredTimes.map((t) => (
                  <FormControlLabel
                    key={t}
                    value={t}
                    control={<Radio size="small" disabled={submitting} />}
                    label={
                      formatBackendTimestampForDisplay(t, {
                        year: "numeric",
                        month: "short",
                        day: "numeric",
                        hour: "numeric",
                        minute: "2-digit",
                        timeZoneName: "short",
                      }) ?? t
                    }
                  />
                ))}
                <FormControlLabel
                  value={CUSTOM_TIME_VALUE}
                  control={<Radio size="small" disabled={submitting} />}
                  label="Custom time"
                />
              </RadioGroup>
            </Box>
          )}

          {(preferredTimes.length === 0 || selectedTime === CUSTOM_TIME_VALUE) && (
            <TextField
              label={`Meeting time (${timeZone})`}
              type="datetime-local"
              value={customTime}
              onChange={(e) => {
                setCustomTime(e.target.value);
                setPastError(false);
              }}
              fullWidth
              required
              size="small"
              disabled={submitting}
              // datetime-local always shows a placeholder, so force the label to
              // float up — otherwise it overlaps the mm/dd/yyyy placeholder.
              InputLabelProps={{ shrink: true }}
              error={
                (selectedTime === CUSTOM_TIME_VALUE && !!customTime && !customTimeValid) ||
                pastError
              }
              helperText={
                selectedTime === CUSTOM_TIME_VALUE && customTime && !customTimeValid
                  ? "Invalid date/time."
                  : pastError
                    ? "Meeting time must be in the future."
                    : `Entered in your timezone (${timeZone}); stored as UTC.`
              }
            />
          )}

          <TextField
            label="Duration (minutes)"
            type="number"
            value={duration}
            onChange={(e) => setDuration(e.target.value)}
            fullWidth
            required
            size="small"
            disabled={submitting}
            error={duration !== "" && !durationValid}
            helperText={`Between ${MIN_DURATION_MINUTES} and ${MAX_DURATION_MINUTES} minutes.`}
            inputProps={{ min: MIN_DURATION_MINUTES, max: MAX_DURATION_MINUTES }}
          />

          <TextField
            label="Assignee (optional)"
            placeholder="engineer@example.com"
            value={assignee}
            onChange={(e) => setAssignee(e.target.value)}
            fullWidth
            size="small"
            disabled={submitting}
            helperText="Agent or team assigned to run the call."
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
          {isReschedule ? "Reschedule" : "Schedule"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
