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
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import {
  resolveDisplayTimeZone,
  utcMsToZonedInputValue,
  zonedInputToUtcIso,
} from "@utils/dateTime";

// ---------------------------------------------------------------------------
// Constants — mirror the backend's call-request contract so we fail in the
// dialog instead of round-tripping to a generic 400.
// ---------------------------------------------------------------------------

const MIN_DURATION_MINUTES = 15;
const MAX_DURATION_MINUTES = 240;
const MAX_TIME_SLOTS = 3;

/**
 * Minimum lead time (minutes) each proposed slot must be in the future, keyed by
 * the UI severity scale (S0-S4). Hardcoded mirror of the ServiceNow rule until
 * the backend exposes it as a contract. Composed from three backend maps:
 *   UI Severity -> BeCaseSeverity (mappers.priorityFromSeverity):
 *     S0=catastrophic, S1=critical, S2=high, S3=medium, S4=low
 *   BeCaseSeverity -> SN priority id (entity-service snSeverityIDMap):
 *     catastrophic=14, critical=10, high=11, medium=12, low=13
 *   SN priority id -> offset (SN CallRequestUtils._PRIORITY_TIME_OFFSETS):
 *     14=15, 10=30, 11=60, 12=90, 13=120; null/unknown -> 300
 * Caveat: the mapper collapses null-priority cases to S3, so a genuinely
 * null-priority case is enforced at 90 min here vs 300 min at the backend
 * (lenient: it may still 400, but never falsely blocks a valid time).
 * If these backend maps change, this table must change with them.
 */
const LEAD_TIME_MINUTES_BY_SEVERITY: Record<Severity, number> = {
  S0: 15,
  S1: 30,
  S2: 60,
  S3: 90,
  S4: 120,
};
const DEFAULT_LEAD_TIME_MINUTES = 300;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Human-readable lead time, e.g. 300 -> "5 hours", 90 -> "90 minutes". */
function formatLeadTime(minutes: number): string {
  if (minutes % 60 === 0) {
    const hours = minutes / 60;
    return `${hours} hour${hours === 1 ? "" : "s"}`;
  }
  return `${minutes} minutes`;
}

/** Earliest acceptable instant (epoch ms) given the required lead time. */
function earliestAllowedMs(leadMinutes: number): number {
  return Date.now() + leadMinutes * 60_000;
}

/** UTC instant (epoch ms) for a datetime-local value interpreted in `timeZone`. */
function slotUtcMs(localValue: string, timeZone: string): number {
  const iso = zonedInputToUtcIso(localValue, timeZone);
  return iso ? new Date(iso).getTime() : NaN;
}

/**
 * Validation + UTC-preview state for a single slot. The datetime-local value is
 * interpreted in the user's timezone; we show the resolved UTC instant so the
 * conversion is explicit.
 */
function slotStatus(
  localValue: string,
  timeZone: string,
  minAllowedMs: number,
  leadMinutes: number,
): { error?: string; hint?: string } {
  if (!localValue) return {};
  const ms = slotUtcMs(localValue, timeZone);
  if (Number.isNaN(ms)) return { error: "Invalid date/time." };
  if (ms < minAllowedMs) {
    return {
      error: `Must be at least ${formatLeadTime(leadMinutes)} from now for this case severity.`,
    };
  }
  const utc = new Date(ms).toISOString().slice(0, 16).replace("T", " ");
  return { hint: `= ${utc} UTC` };
}

/** Filled slots mapped to future (>= lead time), de-duplicated UTC ISO instants. */
function toFutureUtcTimes(
  filledSlots: string[],
  timeZone: string,
  minAllowedMs: number,
): string[] {
  return Array.from(
    new Set(
      filledSlots
        .map((v) => slotUtcMs(v, timeZone))
        .filter((ms) => !Number.isNaN(ms) && ms >= minAllowedMs)
        .map((ms) => new Date(ms).toISOString()),
    ),
  );
}

