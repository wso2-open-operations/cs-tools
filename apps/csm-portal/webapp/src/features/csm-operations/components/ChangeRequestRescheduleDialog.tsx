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
  AdapterDateFns,
  Alert,
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import type {
  BeChangeRequestCustomerProposal,
  BeChangeRequestDetail,
  BePatchChangeRequestPayload,
} from "@api/backend/types";
import {
  customerProposalProposer,
  customerProposalWording,
  formatCrWindow,
  proposedWindowMs,
} from "@features/csm-operations/utils/changeRequests";
import {
  backendUtcToZonedInput,
  formatDateTimeLocal,
  parseBackendTimestamp,
  parseDateTimeLocal,
  zonedInputToBackendUtc,
} from "@utils/dateTime";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface ChangeRequestRescheduleDialogProps {
  cr: BeChangeRequestDetail;
  /**
   * The time stored on the change while it waits in Customer Approval. With a customer recorded as its
   * proposer the dialog is WSO2's COUNTER ("Propose a different time", or a decline that keeps the
   * current time) instead of a plain Re-schedule. With nobody recorded as having proposed it there is
   * no proposal to decline: the dialog is a plain Re-schedule (a changed window is required) that names
   * the stored time ("Propose a different time"). Absent = Re-schedule.
   */
  proposal?: BeChangeRequestCustomerProposal | null;
  /** True while the PATCH is in flight. */
  isSubmitting: boolean;
  /** The backend's refusal for the last attempt, shown verbatim. */
  error?: string | null;
  /**
   * True when the page now holds something other than what this dialog was opened on (the planned
   * window, the proposal or the state moved) and an attempt was refused: the same request would be
   * refused again, so submit is held back and the dialog says to close it and look at the current
   * state. Only set after a refusal: until then the dialog keeps what its reader was shown, and the
   * backend refuses a request for a version that has moved in words.
   */
  stale?: boolean;
  onClose: () => void;
  /**
   * `{state: "authorize", plannedStartOn?, plannedEndOn?}` plus the optional
   * reason ("" when none), which the caller records as an internal comment once the
   * change has been updated (never before: a refused attempt leaves no note behind, so a
   * retry or a reopened dialog cannot post it twice). Answering a
   * proposal adds `expectedCustomerUpdatedOn` (the proposal this page showed) and the
   * planned window it showed, so a change that moved behind the dialog is refused in
   * words instead of answering a time its reader never saw.
   */
  onSubmit: (patch: BePatchChangeRequestPayload, reason: string) => void;
}

/**
 * The Time Change loop out of Customer Approval, in two modes. The wire name is
 * `{state: "authorize"}` in both, but the change NEVER leaves Customer Approval and never
 * goes back through CAB: the change itself has not changed.
 *
 *  - RE-SCHEDULE (no proposal waiting): WSO2 changes the planned time and the customer is
 *    asked to approve it. Collects the new planned start and/or end -- prefilled with the
 *    current values, at least one must change -- and an optional reason. Only the changed
 *    dates are sent; the backend repeats the check ("re-scheduling requires a changed planned
 *    start or end") and its refusal is shown as returned.
 *  - COUNTER (a customer's proposal waiting): "Propose a different time" -- the previous system's
 *    "Disagree". Prefilled with the PLANNED window; any window but the very one the customer
 *    proposed can be sent (that one is "Accept proposed time", in the banner). Leaving the
 *    window as it is declines the proposal: the customer keeps their request to approve the
 *    current time. A different window answers the proposal and asks the customer again.
 *  - A STORED TIME nobody is recorded as having proposed: also titled "Propose a different time", but
 *    it is a plain Re-schedule. Nothing was proposed, so there is nothing to decline and the window
 *    must change; the stored time may be named as the new window (WSO2 then asks the customer to
 *    approve it). The request still names the stored time and the planned window the page showed,
 *    so one that moved is refused in words instead of acted on.
 */
