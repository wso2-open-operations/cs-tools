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
  Card,
  Chip,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Typography,
} from "@wso2/oxygen-ui";
import { Phone, Plus, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useState, type JSX } from "react";
import type { BeCallRequestView, BeCallRequestStateKey } from "@api/backend/types";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import {
  useGetCsmCaseCallRequests,
  usePostCsmCaseCallRequest,
  usePatchCsmCaseCallRequest,
} from "@features/csm-cases/api/useCsmCaseCallRequests";
import {
  ALL_CALL_REQUEST_STATES,
  CALL_REQUEST_STATE_LABEL,
  type CallRequestAgentAction,
  resolveCallRequestStateKey,
} from "@features/csm-cases/utils/callRequestState";
import { CreateCallRequestDialog } from "./CreateCallRequestDialog";
import { ScheduleCallDialog } from "./ScheduleCallDialog";
import { RejectCallDialog } from "./RejectCallDialog";
import { SendCallNotesDialog } from "./SendCallNotesDialog";
import { CancelCallDialog } from "./CancelCallDialog";
import { CallRequestRow } from "./CallRequestRow";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface CallRequestsWidgetProps {
  caseId: string;
  /** Case severity (S0-S4) — passed to the create dialog to enforce the lead-time rule. */
  severity?: Severity;
  /** True to pop the "Create call request" dialog from outside the widget
   * (e.g. the case action bar's "Request a call" item). One-shot: the
   * widget calls `onAutoOpenCreateHandled` once it has acted on it, so the
   * caller can drop it back to false — otherwise every remount of this
   * widget (e.g. just clicking back onto the tab) would reopen the dialog. */
  autoOpenCreate?: boolean;
  onAutoOpenCreateHandled?: () => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CallRequestsWidget({
  caseId,
  severity,
  autoOpenCreate,
  onAutoOpenCreateHandled,
}: CallRequestsWidgetProps): JSX.Element {
  // State filter — empty string means "all". Filtering happens server-side
  // via `filters.states` on the search request.
  const [stateFilter, setStateFilter] = useState<BeCallRequestStateKey | "">("");
  const activeStates = stateFilter ? [stateFilter] : undefined;

  const { data, isLoading, isError, refetch } = useGetCsmCaseCallRequests(
    caseId,
    activeStates,
  );
  const postCallRequest = usePostCsmCaseCallRequest();
  const patchCallRequest = usePatchCsmCaseCallRequest();

  const [createOpen, setCreateOpen] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  useEffect(() => {
    if (autoOpenCreate) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs the dialog open to an external one-shot trigger from the case action bar
      setCreateOpen(true);
      onAutoOpenCreateHandled?.();
    }
  }, [autoOpenCreate, onAutoOpenCreateHandled]);

  // Dialog targets — only one dialog is ever open at a time, driven by which
  // action was clicked on a row.
  const [scheduleTarget, setScheduleTarget] = useState<BeCallRequestView | null>(null);
  const [rejectTarget, setRejectTarget] = useState<BeCallRequestView | null>(null);
  const [notesTarget, setNotesTarget] = useState<BeCallRequestView | null>(null);
  const [cancelTarget, setCancelTarget] = useState<BeCallRequestView | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const isReschedule =
    resolveCallRequestStateKey(scheduleTarget?.state) === "scheduled";

  const requests = data ?? [];

  const handleCreate = async (
    reason: string,
    utcTimes: string[],
    durationInMinutes: number,
  ) => {
    setCreateError(null);
    try {
      await postCallRequest.mutateAsync({ caseId, reason, utcTimes, durationInMinutes });
      setCreateOpen(false);
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : "Could not submit the call request.",
      );
    }
  };

  const handleAction = (action: CallRequestAgentAction, cr: BeCallRequestView) => {
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

  const handleSchedule = async (input: {
    meetingDate: string;
    durationInMinutes: number;
    assignee?: string;
  }) => {
    if (!scheduleTarget) return;
    setActionError(null);
    try {
      await patchCallRequest.mutateAsync({
        caseId,
        callRequestId: scheduleTarget.id,
        patch: { state: "scheduled", ...input },
      });
      setScheduleTarget(null);
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Could not schedule the call.",
      );
    }
  };

  const handleReject = async (reason?: string) => {
    if (!rejectTarget) return;
    setActionError(null);
    try {
      await patchCallRequest.mutateAsync({
        caseId,
        callRequestId: rejectTarget.id,
        patch: {
          state: "wso2_rejected",
          ...(reason ? { cancellationReason: reason } : {}),
        },
      });
      setRejectTarget(null);
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Could not reject the call request.",
      );
    }
  };

  const handleSendNotes = async (input: {
    notes: string;
    plan?: string;
    attendees?: string;
    actionItems?: string;
    actualDuration?: number;
  }) => {
    if (!notesTarget) return;
    setActionError(null);
    try {
      const { actualDuration, ...rest } = input;
      await patchCallRequest.mutateAsync({
        caseId,
        callRequestId: notesTarget.id,
        patch: {
          state: "concluded",
          ...rest,
          ...(actualDuration !== undefined
            ? { actualDurationMin: actualDuration }
            : {}),
        },
      });
      setNotesTarget(null);
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Could not send the call notes.",
      );
    }
  };

  const handleCancel = async (cancellationReason: string) => {
    if (!cancelTarget) return;
    setActionError(null);
    try {
      await patchCallRequest.mutateAsync({
        caseId,
        callRequestId: cancelTarget.id,
        patch: { state: "canceled", cancellationReason },
      });
      setCancelTarget(null);
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : "Could not cancel the call request.",
      );
    }
  };

  return (
    <>
      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        {/* Header */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 1,
            flexWrap: "wrap",
          }}
        >
          <Box sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <Phone size={16} />
            <Typography variant="subtitle2">Call requests</Typography>
            {!isLoading && !isError && (
              <Chip
                size="small"
                variant="outlined"
                label={`${requests.length} ${stateFilter ? "matching" : "total"}`}
              />
            )}
          </Box>
          <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
            {/* State filter */}
            <FormControl size="small" sx={{ minWidth: 180 }}>
              <InputLabel id="cr-filter-label">Filter by state</InputLabel>
              <Select
                labelId="cr-filter-label"
                value={stateFilter}
                label="Filter by state"
                onChange={(e) =>
                  setStateFilter(e.target.value as BeCallRequestStateKey | "")
                }
              >
                <MenuItem value="">All states</MenuItem>
                <Divider />
                {ALL_CALL_REQUEST_STATES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {CALL_REQUEST_STATE_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <Button
              size="small"
              variant="contained"
              startIcon={<Plus size={14} />}
              onClick={() => setCreateOpen(true)}
              sx={{ textTransform: "none" }}
            >
              Create call request
            </Button>
          </Box>
        </Box>

        {/* Content */}
        {isLoading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} variant="rectangular" height={64} />
            ))}
          </Box>
        )}

        {isError && (
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 1,
              py: 3,
            }}
          >
            <Typography variant="body2" color="error">
              Could not load call requests.
            </Typography>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshCw size={14} />}
              onClick={() => void refetch()}
              sx={{ textTransform: "none" }}
            >
              Retry
            </Button>
          </Box>
        )}

        {!isLoading && !isError && requests.length === 0 && (
          <Box sx={{ py: 3, textAlign: "center" }}>
            <Typography variant="body2" color="text.secondary">
              {stateFilter
                ? `No call requests in state "${CALL_REQUEST_STATE_LABEL[stateFilter]}".`
                : "No call requests yet."}
            </Typography>
          </Box>
        )}

        {!isLoading && !isError && requests.length > 0 && (
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            {requests.map((cr) => (
              <CallRequestRow key={cr.id} cr={cr} onAction={handleAction} />
            ))}
          </Box>
        )}
      </Card>

      <CreateCallRequestDialog
        open={createOpen}
        submitting={postCallRequest.isPending}
        error={createError}
        severity={severity}
        onClose={() => {
          setCreateOpen(false);
          setCreateError(null);
        }}
        onSubmit={(reason, utcTimes, durationInMinutes) =>
          void handleCreate(reason, utcTimes, durationInMinutes)
        }
      />

      <ScheduleCallDialog
        callRequest={scheduleTarget}
        isReschedule={!!isReschedule}
        submitting={patchCallRequest.isPending}
        error={actionError}
        onClose={() => {
          setScheduleTarget(null);
          setActionError(null);
        }}
        onSubmit={(input) => void handleSchedule(input)}
      />

      <RejectCallDialog
        callRequest={rejectTarget}
        submitting={patchCallRequest.isPending}
        error={actionError}
        onClose={() => {
          setRejectTarget(null);
          setActionError(null);
        }}
        onSubmit={(reason) => void handleReject(reason)}
      />

      <SendCallNotesDialog
        callRequest={notesTarget}
        submitting={patchCallRequest.isPending}
        error={actionError}
        onClose={() => {
          setNotesTarget(null);
          setActionError(null);
        }}
        onSubmit={(input) => void handleSendNotes(input)}
      />

      <CancelCallDialog
        callRequest={cancelTarget}
        submitting={patchCallRequest.isPending}
        error={actionError}
        onClose={() => {
          setCancelTarget(null);
          setActionError(null);
        }}
        onSubmit={(reason) => void handleCancel(reason)}
      />
    </>
  );
}
