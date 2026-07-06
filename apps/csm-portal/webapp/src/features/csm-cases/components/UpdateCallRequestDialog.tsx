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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeCallRequestView, BeCallRequestStateKey } from "@api/backend/types";
import {
  CALL_REQUEST_STATE_LABEL,
  CALL_REQUEST_TRANSITIONS,
  callRequestStateLabel,
  requiresCancellationReason,
} from "@features/csm-cases/utils/callRequestState";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface UpdateDialogProps {
  callRequest: BeCallRequestView | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (newState: BeCallRequestStateKey, cancellationReason?: string) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function UpdateCallRequestDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: UpdateDialogProps): JSX.Element {
  const currentStateKey = callRequest?.state
    ? (String(callRequest.state.id) as BeCallRequestStateKey)
    : null;
  const transitions = currentStateKey ? CALL_REQUEST_TRANSITIONS[currentStateKey] : [];

  // Use `string` here to avoid TS union-narrowing false-positive on the "" guard.
  const [selectedState, setSelectedState] = useState<string>("");
  const [cancelReason, setCancelReason] = useState("");

  const handleClose = () => {
    setSelectedState("");
    setCancelReason("");
    onClose();
  };

  const needsCancelReason =
    selectedState !== "" && requiresCancellationReason(selectedState as BeCallRequestStateKey);
  const canSubmit =
    selectedState !== "" &&
    (!needsCancelReason || cancelReason.trim().length > 0);

  const handleSubmit = () => {
    if (!canSubmit || selectedState === "") return;
    onSubmit(selectedState as BeCallRequestStateKey, needsCancelReason ? cancelReason.trim() : undefined);
  };

  return (
    <Dialog
      open={!!callRequest}
      onClose={handleClose}
      maxWidth="xs"
      fullWidth
    >
      <DialogTitle>Update call request state</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          {callRequest && (
            <Typography variant="body2" color="text.secondary">
              Current state:{" "}
              <strong>{callRequestStateLabel(callRequest.state)}</strong>
            </Typography>
          )}
          {transitions.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No transitions available from this state.
            </Typography>
          ) : (
            <FormControl fullWidth size="small" disabled={submitting}>
              <InputLabel id="cr-state-select-label">New state</InputLabel>
              <Select
                labelId="cr-state-select-label"
                value={selectedState}
                label="New state"
                onChange={(e) =>
                  setSelectedState(e.target.value)
                }
              >
                {transitions.map((s) => (
                  <MenuItem key={s} value={s}>
                    {CALL_REQUEST_STATE_LABEL[s]}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
          {needsCancelReason && (
            <TextField
              label="Cancellation reason"
              multiline
              minRows={2}
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
              fullWidth
              required
              disabled={submitting}
            />
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={handleClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={!canSubmit || transitions.length === 0 || submitting}
          loading={submitting}
        >
          Update
        </Button>
      </DialogActions>
    </Dialog>
  );
}
