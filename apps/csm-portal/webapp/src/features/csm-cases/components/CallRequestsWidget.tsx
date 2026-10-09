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
  Card,
  Chip,
  Divider,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Phone, Plus, RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useRef, useState, type JSX } from "react";
import type { BeCallRequestView, BeCallRequestStateKey } from "@api/backend/types";
import type {
  CaseState,
  SeverityOrUnset,
} from "@features/csm-dashboard/types/abtDashboard";
import {
  useGetCsmCaseCallRequests,
  usePostCsmCaseCallRequest,
  usePatchCsmCaseCallRequest,
} from "@features/csm-cases/api/useCsmCaseCallRequests";
import {
  ALL_CALL_REQUEST_STATES,
  CALL_REQUEST_STATE_LABEL,
  OPEN_CALL_REQUEST_STATES,
  callRequestCaseStateBlockReason,
  type CallRequestAgentAction,
  resolveCallRequestStateKey,
} from "@features/csm-cases/utils/callRequestState";
import { CreateCallRequestDialog } from "./CreateCallRequestDialog";
import { ScheduleCallDialog } from "./ScheduleCallDialog";
import { RejectCallDialog } from "./RejectCallDialog";
import { SendCallNotesDialog } from "./SendCallNotesDialog";
import { CancelCallDialog } from "./CancelCallDialog";
import { CallRequestsTable } from "./CallRequestsTable";
import RefreshButton from "@components/RefreshButton";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface CallRequestsWidgetProps {
  caseId: string;
  /** Case severity — passed to the create dialog to enforce the lead-time rule. */
  severity?: SeverityOrUnset;
  /**
   * Case's current state — the data source only accepts a call request while
   * the case is in one of a fixed set of states. Gates both the "Create call
   * request" trigger and the dialog's submit action; passed straight from the
   * live case-detail data so it stays in sync if the case's state changes.
   */
  caseState?: CaseState;
  /** True to pop the "Create call request" dialog from outside the widget
   * (e.g. the case action bar's "Request a call" item). One-shot: the
   * widget calls `onAutoOpenCreateHandled` once it has acted on it, so the
   * caller can drop it back to false — otherwise every remount of this
   * widget (e.g. just clicking back onto the tab) would reopen the dialog. */
  autoOpenCreate?: boolean;
  onAutoOpenCreateHandled?: () => void;
  /** True when the parent case is closed — call requests stay visible but
   * become read-only: no new requests, no updates to existing ones. */
  isClosed?: boolean;
  /** True when the caller has no write access: requests stay visible, but
   * creating or updating one is disabled (the backend would 403 it). */
  readOnly?: boolean;
}

