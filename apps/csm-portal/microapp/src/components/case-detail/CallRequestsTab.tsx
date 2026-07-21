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

import { Suspense, useState, type ReactNode } from "react";
import {
  AdapterDateFns,
  Button,
  Chip,
  DatePickers,
  Dialog,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useMutation, useQueryClient, useQueryErrorResetBoundary, useSuspenseQuery } from "@tanstack/react-query";
import { callRequests } from "@src/services/callRequests";
import type { CallRequest, CallRequestUpdateInput, CaseSeverity } from "@src/types";
import { DialogPaper } from "@components/common/DialogPaper";
import { ErrorBoundary } from "@components/common/ErrorBoundary";
import { openUrl } from "@components/microapp-bridge";
import { ErrorState } from "@components/support/ErrorState";
import { formatDate } from "@utils/dateTime";
import {
  CALL_REQUEST_ACTION_LABEL,
  CALL_REQUEST_AGENT_ACTIONS,
  CALL_REQUEST_STATE_COLOR,
  callRequestLeadTimeMinutes,
  formatCallRequestLeadTime,
  resolveCallRequestStateKey,
  type CallRequestAgentAction,
} from "@utils/callRequestState";

const { LocalizationProvider, DateTimePicker } = DatePickers;

export function CallRequestsTab({ caseId, severity }: { caseId: string; severity: CaseSeverity | null }) {
  return (
    <CallRequestsTabErrorBoundary>
      <Suspense fallback={<CallRequestsTabSkeleton />}>
        <CallRequestsTabContent caseId={caseId} severity={severity} />
      </Suspense>
    </CallRequestsTabErrorBoundary>
  );
}

function CallRequestsTabContent({ caseId, severity }: { caseId: string; severity: CaseSeverity | null }) {
  const queryClient = useQueryClient();
  const { data: requests } = useSuspenseQuery(callRequests.forCase(caseId));
  const [createOpen, setCreateOpen] = useState(false);

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: ["case", caseId, "call-requests"] });

  const updateMutation = useMutation({
    mutationFn: (input: CallRequestUpdateInput) => callRequests.update(input),
  });

  // Only one action dialog is ever open at a time, driven by which action was clicked on a row.
  const [scheduleTarget, setScheduleTarget] = useState<CallRequest | null>(null);
  const [rejectTarget, setRejectTarget] = useState<CallRequest | null>(null);
  const [notesTarget, setNotesTarget] = useState<CallRequest | null>(null);
  const [cancelTarget, setCancelTarget] = useState<CallRequest | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const isReschedule = resolveCallRequestStateKey(scheduleTarget?.state) === "scheduled";

  const handleAction = (action: CallRequestAgentAction, cr: CallRequest) => {
    setActionError(null);
    switch (action) {
      case "schedule":
      case "reschedule":
        setScheduleTarget(cr);
        break;
      case "reject":
        setRejectTarget(cr);
        break;
      case "sendNotes":
        setNotesTarget(cr);
        break;
      case "cancel":
        setCancelTarget(cr);
        break;
    }
  };

  const runUpdate = (input: CallRequestUpdateInput, onDone: () => void) => {
    setActionError(null);
    updateMutation.mutate(input, {
      onSuccess: () => {
        onDone();
        invalidate();
      },
      onError: () => setActionError("Could not update the call request. Please try again."),
    });
  };

  const handleSchedule = (input: { meetingDate: string; durationInMinutes: number; assignee?: string }) => {
    if (!scheduleTarget) return;
    runUpdate({ caseId, callRequestId: scheduleTarget.id, state: "scheduled", ...input }, () =>
      setScheduleTarget(null),
    );
  };

  const handleReject = (reason?: string) => {
    if (!rejectTarget) return;
    runUpdate({ caseId, callRequestId: rejectTarget.id, state: "wso2_rejected", cancellationReason: reason }, () =>
      setRejectTarget(null),
    );
  };

  const handleCancel = (cancellationReason: string) => {
    if (!cancelTarget) return;
    runUpdate({ caseId, callRequestId: cancelTarget.id, state: "canceled", cancellationReason }, () =>
      setCancelTarget(null),
    );
  };

  const handleSendNotes = (input: {
    notes: string;
    plan?: string;
    attendees?: string;
    actionItems?: string;
    actualDurationMin?: number;
  }) => {
    if (!notesTarget) return;
    runUpdate({ caseId, callRequestId: notesTarget.id, state: "concluded", ...input }, () => setNotesTarget(null));
  };

  return (
    <Stack gap={2}>
      <Button variant="contained" size="small" onClick={() => setCreateOpen(true)} sx={{ alignSelf: "start" }}>
        Request a call
      </Button>

      {requests.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No call requests on this case.
        </Typography>
      ) : (
        <Stack gap={1}>
          {requests.map((cr) => (
            <CallRequestRow key={cr.id} callRequest={cr} onAction={handleAction} />
          ))}
        </Stack>
      )}

      {createOpen && (
        <CreateCallRequestDialog
          caseId={caseId}
          severity={severity}
          onClose={() => setCreateOpen(false)}
          onCreated={() => {
            setCreateOpen(false);
            invalidate();
          }}
        />
      )}

      <ScheduleCallDialog
        callRequest={scheduleTarget}
        isReschedule={isReschedule}
        submitting={updateMutation.isPending}
        error={actionError}
        onClose={() => {
          setScheduleTarget(null);
          setActionError(null);
        }}
        onSubmit={handleSchedule}
      />

      <RejectCallDialog
        callRequest={rejectTarget}
        submitting={updateMutation.isPending}
        error={actionError}
        onClose={() => {
          setRejectTarget(null);
          setActionError(null);
        }}
        onSubmit={handleReject}
      />

      <CancelCallDialog
        callRequest={cancelTarget}
        submitting={updateMutation.isPending}
        error={actionError}
        onClose={() => {
          setCancelTarget(null);
          setActionError(null);
        }}
        onSubmit={handleCancel}
      />

      <SendCallNotesDialog
        callRequest={notesTarget}
        submitting={updateMutation.isPending}
        error={actionError}
        onClose={() => {
          setNotesTarget(null);
          setActionError(null);
        }}
        onSubmit={handleSendNotes}
      />
    </Stack>
  );
}

