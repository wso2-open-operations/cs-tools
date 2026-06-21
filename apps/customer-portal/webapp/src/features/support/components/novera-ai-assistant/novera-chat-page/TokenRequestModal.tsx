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
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useState, type JSX } from "react";

export type TokenRequestModalProps = {
  open: boolean;
  onClose: () => void;
  onSubmit: (reason: string) => Promise<void>;
};

/**
 * Modal for requesting a session token limit increase with a reason.
 *
 * @param {TokenRequestModalProps} props - Component props.
 * @returns {JSX.Element} The rendered modal.
 */
export default function TokenRequestModal({
  open,
  onClose,
  onSubmit,
}: TokenRequestModalProps): JSX.Element {
  const [reason, setReason] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setReason("");
      setIsSubmitting(false);
      setSubmitted(false);
      setSubmitError(null);
    }
  }, [open]);

  const handleClose = () => {
    if (isSubmitting) return;
    setReason("");
    setSubmitted(false);
    onClose();
  };

  const handleSubmit = async () => {
    if (!reason.trim() || isSubmitting) return;
    setIsSubmitting(true);
    setSubmitError(null);
    try {
      await onSubmit(reason.trim());
      setSubmitted(true);
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "Failed to submit request. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Request More Session Tokens</DialogTitle>
      <DialogContent>
        {submitted ? (
          <Typography variant="body2" sx={{ mt: 1 }}>
            Your request has been submitted. Our team will review it and get back to you.
          </Typography>
        ) : (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2, mt: 1 }}>
              You have reached your session token limit. Please provide a reason for requesting
              additional tokens and our team will review your request.
            </Typography>
            <TextField
              label="Reason"
              placeholder="Describe why you need more tokens for this session..."
              multiline
              rows={4}
              fullWidth
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              disabled={isSubmitting}
              autoFocus
            />
            {submitError && (
              <Typography variant="body2" color="error" sx={{ mt: 1 }}>
                {submitError}
              </Typography>
            )}
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={isSubmitting} variant="outlined">
          {submitted ? "Close" : "Cancel"}
        </Button>
        {!submitted && (
          <Button
            onClick={() => void handleSubmit()}
            disabled={!reason.trim() || isSubmitting}
            loading={isSubmitting}
            variant="contained"
          >
            Submit Request
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}
