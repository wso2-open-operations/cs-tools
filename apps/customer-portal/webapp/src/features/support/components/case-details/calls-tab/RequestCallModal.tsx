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
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Info, Plus, Trash2, X } from "@wso2/oxygen-ui-icons-react";
import {
  useCallback,
  useMemo,
  useState,
  useEffect,
  useRef,
  type JSX,
  type ChangeEvent,
} from "react";
import type { SelectChangeEvent } from "@wso2/oxygen-ui";
import { usePostCallRequest } from "@features/support/api/usePostCallRequest";
import { usePatchCallRequest } from "@features/support/api/usePatchCallRequest";
import type { CallRequest } from "@features/support/types/calls";
import { CALL_REQUEST_STATE_PENDING_ON_WSO2 } from "@features/support/constants/supportConstants";
import {
  callRequestApiPreferredTimeToDatetimeLocal,
  callRequestPreferredTimeFromDatetimeLocal,
  computeMinScheduleDatetimeLocalForTimeZone,
  normalizeDatetimeLocalForCompare,
  sortCallRequestPreferredTimeStringsAsc,
  stripCustomerPrefixFromReason,
} from "@features/support/utils/support";

const DURATION_OPTIONS = [
  { value: 15, label: "15 minutes" },
  { value: 30, label: "30 minutes" },
  { value: 45, label: "45 minutes" },
  { value: 60, label: "1 hour" },
];

const INITIAL_FORM = {
  preferredDateTimesLocal: [""],
  durationInMinutes: 30,
  notes: "",
};
const MAX_PREFERRED_TIMES = 3;

export interface RequestCallModalProps {
  open: boolean;
  projectId: string;
  caseId: string;
  onClose: () => void;
  onSuccess?: () => void;
  onError?: (message: string) => void;
  /** When provided, modal opens in edit mode with pre-filled values. */
  editCall?: CallRequest;
  userTimeZone?: string;
  /** Minutes after now (from project filters × case severity) before first schedulable slot. */
  severityAllocationMinutes?: number;
}

/**
 * Modal for requesting a call for a case.
 *
 * @param {RequestCallModalProps} props - open, projectId, caseId, onClose, onSuccess, onError.
 * @returns {JSX.Element} The request call modal.
 */