function CallRequestRow({
  callRequest,
  onAction,
}: {
  callRequest: CallRequest;
  onAction: (action: CallRequestAgentAction, cr: CallRequest) => void;
}) {
  const stateKey = resolveCallRequestStateKey(callRequest.state);
  const actions = stateKey ? CALL_REQUEST_AGENT_ACTIONS[stateKey] : [];

  return (
    <Stack gap={0.5} sx={{ p: 1.5, border: "1px solid", borderColor: "divider", borderRadius: 1 }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
        <Typography variant="body2" fontWeight={500} noWrap sx={{ minWidth: 0 }}>
          {callRequest.number}
        </Typography>
        <Chip
          size="small"
          label={callRequest.stateLabel}
          color={stateKey ? CALL_REQUEST_STATE_COLOR[stateKey] : "default"}
        />
      </Stack>
      <Typography variant="body2" color="text.primary">
        {callRequest.reason}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {callRequest.durationMin} min
        {callRequest.scheduleTime ? ` · Scheduled ${formatDate(callRequest.scheduleTime)}` : ""}
      </Typography>
      {callRequest.assignee && (
        <Typography variant="caption" color="text.secondary">
          Assignee: {callRequest.assignee}
        </Typography>
      )}
      {callRequest.meetingLink && (
        <Typography
          variant="caption"
          color="primary.main"
          noWrap
          onClick={() => openUrl({ url: callRequest.meetingLink as string, presentationStyle: "fullScreen" })}
          sx={{ cursor: "pointer" }}
        >
          {callRequest.meetingLink}
        </Typography>
      )}
      {callRequest.notes && (
        <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: "pre-line" }}>
          Notes: {callRequest.notes}
        </Typography>
      )}
      {actions.length > 0 && (
        <Stack direction="row" gap={1} flexWrap="wrap" pt={0.5}>
          {actions.map((action) => (
            <Button
              key={action}
              size="small"
              variant="outlined"
              color={action === "reject" || action === "cancel" ? "error" : "primary"}
              onClick={() => onAction(action, callRequest)}
            >
              {CALL_REQUEST_ACTION_LABEL[action]}
            </Button>
          ))}
        </Stack>
      )}
    </Stack>
  );
}

