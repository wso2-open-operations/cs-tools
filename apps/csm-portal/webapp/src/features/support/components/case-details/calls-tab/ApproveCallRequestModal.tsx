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

import type { ApproveCallRequestModalProps } from "@features/support/types/supportComponents";
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
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Info, Plus, Trash2, X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ChangeEvent,
  type JSX,
} from "react";
import { usePatchCallRequest } from "@features/support/api/usePatchCallRequest";
import {
  callRequestApiPreferredTimeToDatetimeLocal,
  callRequestPreferredTimeFromDatetimeLocal,
  computeMinScheduleDatetimeLocalForTimeZone,
  normalizeDatetimeLocalForCompare,
  sortCallRequestPreferredTimeStringsAsc,
} from "@features/support/utils/support";

const MAX_PREFERRED_TIMES = 3;

/**
 * Modal for approving a "Pending on Customer" call request.
 * Sends a PATCH with stateKey=2 (PENDING_ON_WSO2) and the customer's preferred time.
 *
 * @param {ApproveCallRequestModalProps} props - open, call, projectId, caseId, onClose, onSuccess, onError.
 * @returns {JSX.Element} The approve call request modal.
 */
export default function ApproveCallRequestModal({
  open,
  call,
  projectId,
  caseId,
  onClose,
  onSuccess,
  onError,
  userTimeZone,
  severityAllocationMinutes,
  approveStateKey,
}: ApproveCallRequestModalProps): JSX.Element {
  const patchCallRequest = usePatchCallRequest(projectId, caseId);
  const [preferredDateTimes, setPreferredDateTimes] = useState<string[]>([""]);
  const [modalError, setModalError] = useState<string | null>(null);
  const [minTick, setMinTick] = useState(0);

  useEffect(() => {
    if (!open) return;
    const id = setInterval(() => setMinTick((t) => t + 1), 60 * 1000);
    return () => clearInterval(id);
  }, [open]);

  useEffect(() => {
    if (!open) {
      queueMicrotask(() => setPreferredDateTimes([""]));
      return;
    }
    if (!call) {
      queueMicrotask(() => setPreferredDateTimes([""]));
      return;
    }
    const rawPreferred =
      call.preferredTimes && call.preferredTimes.length > 0
        ? call.preferredTimes.slice(0, MAX_PREFERRED_TIMES)
        : call.scheduleTime
          ? [call.scheduleTime]
          : [""];
    const sorted = sortCallRequestPreferredTimeStringsAsc(rawPreferred);
    queueMicrotask(() => {
      setPreferredDateTimes(
        sorted.map((t) =>
          callRequestApiPreferredTimeToDatetimeLocal(t, userTimeZone),
        ),
      );
      setModalError(null);
    });
  }, [open, call, userTimeZone]);

  const minDatetimeLocal = useMemo(() => {
    void minTick;
    return computeMinScheduleDatetimeLocalForTimeZone(
      severityAllocationMinutes,
      userTimeZone,
    );
  }, [severityAllocationMinutes, userTimeZone, minTick]);
  const isValid =
    preferredDateTimes.every((value) => value.trim() !== "") &&
    approveStateKey !== undefined;
  const isPending = patchCallRequest.isPending;

  const handleDialogClose = useCallback(() => {
    if (isPending) return;
    setPreferredDateTimes([""]);
    setModalError(null);
    onClose();
  }, [isPending, onClose]);

  const handleClose = useCallback(() => {
    if (isPending) return;
    setPreferredDateTimes([""]);
    setModalError(null);
    onClose();
  }, [isPending, onClose]);

  const handleDateTimeChange = useCallback(
    (index: number) => (e: ChangeEvent<HTMLInputElement>) => {
      const value = e.target.value;
      setPreferredDateTimes((prev) => {
        const next = [...prev];
        next[index] = value;
        return next;
      });
    },
    [],
  );

  const handleAddPreferredTime = useCallback(() => {
    setPreferredDateTimes((prev) => {
      if (prev.length >= MAX_PREFERRED_TIMES) return prev;
      return [...prev, ""];
    });
  }, []);

  const handleRemovePreferredTime = useCallback((index: number) => {
    setPreferredDateTimes((prev) => {
      if (prev.length <= 1) return prev;
      return prev.filter((_value, i) => i !== index);
    });
  }, []);

  const handleSubmit = useCallback(() => {
    if (!isValid || !call || approveStateKey === undefined) return;
    setModalError(null);

    const minKey = normalizeDatetimeLocalForCompare(minDatetimeLocal);
    const utcTimes: string[] = [];
    for (const localTime of preferredDateTimes) {
      const iso = callRequestPreferredTimeFromDatetimeLocal(
        localTime,
        userTimeZone,
      );
      if (!iso) {
        setModalError("Please enter a valid preferred time.");
        return;
      }
      const selKey = normalizeDatetimeLocalForCompare(localTime);
      if (minKey && selKey && selKey < minKey) {
        setModalError(
          "The selected date and time must be at least the minimum shown in the picker (including severity-based lead time).",
        );
        return;
      }
      utcTimes.push(iso);
    }

    patchCallRequest.mutate(
      {
        callRequestId: call.id,
        stateKey: approveStateKey,
        utcTimes,
      },
      {
        onSuccess: () => {
          setPreferredDateTimes([""]);
          setModalError(null);
          onClose();
          onSuccess?.();
        },
        onError: (error) => {
          const msg = error?.message ?? "Failed to approve call request.";
          setModalError(msg);
          onError?.(msg);
        },
      },
    );
  }, [
    isValid,
    call,
    preferredDateTimes,
    approveStateKey,
    patchCallRequest,
    onClose,
    onSuccess,
    onError,
    minDatetimeLocal,
    userTimeZone,
  ]);

  return (
    <Dialog
      open={open}
      onClose={handleDialogClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="approve-call-request-dialog-title"
      aria-describedby="approve-call-request-dialog-description"
    >
      <DialogTitle
        id="approve-call-request-dialog-title"
        sx={{ pr: 6, position: "relative", pb: 0.5 }}
      >
        Approve Call Request
        <Typography
          id="approve-call-request-dialog-description"
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
        >
          Enter preferred time for this call request.
        </Typography>
        <IconButton
          aria-label="Close"
          onClick={handleClose}
          disabled={isPending}
          sx={{ position: "absolute", right: 12, top: 12 }}
          size="small"
        >
          <X size={20} aria-hidden />
        </IconButton>
      </DialogTitle>

      <DialogContent sx={{ pt: 1 }}>
        {modalError && (
          <Alert
            severity="error"
            onClose={() => setModalError(null)}
            sx={{ mb: 2 }}
          >
            {modalError}
          </Alert>
        )}
        {preferredDateTimes.map((value, index) => (
          <Box
            key={`approve-preferred-time-${index}`}
            sx={{
              mt: index === 0 ? 4 : 1.5,
              display: "flex",
              alignItems: "center",
              gap: 1,
            }}
          >
            <TextField
              id={`approve-preferred-time-${index}`}
              label={
                index === 0
                  ? "Preferred Time *"
                  : `Preferred Time ${index + 1} *`
              }
              type="datetime-local"
              value={value}
              onChange={handleDateTimeChange(index)}
              fullWidth
              size="small"
              slotProps={{
                inputLabel: { shrink: true },
                htmlInput: {
                  min: minDatetimeLocal,
                  step: 300,
                },
              }}
              inputProps={{
                min: minDatetimeLocal,
                step: 300,
              }}
              disabled={isPending}
            />
            <Box
              sx={{
                width: 64,
                display: "flex",
                alignItems: "center",
                justifyContent: "flex-end",
                gap: 0.5,
              }}
            >
              <IconButton
                aria-label="Add preferred time"
                onClick={handleAddPreferredTime}
                disabled={
                  isPending || preferredDateTimes.length >= MAX_PREFERRED_TIMES
                }
                size="small"
              >
                <Plus size={18} />
              </IconButton>
              <IconButton
                aria-label={`Remove preferred time ${index + 1}`}
                onClick={() => handleRemovePreferredTime(index)}
                disabled={isPending || index === 0}
                size="small"
              >
                <Trash2 size={16} />
              </IconButton>
            </Box>
          </Box>
        ))}
        {userTimeZone && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.5 }}>
            <Typography variant="caption" color="text.secondary">
              Your current time zone is {userTimeZone}
            </Typography>
            <Tooltip title="To change it, please go to the profile page." arrow>
              <Box component="span" sx={{ display: "inline-flex", ml: 0.25 }}>
                <Info size={14} />
              </Box>
            </Tooltip>
          </Box>
        )}
      </DialogContent>

      <DialogActions
        sx={{ px: 3, pb: 3, pt: 1, justifyContent: "flex-end", gap: 1 }}
      >
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          Cancel
        </Button>
        {isPending ? (
          <Button
            variant="contained"
            color="primary"
            startIcon={<CircularProgress color="inherit" size={16} />}
            disabled
          >
            Approving...
          </Button>
        ) : (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleSubmit}
            disabled={!isValid}
          >
            Approve
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