export default function RequestCallModal({
  open,
  projectId,
  caseId,
  onClose,
  onSuccess,
  onError,
  editCall,
  userTimeZone,
  severityAllocationMinutes,
}: RequestCallModalProps): JSX.Element {
  const postCallRequest = usePostCallRequest(projectId, caseId);
  const patchCallRequest = usePatchCallRequest(projectId, caseId);
  const postMutate = postCallRequest.mutate;
  const patchMutate = patchCallRequest.mutate;
  const isEdit = !!editCall;

  const [form, setForm] = useState(INITIAL_FORM);
  const [modalError, setModalError] = useState<string | null>(null);
  const [minDatetimeTick, setMinDatetimeTick] = useState(0);

  useEffect(() => {
    if (!open) return;
    const id = setInterval(() => setMinDatetimeTick((t) => t + 1), 60 * 1000);
    return () => clearInterval(id);
  }, [open]);

  const minDatetimeLocal = useMemo(() => {
    void minDatetimeTick;
    return computeMinScheduleDatetimeLocalForTimeZone(
      severityAllocationMinutes,
      userTimeZone,
    );
  }, [severityAllocationMinutes, minDatetimeTick, userTimeZone]);

  const stateKey = CALL_REQUEST_STATE_PENDING_ON_WSO2;

  const prevEditCallIdRef = useRef<string | null>(null);
  const createFormSeededRef = useRef(false);

  useEffect(() => {
    if (!open) {
      prevEditCallIdRef.current = null;
      createFormSeededRef.current = false;
      return;
    }
    if (!editCall) {
      return;
    }
    if (prevEditCallIdRef.current === editCall.id) {
      return;
    }
    prevEditCallIdRef.current = editCall.id;
    const rawPreferred =
      editCall.preferredTimes && editCall.preferredTimes.length > 0
        ? editCall.preferredTimes.slice(0, MAX_PREFERRED_TIMES)
        : editCall.scheduleTime
          ? [editCall.scheduleTime]
          : [""];
    const preferredUtcTimes =
      sortCallRequestPreferredTimeStringsAsc(rawPreferred);
    queueMicrotask(() => {
      setForm({
        preferredDateTimesLocal: preferredUtcTimes.map((time) =>
          callRequestApiPreferredTimeToDatetimeLocal(time, userTimeZone),
        ),
        durationInMinutes: editCall.durationMin ?? 30,
        notes: stripCustomerPrefixFromReason(editCall.reason || ""),
      });
    });
  }, [open, editCall, userTimeZone]);

  useEffect(() => {
    if (!open || editCall) {
      return;
    }
    if (createFormSeededRef.current) {
      return;
    }
    createFormSeededRef.current = true;
    queueMicrotask(() => {
      setForm({
        ...INITIAL_FORM,
        preferredDateTimesLocal: [minDatetimeLocal],
      });
      setModalError(null);
    });
  }, [open, editCall, minDatetimeLocal]);

  /** Create mode: bump preferred slots when severity floor rises (filters load or minute tick). */
  useEffect(() => {
    if (!open || isEdit) return;
    const floorKey = normalizeDatetimeLocalForCompare(minDatetimeLocal);
    if (!floorKey) return;
    queueMicrotask(() => {
      setForm((prev) => {
        let changed = false;
        const next = prev.preferredDateTimesLocal.map((v) => {
          if (!v.trim()) {
            changed = true;
            return minDatetimeLocal;
          }
          const vk = normalizeDatetimeLocalForCompare(v);
          if (!vk || vk < floorKey) {
            changed = true;
            return minDatetimeLocal;
          }
          return v;
        });
        return changed ? { ...prev, preferredDateTimesLocal: next } : prev;
      });
    });
  }, [open, isEdit, minDatetimeLocal]);

  const isPending = postCallRequest.isPending || patchCallRequest.isPending;
  const isValid =
    form.preferredDateTimesLocal.every((value) => value.trim() !== "") &&
    form.durationInMinutes > 0 &&
    (isEdit || form.notes.trim() !== "");

  const handleClose = useCallback(() => {
    setForm(INITIAL_FORM);
    setModalError(null);
    onClose();
  }, [onClose]);

  const handleTextChange =
    (field: keyof typeof form) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setForm((prev) => ({ ...prev, [field]: event.target.value }));
    };

  const handlePreferredTimeChange = useCallback(
    (index: number) =>
      (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        let value = event.target.value;
        const floorKey = normalizeDatetimeLocalForCompare(minDatetimeLocal);
        const selKey = normalizeDatetimeLocalForCompare(value);
        if (floorKey && selKey && selKey < floorKey) {
          value = minDatetimeLocal;
        }
        setForm((prev) => {
          const nextTimes = [...prev.preferredDateTimesLocal];
          nextTimes[index] = value;
          return { ...prev, preferredDateTimesLocal: nextTimes };
        });
      },
    [minDatetimeLocal],
  );

  const handleAddPreferredTime = useCallback(() => {
    setForm((prev) => {
      if (prev.preferredDateTimesLocal.length >= MAX_PREFERRED_TIMES)
        return prev;
      return {
        ...prev,
        preferredDateTimesLocal: [
          ...prev.preferredDateTimesLocal,
          minDatetimeLocal,
        ],
      };
    });
  }, [minDatetimeLocal]);

  const handleRemovePreferredTime = useCallback((index: number) => {
    setForm((prev) => {
      if (prev.preferredDateTimesLocal.length <= 1) return prev;
      return {
        ...prev,
        preferredDateTimesLocal: prev.preferredDateTimesLocal.filter(
          (_value, i) => i !== index,
        ),
      };
    });
  }, []);

  const handleDurationChange = (event: SelectChangeEvent<number>) => {
    setForm((prev) => ({
      ...prev,
      durationInMinutes: Number(event.target.value),
    }));
  };

  const handleSubmit = useCallback(() => {
    setModalError(null);
    if (!isValid) return;

    const minKey = normalizeDatetimeLocalForCompare(minDatetimeLocal);
    const utcTimes: string[] = [];
    for (const localTime of form.preferredDateTimesLocal) {
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

    const handleError = (error: Error) => {
      const msg = error?.message ?? "";
      const friendlyMsg =
        /\b(?:cannot be past|date.*past|time.*past|in the past)\b/i.test(msg)
          ? "The selected date and time cannot be in the past. Please choose a future date and time."
          : msg || "Failed to save call request.";
      setModalError(friendlyMsg);
      onError?.(friendlyMsg);
    };

    if (isEdit && editCall) {
      patchMutate(
        {
          callRequestId: editCall.id,
          stateKey,
          utcTimes,
          durationInMinutes: form.durationInMinutes,
        },
        {
          onSuccess: () => {
            handleClose();
            onSuccess?.();
          },
          onError: handleError,
        },
      );
    } else {
      postMutate(
        {
          durationInMinutes: form.durationInMinutes,
          reason: form.notes.trim(),
          utcTimes,
        },
        {
          onSuccess: () => {
            handleClose();
            onSuccess?.();
          },
          onError: handleError,
        },
      );
    }
  }, [
    isValid,
    form,
    stateKey,
    isEdit,
    editCall,
    postMutate,
    patchMutate,
    handleClose,
    onSuccess,
    onError,
    minDatetimeLocal,
    userTimeZone,
  ]);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="request-call-dialog-title"
      aria-describedby="request-call-dialog-description"
    >
      <DialogTitle
        id="request-call-dialog-title"
        sx={{ pr: 6, position: "relative", pb: 0.5 }}
      >
        {isEdit ? "Edit Call Request" : "Request Call"}
        <Typography
          id="request-call-dialog-description"
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal", fontSize: "0.875rem" }}
        >
          {isEdit
            ? "Update preferred times and meeting duration for this call request."
            : "Schedule a call with our support team."}
        </Typography>
        <IconButton
          aria-label="close"
          onClick={handleClose}
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
        {form.preferredDateTimesLocal.map((value, index) => (
          <Box
            key={`preferred-time-${index}`}
            sx={{
              mt: index === 0 ? 4 : 1.5,
              display: "flex",
              alignItems: "center",
              gap: 1,
            }}
          >
            <TextField
              id={`preferred-time-${index}`}
              label={
                index === 0
                  ? "Preferred Time *"
                  : `Preferred Time ${index + 1} *`
              }
              type="datetime-local"
              value={value}
              onChange={handlePreferredTimeChange(index)}
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
                  isPending ||
                  form.preferredDateTimesLocal.length >= MAX_PREFERRED_TIMES
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
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              mt: 0.5,
              mb: 2,
            }}
          >
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

        <FormControl fullWidth size="small" sx={{ mt: 1, mb: 2 }}>
          <InputLabel id="duration-label">Meeting Duration *</InputLabel>
          <Select<number>
            labelId="duration-label"
            id="duration"
            value={form.durationInMinutes}
            label="Meeting Duration *"
            onChange={handleDurationChange}
            disabled={isPending}
          >
            {DURATION_OPTIONS.map(({ value, label }) => (
              <MenuItem key={value} value={value}>
                {label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        {!isEdit && (
          <TextField
            id="additional-notes"
            label="Reason *"
            placeholder="Describe your call request or topics you'd like to discuss."
            value={form.notes}
            onChange={handleTextChange("notes")}
            fullWidth
            size="small"
            multiline
            rows={3}
            disabled={isPending}
          />
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
            {isEdit ? "Updating..." : "Requesting..."}
          </Button>
        ) : (
          <Button
            type="button"
            variant="contained"
            color="primary"
            onClick={handleSubmit}
            disabled={!isValid}
          >
            {isEdit ? "Update Call Request" : "Request Call"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