export default function ChangeRequestRescheduleDialog({
  cr,
  proposal,
  isSubmitting,
  error,
  stale,
  onClose,
  onSubmit,
}: ChangeRequestRescheduleDialogProps): JSX.Element {
  const proposer = proposal ? customerProposalProposer(proposal) : null;
  // A customer's proposal waits for WSO2's answer: the counter / decline mode.
  const counter = !!proposal && !!proposer;
  // A time is stored but nobody is recorded as having proposed it: a plain Re-schedule that names it.
  const storedTime = !!proposal && !proposer;
  const initialStart = useMemo(() => backendUtcToZonedInput(cr.plannedStartOn), [cr.plannedStartOn]);
  const initialEnd = useMemo(() => backendUtcToZonedInput(cr.plannedEndOn), [cr.plannedEndOn]);
  // The pickers' own values are kept as emitted, partial (Invalid Date) ones
  // included: feeding a half-typed field back as null makes the picker wipe the
  // digits typed so far.
  const [startDate, setStartDate] = useState<Date | null>(() => parseDateTimeLocal(initialStart));
  const [endDate, setEndDate] = useState<Date | null>(() => parseDateTimeLocal(initialEnd));
  const [reason, setReason] = useState("");

  const validLocal = (d: Date | null): string => (d && !Number.isNaN(d.getTime()) ? formatDateTimeLocal(d) : "");
  const plannedStart = validLocal(startDate);
  const plannedEnd = validLocal(endDate);
  const startChanged = !!plannedStart && plannedStart !== initialStart;
  const endChanged = !!plannedEnd && plannedEnd !== initialEnd;
  const endBeforeStart = !!plannedStart && !!plannedEnd && endDate!.getTime() <= startDate!.getTime();

  const changedStartUtc = startChanged ? zonedInputToBackendUtc(plannedStart) : null;
  const changedEndUtc = endChanged ? zonedInputToBackendUtc(plannedEnd) : null;

  // Counter mode: the window WSO2 would send (what is changed, else what is planned) against the one
  // the customer proposed. The very same window is the Accept action's, not a counter.
  const proposed = proposal ? proposedWindowMs(cr, proposal) : null;
  // "The customer proposed ..." only when the proposer is on record; otherwise the dialog says a time is stored and nobody proposed it.
  const wording = customerProposalWording(proposer);
  const instantOf = (utc: string | null, planned: string | null | undefined): number | null =>
    (utc ? parseBackendTimestamp(utc) : parseBackendTimestamp(planned))?.getTime() ?? null;
  const effectiveStartMs = instantOf(changedStartUtc, cr.plannedStartOn);
  const effectiveEndMs = instantOf(changedEndUtc, cr.plannedEndOn);
  const isTheCustomersTime =
    counter &&
    !!proposed &&
    effectiveStartMs === proposed.startMs &&
    (proposed.endMs === null || effectiveEndMs === proposed.endMs);
  const keepsCurrentTime = counter && !startChanged && !endChanged;

  // A counter may leave the window as it is (a decline); a Re-schedule, a stored time included, must change it.
  const canSubmit =
    !stale &&
    (counter
      ? !endBeforeStart && !isTheCustomersTime && !isSubmitting
      : (startChanged || endChanged) && !endBeforeStart && !isSubmitting);

  const submit = (): void => {
    const patch: BePatchChangeRequestPayload = { state: "authorize" };
    if (changedStartUtc) patch.plannedStartOn = changedStartUtc;
    if (changedEndUtc) patch.plannedEndOn = changedEndUtc;
    if (proposal) {
      patch.expectedCustomerUpdatedOn = proposal.startOn;
      if (cr.plannedStartOn) patch.expectedPlannedStartOn = cr.plannedStartOn;
      if (cr.plannedEndOn) patch.expectedPlannedEndOn = cr.plannedEndOn;
    }
    onSubmit(patch, reason.trim());
  };

  const title = proposal ? "Propose a different time" : "Re-schedule this change?";
  const submitLabel = counter ? (keepsCurrentTime ? "Decline proposed time" : "Propose this time") : storedTime ? "Propose this time" : "Re-schedule";

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth aria-labelledby="cr-reschedule-title">
      <DialogTitle id="cr-reschedule-title">{title}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Alert severity="error" role="alert">
              {error}
            </Alert>
          )}
          {stale && (
            <Typography variant="caption" color="text.secondary" role="status">
              This change request changed while this dialog was open, so the same request would be refused again. Close
              this dialog to see the current state.
            </Typography>
          )}
          {counter && proposal ? (
            <Typography variant="body2" color="text.secondary">
              {`${wording.counterLead(formatCrWindow(proposal.startOn, proposed?.endMs ?? null))} ` +
                "Set the time WSO2 proposes instead and the customer is asked to approve it. " +
                "Keep the current time to decline the proposal. No CAB approval is needed."}
            </Typography>
          ) : storedTime && proposal ? (
            <Typography variant="body2" color="text.secondary">
              {`${wording.counterLead(formatCrWindow(proposal.startOn, proposed?.endMs ?? null))} ` +
                "Set the time WSO2 proposes and the customer is asked to approve it. No CAB approval is needed."}
            </Typography>
          ) : (
            <Typography variant="body2" color="text.secondary">
              Set the new planned time. The customer is asked to approve it. No further internal approval is needed: the
              change itself has not changed.
            </Typography>
          )}
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="Planned start"
              value={startDate}
              disabled={isSubmitting}
              onChange={(next) => setStartDate(next instanceof Date ? next : null)}
              slotProps={{ textField: { size: "small", fullWidth: true } }}
            />
            <DateTimePicker
              label="Planned end"
              value={endDate}
              disabled={isSubmitting}
              onChange={(next) => setEndDate(next instanceof Date ? next : null)}
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  error: endBeforeStart,
                  helperText: endBeforeStart ? "Planned end must be after planned start." : undefined,
                },
              }}
            />
          </LocalizationProvider>
          {!counter && !startChanged && !endChanged && (
            <Typography variant="caption" color="text.secondary">
              {storedTime ? "Change the planned start or end to propose a time." : "Change the planned start or end to re-schedule."}
            </Typography>
          )}
          {isTheCustomersTime && (
            <Typography variant="caption" color="warning.main" role="status">
              {wording.isTheProposedTimeNote}
            </Typography>
          )}
          {keepsCurrentTime && !isTheCustomersTime && (
            <Typography variant="caption" color="text.secondary">
              The current time stays, so the proposal is declined. The customer is not asked again: their request to
              approve the current time stays open.
            </Typography>
          )}
          <TextField
            label="Reason (optional)"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isSubmitting}
            multiline
            minRows={2}
            fullWidth
            size="small"
            helperText="Recorded as an internal work note once the change has been updated."
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Close
        </Button>
        <Button variant="contained" onClick={submit} disabled={!canSubmit} loading={isSubmitting}>
          {submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