/** True if any filled slot is unparseable or earlier than the required lead time. */
function hasInvalidFutureSlot(
  filledSlots: string[],
  timeZone: string,
  minAllowedMs: number,
): boolean {
  return filledSlots.some((v) => {
    const ms = slotUtcMs(v, timeZone);
    return Number.isNaN(ms) || ms < minAllowedMs;
  });
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CreateDialogProps {
  open: boolean;
  submitting: boolean;
  error: string | null;
  /** Case severity (S0-S4) — drives the minimum lead time for each proposed slot. */
  severity?: Severity;
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
  severity,
  onClose,
  onSubmit,
}: CreateDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  const [duration, setDuration] = useState("30");
  // Each entry is a datetime-local string entered in the user's timezone.
  const [timeslots, setTimeslots] = useState<string[]>([""]);

  const handleClose = () => {
    setReason("");
    setDuration("30");
    setTimeslots([""]);
    onClose();
  };

  const addTimeslot = () =>
    setTimeslots((prev) => (prev.length >= MAX_TIME_SLOTS ? prev : [...prev, ""]));
  const removeTimeslot = (i: number) =>
    setTimeslots((prev) => prev.filter((_, idx) => idx !== i));
  const updateTimeslot = (i: number, val: string) =>
    setTimeslots((prev) => prev.map((v, idx) => (idx === i ? val : v)));

  // Times are entered in the user's timezone and stored/submitted as UTC.
  const timeZone = resolveDisplayTimeZone();
  const leadMinutes = severity
    ? LEAD_TIME_MINUTES_BY_SEVERITY[severity]
    : DEFAULT_LEAD_TIME_MINUTES;
  const minAllowedMs = earliestAllowedMs(leadMinutes);
  const minLocal = utcMsToZonedInputValue(minAllowedMs, timeZone);

  const filledSlots = timeslots.filter(Boolean);
  const durationNum = parseInt(duration, 10);

  // Future (>= lead time), de-duplicated UTC instants — the exact payload sent.
  const utcTimes = toFutureUtcTimes(filledSlots, timeZone, minAllowedMs);
  const hasInvalidSlot = hasInvalidFutureSlot(filledSlots, timeZone, minAllowedMs);

  const durationValid =
    !isNaN(durationNum) &&
    durationNum >= MIN_DURATION_MINUTES &&
    durationNum <= MAX_DURATION_MINUTES;

  const canSubmit =
    reason.trim().length > 0 &&
    utcTimes.length > 0 &&
    utcTimes.length <= MAX_TIME_SLOTS &&
    !hasInvalidSlot &&
    durationValid;

  const handleSubmit = () => {
    if (!canSubmit) return;
    onSubmit(reason.trim(), utcTimes, durationNum);
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
            error={duration !== "" && !durationValid}
            helperText={`Between ${MIN_DURATION_MINUTES} and ${MAX_DURATION_MINUTES} minutes.`}
            inputProps={{ min: MIN_DURATION_MINUTES, max: MAX_DURATION_MINUTES }}
          />
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            <Typography variant="caption" color="text.secondary">
              Preferred times — enter in your timezone ({timeZone}); stored as
              UTC. Each must be at least {formatLeadTime(leadMinutes)} from now.
              Add up to {MAX_TIME_SLOTS} options.
            </Typography>
            {timeslots.map((slot, i) => {
              const status = slotStatus(slot, timeZone, minAllowedMs, leadMinutes);
              return (
                <Box
                  key={i}
                  sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}
                >
                  <TextField
                    type="datetime-local"
                    value={slot}
                    onChange={(e) => updateTimeslot(i, e.target.value)}
                    fullWidth
                    disabled={submitting}
                    size="small"
                    error={Boolean(status.error)}
                    helperText={status.error ?? status.hint ?? " "}
                    inputProps={{ min: minLocal }}
                  />
                  {timeslots.length > 1 && (
                    <Button
                      size="small"
                      color="inherit"
                      onClick={() => removeTimeslot(i)}
                      disabled={submitting}
                      sx={{ minWidth: 0, px: 1, mt: 0.5 }}
                    >
                      Remove
                    </Button>
                  )}
                </Box>
              );
            })}
            {timeslots.length < MAX_TIME_SLOTS && (
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
            )}
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
