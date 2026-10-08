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
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  TextField,
  Typography,
  alpha,
  colors,
} from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useRef, useState, type JSX } from "react";
import { usePatchChangeRequest } from "@features/operations/api/usePatchChangeRequest";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useSuccessBanner } from "@context/success-banner/SuccessBannerContext";
import { resolveDisplayTimeZone, formatBackendTimestampForDisplay } from "@utils/dateTime";
import {
  callRequestApiPreferredTimeToDatetimeLocal,
  computeMinScheduleDatetimeLocalForTimeZone,
} from "@features/support/utils/support";
import {
  describeChangeRequestActionError,
  getCustomerProposal,
  isProposalPending,
} from "@features/operations/utils/changeRequests";
import {
  buildProposedWindowPayload,
  formatPlannedLength,
  getChangeRequestWindow,
  getProposalCopy,
  hasProposedStartErrors,
  shiftEndKeepingDuration,
  validateProposedStart,
} from "@features/operations/utils/changeRequestSchedule";
import type {
  ChangeRequestDetails,
  ProposeNewImplementationTimeModalProps,
} from "@features/operations/types/changeRequests";

const PROPOSE_FORM_ID = "propose-implementation-form";

const SCHEDULE_DISPLAY_OPTIONS: Intl.DateTimeFormatOptions = {
  weekday: "long",
  year: "numeric",
  month: "long",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  timeZoneName: "short",
};

type ProposeNewImplementationTimeModalBodyProps = {
  changeRequest: ChangeRequestDetails;
  onClose: () => void;
  onProposed?: () => void;
  onRefused?: () => void;
};

/**
 * Mounted only while the dialog is open: the start initializes from
 * `changeRequest` on mount (no `setState` in `useEffect`).
 *
 * A proposal is a new START: the planned length stays, so the end is shown, not
 * asked for, and follows the start. Nothing is sent until the start passes
 * validation, and a failure that editing can fix stays in the dialog.
 */
