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
import { useState, type JSX } from "react";
import type { BeCallRequestView } from "@api/backend/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CancelCallDialogProps {
  callRequest: BeCallRequestView | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (cancellationReason: string) => void;
}

// ---------------------------------------------------------------------------
// Component
//
// Cancel reuses the existing generic PATCH (state="canceled" +
// cancellationReason) rather than a new endpoint -- see
// usePatchCsmCaseCallRequest.
// ---------------------------------------------------------------------------

export function CancelCallDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: CancelCallDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  // Reset the reason whenever the target call request changes, using the
  // React-recommended "adjust state during render" pattern instead of an
  // effect (avoids an extra render pass for a same-frame reset).
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setReason("");
  }

  const handleClose = () => {
    setReason("");
    onClose();
  };

  const canSubmit = reason.trim().length > 0;

  const handleSubmit = () => {
    if (!canSubmit) return;
    onSubmit(reason.trim());
  };

  return (
    <Dialog open={!!callRequest} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Cancel call request</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          <TextField
            label="Cancellation reason"
            multiline
            minRows={2}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            fullWidth
            required
            disabled={submitting}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={handleClose} disabled={submitting}>
          Back
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={handleSubmit}
          disabled={!canSubmit || submitting}
          loading={submitting}
        >
          Cancel request
        </Button>
      </DialogActions>
    </Dialog>
  );
}