function CreateCallRequestDialog({
  caseId,
  severity,
  onClose,
  onCreated,
}: {
  caseId: string;
  severity: CaseSeverity | null;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [reason, setReason] = useState("");
  const [preferredTime, setPreferredTime] = useState<Date | null>(null);
  // Kept as the raw typed string — same pattern as the date picker's own `preferredTime` state
  // (the widget's natural value, unmodified as you interact with it). Converting and clamping on
  // every keystroke (the previous approach) snapped the field back to "1" the instant it was
  // cleared to retype, making it impossible to type a new value normally.
  const [durationInput, setDurationInput] = useState("30");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // ServiceNow rejects a preferred time that's too soon, with a plain 400 the
  // backend genericizes to "Invalid request payload." — pre-validate the same
  // lead-time rule the webapp does so this fails in the dialog instead.
  const leadMinutes = callRequestLeadTimeMinutes(severity);
  const minAllowedTime = new Date(Date.now() + leadMinutes * 60_000);
  const isTimeValid = preferredTime !== null && preferredTime.getTime() >= minAllowedTime.getTime();

  const durationMinutes = Number(durationInput);
  const isDurationValid = durationInput.trim().length > 0 && Number.isFinite(durationMinutes) && durationMinutes > 0;
  const canSubmit = reason.trim().length > 0 && isTimeValid && isDurationValid && !isSubmitting;

  const handleSubmit = () => {
    if (!canSubmit || !preferredTime) return;
    setIsSubmitting(true);
    setError(null);
    callRequests
      .create(caseId, {
        reason: reason.trim(),
        utcTimes: [preferredTime.toISOString()],
        durationInMinutes: durationMinutes,
      })
      .then(onCreated)
      .catch(() => setError("Could not request the call. Please try again."))
      .finally(() => setIsSubmitting(false));
  };

  return (
    <Dialog
      open
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Request a call
      </Typography>

      <TextField
        label="Reason"
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
      />

      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DateTimePicker
          label="Preferred time"
          value={preferredTime}
          onChange={setPreferredTime}
          minDateTime={minAllowedTime}
          slotProps={{
            textField: {
              size: "small",
              fullWidth: true,
              error: preferredTime !== null && !isTimeValid,
              helperText: `Must be at least ${formatCallRequestLeadTime(leadMinutes)} from now.`,
            },
          }}
        />
      </LocalizationProvider>

      <TextField
        label="Duration (minutes)"
        type="number"
        size="small"
        fullWidth
        value={durationInput}
        onChange={(e) => setDurationInput(e.target.value)}
        error={durationInput.trim().length > 0 && !isDurationValid}
        slotProps={{ htmlInput: { min: 1 } }}
      />

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSubmit} onClick={handleSubmit}>
          Send
        </Button>
      </Stack>
    </Dialog>
  );
}

function ScheduleCallDialog({
  callRequest,
  isReschedule,
  submitting,
  error,
  onClose,
  onSubmit,
}: {
  callRequest: CallRequest | null;
  isReschedule: boolean;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (input: { meetingDate: string; durationInMinutes: number; assignee?: string }) => void;
}) {
  const [meetingTime, setMeetingTime] = useState<Date | null>(null);
  const [durationInput, setDurationInput] = useState("30");
  const [assignee, setAssignee] = useState("");

  // Reset local state whenever the target call request changes (render-time state
  // adjustment, not an effect — matches the pattern already used in CreateCallRequestDialog's
  // sibling dialogs on the webapp).
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setMeetingTime(null);
    setDurationInput(callRequest?.durationMin ? String(callRequest.durationMin) : "30");
    setAssignee(callRequest?.assignee ?? "");
  }

  const durationMinutes = Number(durationInput);
  const isDurationValid = durationInput.trim().length > 0 && Number.isFinite(durationMinutes) && durationMinutes > 0;
  const canSubmit = meetingTime !== null && isDurationValid && !submitting;

  const handleSubmit = () => {
    if (!canSubmit || !meetingTime) return;
    onSubmit({
      meetingDate: meetingTime.toISOString(),
      durationInMinutes: durationMinutes,
      ...(assignee.trim() ? { assignee: assignee.trim() } : {}),
    });
  };

  const preferredTimes = callRequest?.preferredTimes ?? [];

  return (
    <Dialog
      open={!!callRequest}
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        {isReschedule ? "Reschedule call" : "Schedule call"}
      </Typography>

      {preferredTimes.length > 0 && (
        <Typography variant="caption" color="text.secondary">
          Customer's preferred times: {preferredTimes.map((t) => formatDate(new Date(t))).join(", ")}
        </Typography>
      )}

      <LocalizationProvider dateAdapter={AdapterDateFns}>
        <DateTimePicker
          label="Meeting time"
          value={meetingTime}
          onChange={setMeetingTime}
          disabled={submitting}
          slotProps={{ textField: { size: "small", fullWidth: true } }}
        />
      </LocalizationProvider>

      <TextField
        label="Duration (minutes)"
        type="number"
        size="small"
        fullWidth
        value={durationInput}
        onChange={(e) => setDurationInput(e.target.value)}
        disabled={submitting}
        error={durationInput.trim().length > 0 && !isDurationValid}
        slotProps={{ htmlInput: { min: 1 } }}
      />

      <TextField
        label="Assignee (optional)"
        placeholder="engineer@example.com"
        size="small"
        fullWidth
        value={assignee}
        onChange={(e) => setAssignee(e.target.value)}
        disabled={submitting}
      />

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSubmit} onClick={handleSubmit}>
          {isReschedule ? "Reschedule" : "Schedule"}
        </Button>
      </Stack>
    </Dialog>
  );
}

function RejectCallDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: {
  callRequest: CallRequest | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (reason?: string) => void;
}) {
  const [reason, setReason] = useState("");
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setReason("");
  }

  return (
    <Dialog
      open={!!callRequest}
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Reject call request
      </Typography>
      <Typography variant="body2" color="text.secondary">
        This declines the request on behalf of the team. The customer will see it as rejected.
      </Typography>

      <TextField
        label="Reason (optional)"
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
        disabled={submitting}
      />

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="error"
          disabled={submitting}
          onClick={() => onSubmit(reason.trim() ? reason.trim() : undefined)}
        >
          Reject
        </Button>
      </Stack>
    </Dialog>
  );
}

function CancelCallDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: {
  callRequest: CallRequest | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (cancellationReason: string) => void;
}) {
  const [reason, setReason] = useState("");
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setReason("");
  }

  const canSubmit = reason.trim().length > 0 && !submitting;

  return (
    <Dialog
      open={!!callRequest}
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Cancel call request
      </Typography>

      <TextField
        label="Cancellation reason"
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
        disabled={submitting}
      />

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={submitting}>
          Back
        </Button>
        <Button variant="contained" color="error" disabled={!canSubmit} onClick={() => onSubmit(reason.trim())}>
          Cancel request
        </Button>
      </Stack>
    </Dialog>
  );
}

function SendCallNotesDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: {
  callRequest: CallRequest | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (input: {
    notes: string;
    plan?: string;
    attendees?: string;
    actionItems?: string;
    actualDurationMin?: number;
  }) => void;
}) {
  const [notes, setNotes] = useState("");
  const [actionItems, setActionItems] = useState("");
  const [plan, setPlan] = useState("");
  const [attendees, setAttendees] = useState("");
  const [actualDurationInput, setActualDurationInput] = useState("");

  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setNotes("");
    setActionItems("");
    setPlan("");
    setAttendees("");
    setActualDurationInput(callRequest?.durationMin ? String(callRequest.durationMin) : "");
  }

  const actualDurationMin = actualDurationInput.trim() === "" ? undefined : Number(actualDurationInput);
  const isDurationValid =
    actualDurationMin === undefined || (Number.isFinite(actualDurationMin) && actualDurationMin > 0);
  const canSubmit = notes.trim().length > 0 && isDurationValid && !submitting;

  const handleSubmit = () => {
    if (!canSubmit) return;
    onSubmit({
      notes: notes.trim(),
      ...(plan.trim() ? { plan: plan.trim() } : {}),
      ...(attendees.trim() ? { attendees: attendees.trim() } : {}),
      ...(actionItems.trim() ? { actionItems: actionItems.trim() } : {}),
      ...(actualDurationMin !== undefined ? { actualDurationMin } : {}),
    });
  };

  return (
    <Dialog
      open={!!callRequest}
      onClose={onClose}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 2, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Send call notes
      </Typography>
      <Typography variant="body2" color="text.secondary">
        Submitting notes concludes this call request.
      </Typography>

      <TextField
        label="Notes"
        multiline
        minRows={3}
        fullWidth
        size="small"
        placeholder="Summary of what was discussed..."
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
        disabled={submitting}
      />

      <TextField
        label="Action items (optional)"
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={actionItems}
        onChange={(e) => setActionItems(e.target.value)}
        disabled={submitting}
      />

      <TextField
        label="Plan (optional)"
        multiline
        minRows={2}
        fullWidth
        size="small"
        value={plan}
        onChange={(e) => setPlan(e.target.value)}
        disabled={submitting}
      />

      <TextField
        label="Attendees (optional)"
        placeholder="Comma-separated names or emails"
        fullWidth
        size="small"
        value={attendees}
        onChange={(e) => setAttendees(e.target.value)}
        disabled={submitting}
      />

      <TextField
        label="Actual duration (minutes, optional)"
        type="number"
        fullWidth
        size="small"
        value={actualDurationInput}
        onChange={(e) => setActualDurationInput(e.target.value)}
        disabled={submitting}
        error={actualDurationInput.trim().length > 0 && !isDurationValid}
        slotProps={{ htmlInput: { min: 1 } }}
      />

      {error && (
        <Typography variant="caption" color="error.main">
          {error}
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button variant="contained" disabled={!canSubmit} onClick={handleSubmit}>
          Send notes
        </Button>
      </Stack>
    </Dialog>
  );
}

function CallRequestsTabSkeleton() {
  return (
    <Stack gap={1}>
      <Skeleton variant="rounded" height={36} width={140} />
      <Skeleton variant="rounded" height={90} />
    </Stack>
  );
}

function CallRequestsTabErrorBoundary({ children }: { children: ReactNode }) {
  const { reset } = useQueryErrorResetBoundary();

  return (
    <ErrorBoundary
      fallback={(_error, resetErrorBoundary) => (
        <ErrorState
          onRetry={() => {
            reset();
            resetErrorBoundary();
          }}
        />
      )}
    >
      {children}
    </ErrorBoundary>
  );
}
