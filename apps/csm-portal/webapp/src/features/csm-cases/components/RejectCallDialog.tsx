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

export interface RejectCallDialogProps {
  callRequest: BeCallRequestView | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (reason?: string) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function RejectCallDialog({
  callRequest,
  submitting,
  error,
  onClose,
  onSubmit,
}: RejectCallDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  // Reset the reason whenever the target call request changes (render-time
  // state adjustment, not an effect -- see React docs on this pattern).
  const [prevTargetId, setPrevTargetId] = useState(callRequest?.id);
  if (callRequest?.id !== prevTargetId) {
    setPrevTargetId(callRequest?.id);
    setReason("");
  }

  const handleClose = () => {
    setReason("");
    onClose();
  };

  const handleSubmit = () => {
    onSubmit(reason.trim() ? reason.trim() : undefined);
  };

  return (
    <Dialog open={!!callRequest} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>Reject call request</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
          <Typography variant="body2" color="text.secondary">
            This declines the request on behalf of the team. The customer will
            see it as rejected.
          </Typography>
          <TextField
            label="Reason (optional)"
            multiline
            minRows={2}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            fullWidth
            disabled={submitting}
          />
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={handleClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={handleSubmit}
          disabled={submitting}
          loading={submitting}
        >
          Reject
        </Button>
      </DialogActions>
    </Dialog>
  );
}