function ProposeNewImplementationTimeModalBody({
  changeRequest,
  onClose,
  onProposed,
  onRefused,
}: ProposeNewImplementationTimeModalBodyProps): JSX.Element {
  const { showError } = useErrorBanner();
  const { showSuccess } = useSuccessBanner();
  const patchMutation = usePatchChangeRequest(changeRequest.id);
  const userTimeZone = resolveDisplayTimeZone();

  const currentStart = callRequestApiPreferredTimeToDatetimeLocal(
    changeRequest.startDate,
    userTimeZone,
  );
  const { durationMs } = getChangeRequestWindow(changeRequest);
  const copy = getProposalCopy(durationMs);
  // A time that already waits for WSO2 (the viewer's own or a colleague's): a new
  // proposal replaces it, and sending the same one again is refused.
  const standing = isProposalPending(changeRequest)
    ? getCustomerProposal(changeRequest)
    : null;
  const standingStart = standing
    ? callRequestApiPreferredTimeToDatetimeLocal(standing.startDate, userTimeZone)
    : "";

  const [proposedStart, setProposedStart] = useState(currentStart);
  const [attempted, setAttempted] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const startInputRef = useRef<HTMLInputElement | null>(null);
  const submitButtonRef = useRef<HTMLButtonElement | null>(null);
  const inFlightRef = useRef(false);
  // Set when a failure keeps the dialog open: the submit button the customer
  // pressed is switched off while the request is in flight, and with it focus
  // leaves the dialog's controls. It is put back once the button is on again.
  const restoreSubmitFocusRef = useRef(false);

  const isModalBusy = patchMutation.isPending;

  useEffect(() => {
    if (!restoreSubmitFocusRef.current || isModalBusy) return;
    restoreSubmitFocusRef.current = false;
    submitButtonRef.current?.focus();
  });
  const minDatetime = computeMinScheduleDatetimeLocalForTimeZone(0, userTimeZone);

  const errors = validateProposedStart({
    start: proposedStart,
    currentStart,
    standingStart: standingStart || undefined,
    durationMs,
    timeZone: userTimeZone,
  });
  // The end that start implies; "" until the start is a real time.
  const derivedEnd = shiftEndKeepingDuration(proposedStart, durationMs, userTimeZone);
  // A change request with no window to move is refused whatever is typed, so that
  // is said at once; the field errors appear once the customer has tried to submit.
  const shown = attempted || durationMs == null ? errors : {};

  const handleClose = () => {
    if (isModalBusy) return;
    onClose();
  };

  const handleStartChange = (value: string) => {
    setProposedStart(value);
    setSubmitError(null);
  };

  const handleSubmit = async () => {
    if (isModalBusy || inFlightRef.current) return;
    setAttempted(true);
    setSubmitError(null);

    if (hasProposedStartErrors(errors)) {
      if (errors.start) startInputRef.current?.focus();
      return;
    }
    const payload = buildProposedWindowPayload(
      proposedStart,
      durationMs,
      userTimeZone,
    );
    if (!payload) {
      setSubmitError("Invalid date and time. Please select valid values.");
      return;
    }

    inFlightRef.current = true;
    try {
      await patchMutation.mutateAsync(payload);
      showSuccess(copy.success);
      onProposed?.();
      onClose();
    } catch (error) {
      const { message, terminal } = describeChangeRequestActionError(
        error,
        "Could not submit your proposal. Please try again.",
      );
      if (terminal) {
        // The change request no longer waits on this customer: nothing left to
        // edit here, so say so on the page and let the refreshed page take over.
        // The page is told first so that it can move focus off the controls that
        // are about to go.
        showError(message);
        onRefused?.();
        onClose();
      } else {
        // The dialog stays: the message is in its alert (announced), and focus
        // goes back to the button that was pressed so the customer can go on.
        setSubmitError(message);
        restoreSubmitFocusRef.current = true;
      }
    } finally {
      inFlightRef.current = false;
    }
  };

  return (
    <Dialog
      open
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="propose-implementation-dialog-title"
      aria-describedby="propose-implementation-dialog-notice"
    >
      {/* The dialog is named by the title text alone (not the subtitle or the
          close button that share this heading), hence the explicit id here. */}
      <DialogTitle
        id="propose-implementation-dialog-heading"
        sx={{
          pr: 6,
          position: "relative",
          display: "flex",
          alignItems: "flex-start",
          justifyContent: "space-between",
        }}
      >
        <Box>
          <Typography
            id="propose-implementation-dialog-title"
            variant="h6"
            component="span"
            display="block"
          >
            Propose New Implementation Time
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            Suggest a different start time for this change request
          </Typography>
        </Box>
        <IconButton
          aria-label="Close"
          onClick={handleClose}
          size="small"
          disabled={isModalBusy}
          sx={{ position: "absolute", right: 8, top: 8 }}
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>
      {/* One form around the content and the actions, so Enter in the field
          submits (the Submit button is tied to it by `form`). Validation stays
          ours: the browser's own bubbles are off. */}
      <form
        id={PROPOSE_FORM_ID}
        noValidate
        // No box of its own: the dialog's scrolling content and its fixed actions
        // stay laid out as children of the dialog.
        style={{ display: "contents" }}
        onSubmit={(event) => {
          event.preventDefault();
          void handleSubmit();
        }}
      >
      <DialogContent sx={{ pt: 1 }}>
        <Alert
          id="propose-implementation-dialog-notice"
          severity="info"
          role="note"
          sx={{ mb: 2 }}
        >
          {copy.notice}
        </Alert>
        {submitError && (
          <Alert severity="error" role="alert" sx={{ mb: 2 }}>
            {submitError}
          </Alert>
        )}
        <Box
          sx={{
            mb: 2,
            p: 2,
            bgcolor: alpha(colors.grey[500], 0.08),
            border: 1,
            borderColor: "divider",
          }}
        >
          <Typography
            variant="subtitle2"
            component="h3"
            color="text.secondary"
            sx={{ mb: 1.5 }}
          >
            Current Schedule
          </Typography>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
              gap: 2,
            }}
          >
            <Box>
              <Typography
                variant="caption"
                color="text.secondary"
                display="block"
              >
                Start Date & Time
              </Typography>
              <Typography variant="body2" color="text.primary">
                {formatBackendTimestampForDisplay(
                  changeRequest.startDate,
                  SCHEDULE_DISPLAY_OPTIONS,
                  userTimeZone,
                ) ?? "Not available"}
              </Typography>
            </Box>
            <Box>
              <Typography
                variant="caption"
                color="text.secondary"
                display="block"
              >
                End Date & Time
              </Typography>
              <Typography variant="body2" color="text.primary">
                {formatBackendTimestampForDisplay(
                  changeRequest.endDate,
                  SCHEDULE_DISPLAY_OPTIONS,
                  userTimeZone,
                ) ?? "Not available"}
              </Typography>
            </Box>
          </Box>
          {standing && (
            <Box sx={{ mt: 2 }}>
              <Typography
                variant="caption"
                color="text.secondary"
                display="block"
              >
                Proposed start waiting for WSO2
              </Typography>
              <Typography variant="body2" color="text.primary">
                {formatBackendTimestampForDisplay(
                  standing.startDate,
                  SCHEDULE_DISPLAY_OPTIONS,
                  userTimeZone,
                ) ?? "Not available"}
              </Typography>
            </Box>
          )}
        </Box>

        <Typography
          variant="subtitle2"
          component="h3"
          color="text.secondary"
          sx={{ mb: 0.5 }}
        >
          Proposed implementation time
        </Typography>
        <Typography
          variant="caption"
          color="text.secondary"
          display="block"
          sx={{ mb: 1.5 }}
        >
          {`Times are in your time zone: ${userTimeZone}.${
            durationMs != null
              ? " The end follows the start: the planned length stays the same."
              : ""
          }`}
        </Typography>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            gap: 2,
            alignItems: "start",
          }}
        >
          <TextField
            id="proposed-start"
            label="Proposed start"
            type="datetime-local"
            size="small"
            fullWidth
            required
            value={proposedStart}
            onChange={(e) => handleStartChange(e.target.value)}
            disabled={isModalBusy || durationMs == null}
            error={Boolean(shown.start)}
            helperText={shown.start}
            inputRef={startInputRef}
            slotProps={{
              inputLabel: { shrink: true },
              htmlInput: { min: minDatetime },
            }}
          />
          {/* Shown, never typed: a proposal moves the start and keeps the planned
              length. Read-only (not disabled), so it keeps its contrast and is read out. */}
          <TextField
            id="proposed-end"
            label="Proposed end"
            type="datetime-local"
            size="small"
            fullWidth
            value={derivedEnd}
            helperText={
              durationMs != null
                ? `Same length as the planned window (${formatPlannedLength(durationMs)})`
                : undefined
            }
            slotProps={{
              inputLabel: { shrink: true },
              input: { readOnly: true },
              htmlInput: { "aria-readonly": true },
            }}
          />
        </Box>
        {shown.window && (
          <Alert severity="warning" role="alert" sx={{ mt: 2 }}>
            {shown.window}
          </Alert>
        )}
      </DialogContent>
      <DialogActions
        sx={{ px: 3, py: 2, borderTop: 1, borderColor: "divider" }}
      >
        <Button
          variant="outlined"
          color="inherit"
          onClick={handleClose}
          disabled={isModalBusy}
        >
          Cancel
        </Button>
        <Button
          type="submit"
          variant="contained"
          color="primary"
          ref={submitButtonRef}
          disabled={isModalBusy || durationMs == null}
          startIcon={
            isModalBusy ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {isModalBusy ? "Submitting..." : "Submit Proposal"}
        </Button>
      </DialogActions>
      </form>
    </Dialog>
  );
}

/**
 * Modal to propose a new implementation start (the planned length stays).
 * Current schedule matches Scheduled Maintenance Window planned start/end.
 *
 * @param props - Dialog state and CR.
 * @returns {JSX.Element} The modal.
 */
export default function ProposeNewImplementationTimeModal({
  open,
  onClose,
  onProposed,
  onRefused,
  changeRequest,
}: ProposeNewImplementationTimeModalProps): JSX.Element | null {
  if (!changeRequest) return null;

  return open ? (
    <ProposeNewImplementationTimeModalBody
      key={changeRequest.id}
      changeRequest={changeRequest}
      onClose={onClose}
      onProposed={onProposed}
      onRefused={onRefused}
    />
  ) : null;
}