/** "open" = calls that can still move on, "all" = every call, else a single state. */
type CallRequestStateFilter = BeCallRequestStateKey | "open" | "all";

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function CallRequestsWidget({
  caseId,
  severity,
  caseState,
  autoOpenCreate,
  onAutoOpenCreateHandled,
  isClosed,
  readOnly,
}: CallRequestsWidgetProps): JSX.Element {
  // State filter. The default shows only calls that can still move on, so a call
  // that was completed, canceled or rejected leaves the list; "all" brings every
  // call back. Filtering happens server-side via `filters.states`.
  const [stateFilter, setStateFilter] = useState<CallRequestStateFilter>("open");
  const activeStates =
    stateFilter === "all"
      ? undefined
      : stateFilter === "open"
        ? OPEN_CALL_REQUEST_STATES
        : [stateFilter];

  const { data, isLoading, isError, refetch, isFetching, dataUpdatedAt } =
    useGetCsmCaseCallRequests(caseId, activeStates);
  const postCallRequest = usePostCsmCaseCallRequest();
  const patchCallRequest = usePatchCsmCaseCallRequest();

  const [createOpen, setCreateOpen] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const stateBlockReason = callRequestCaseStateBlockReason(caseState);

  useEffect(() => {
    if (autoOpenCreate) {
      if (!isClosed && !readOnly && !stateBlockReason) {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- syncs the dialog open to an external one-shot trigger from the case action bar
        setCreateOpen(true);
      }
      onAutoOpenCreateHandled?.();
    }
  }, [autoOpenCreate, isClosed, readOnly, stateBlockReason, onAutoOpenCreateHandled]);

  // Dialog targets — only one dialog is ever open at a time, driven by which
  // action was clicked on a row.
  const [scheduleTarget, setScheduleTarget] = useState<BeCallRequestView | null>(null);
  const [rejectTarget, setRejectTarget] = useState<BeCallRequestView | null>(null);
  const [notesTarget, setNotesTarget] = useState<BeCallRequestView | null>(null);
  const [cancelTarget, setCancelTarget] = useState<BeCallRequestView | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  // "Mark as completed" has no dialog of its own to show a failure in, so its error
  // is shown on the card. The ref (not just the mutation's isPending) is what stops
  // a fast double click from sending the PATCH twice before the first re-render.
  const [completeError, setCompleteError] = useState<string | null>(null);
  const completingRef = useRef(false);

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
    setCompleteError(null);
    switch (action) {
      case "complete":
        void handleComplete(cr);
        break;
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
    if (stateBlockReason) {
      setActionError(stateBlockReason);
      return;
    }
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

  // One click, no dialog and no notes: concludes the call (engineers had no way to finish a call). The
  // backend only accepts this for a scheduled / notes-pending call and answers 409
  // (with the call's current state) when the row on screen is stale.
  const handleComplete = async (cr: BeCallRequestView) => {
    if (completingRef.current) return;
    completingRef.current = true;
    setCompleteError(null);
    try {
      await patchCallRequest.mutateAsync({
        caseId,
        callRequestId: cr.id,
        patch: { state: "concluded" },
      });
    } catch (err) {
      setCompleteError(
        err instanceof Error && err.message
          ? err.message
          : "Could not mark the call request as completed.",
      );
    }
    // Reset after the try/catch rather than in a `finally`: the catch above never
    // rethrows, so this always runs, and the React Compiler does not support a
    // `finally` clause (it would skip optimizing this whole component).
    completingRef.current = false;
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
                label={`${requests.length} ${
                  stateFilter === "open" ? "open" : stateFilter === "all" ? "total" : "matching"
                }`}
              />
            )}
          </Box>
          <Box sx={{ display: "flex", gap: 1, alignItems: "center", flexWrap: "wrap" }}>
            <RefreshButton
              onRefresh={() => void refetch()}
              isFetching={isFetching}
              updatedAt={dataUpdatedAt}
              label="Refresh call requests"
            />
            {/* State filter */}
            <FormControl size="small" sx={{ minWidth: 180 }}>
              <InputLabel
                id="cr-filter-label"
                // oxygen-ui's own theme shifts an unshrunk label up by
                // `top: -7px` for any Select-backed field (see
                // `MultiSelectField.tsx`'s doc comment) -- tie `shrink` to
                // whether a state is actually picked, rather than MUI's
                // focus-driven default, and force the cascade with
                // `!important` since a plain `sx={{ top: 0 }}` loses to that
                // theme rule's higher specificity.
                shrink
                sx={{ top: "0px !important" }}
              >
                Filter by state
              </InputLabel>
              <Select
                labelId="cr-filter-label"
                value={stateFilter}
                label="Filter by state"
                notched
                onChange={(e) => setStateFilter(e.target.value as CallRequestStateFilter)}
              >
                <MenuItem value="open">Open calls</MenuItem>
                <MenuItem value="all">All states</MenuItem>
                <Divider />
                {ALL_CALL_REQUEST_STATES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {CALL_REQUEST_STATE_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <Tooltip
              title={
                isClosed
                  ? "This case is closed — it's read-only."
                  : readOnly
                    ? "You don't have permission to create call requests."
                    : (stateBlockReason ?? "")
              }
            >
              <span>
                <Button
                  size="small"
                  variant="contained"
                  startIcon={<Plus size={14} />}
                  onClick={() => setCreateOpen(true)}
                  disabled={isClosed || readOnly || !!stateBlockReason}
                  sx={{ textTransform: "none" }}
                >
                  Create call request
                </Button>
              </span>
            </Tooltip>
          </Box>
        </Box>

        {/* Content */}
        {isLoading && (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} variant="rounded" height={64} />
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
              {stateFilter === "open"
                ? "No open call requests. Choose \"All states\" to see finished ones."
                : stateFilter === "all"
                  ? "No call requests yet."
                  : `No call requests in state "${CALL_REQUEST_STATE_LABEL[stateFilter]}".`}
            </Typography>
          </Box>
        )}

        {completeError && (
          <Alert severity="error" onClose={() => setCompleteError(null)}>
            {completeError}
          </Alert>
        )}

        {!isLoading && !isError && requests.length > 0 && (
          <CallRequestsTable
            requests={requests}
            onAction={handleAction}
            isClosed={isClosed}
            readOnly={readOnly}
            busy={patchCallRequest.isPending}
          />
        )}
      </Card>

      <CreateCallRequestDialog
        open={createOpen}
        submitting={postCallRequest.isPending}
        error={createError}
        severity={severity}
        caseState={caseState}
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
        stateBlockReason={stateBlockReason}
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
