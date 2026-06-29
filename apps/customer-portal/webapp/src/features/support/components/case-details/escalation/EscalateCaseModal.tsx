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

import type { EscalateCaseModalProps } from "@features/support/types/supportComponents";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  TextField,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { AlertCircle, TriangleAlert, X } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useState, type ChangeEvent, type JSX } from "react";
import { usePostCaseEscalation } from "@features/support/api/usePostCaseEscalation";
import { ESCALATION_NEXT_LEVEL } from "@features/support/constants/supportConstants";

const INLINE_ERROR_STATUSES = new Set([400, 403, 404, 409]);

/**
 * Modal for escalating a case to the next escalation level.
 * Shows current/next level, who will be notified, and a mandatory reason textarea.
 * HTTP 4xx errors are shown inline; 500+ errors are forwarded via onError.
 *
 * @param {EscalateCaseModalProps} props - Modal control props.
 * @returns {JSX.Element} The escalate case modal.
 */
export default function EscalateCaseModal({
  open,
  caseId,
  escalationLevelId,
  escalationLevelLabel,
  onClose,
  onSuccess,
  onError,
}: EscalateCaseModalProps): JSX.Element {
  const theme = useTheme();
  const [reason, setReason] = useState("");
  const [inlineError, setInlineError] = useState<string | null>(null);

  const { mutate, isPending } = usePostCaseEscalation(caseId);
  const nextLevel = ESCALATION_NEXT_LEVEL[escalationLevelId];

  const resetAndClose = useCallback(() => {
    setReason("");
    setInlineError(null);
    onClose();
  }, [onClose]);

  const handleClose = useCallback(() => {
    if (isPending) return;
    resetAndClose();
  }, [isPending, resetAndClose]);

  const handleReasonChange = useCallback(
    (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      setReason(e.target.value);
      if (inlineError) setInlineError(null);
    },
    [inlineError],
  );

  const handleConfirm = useCallback(() => {
    if (!reason.trim() || isPending || !nextLevel) return;
    setInlineError(null);
    mutate(
      { reason: reason.trim() },
      {
        onSuccess: () => {
          resetAndClose();
          onSuccess?.();
        },
        onError: (err) => {
          if (INLINE_ERROR_STATUSES.has(err.status)) {
            setInlineError(err.message);
          } else {
            const msg = err.message ?? "Failed to escalate case. Please try again.";
            onError?.(msg);
            resetAndClose();
          }
        },
      },
    );
  }, [reason, isPending, nextLevel, mutate, resetAndClose, onSuccess, onError]);

  const canConfirm = reason.trim() !== "" && !!nextLevel && !isPending;

  const amberColor = theme.palette.warning.main;
  const amberLight = theme.palette.warning.light;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      maxWidth="sm"
      fullWidth
      aria-labelledby="escalate-case-modal-title"
    >
      <IconButton
        aria-label="Close"
        size="small"
        onClick={handleClose}
        disabled={isPending}
        sx={{ position: "absolute", right: 12, top: 12, zIndex: 1 }}
      >
        <X size={18} />
      </IconButton>

      <DialogTitle
        id="escalate-case-modal-title"
        sx={{ pr: 6, pb: 0.5 }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <TriangleAlert size={22} color={amberColor} />
          <Typography variant="h6" component="span" fontWeight={700}>
            Escalate to {nextLevel?.notifiedLabel ?? "Next Level"} Escalation
          </Typography>
        </Box>
        <Typography
          variant="body2"
          color="text.secondary"
          sx={{ mt: 0.5, fontWeight: "normal" }}
        >
          This will escalate your case for higher-level attention and faster resolution.
        </Typography>
      </DialogTitle>

      <DialogContent sx={{ pt: 1.5 }}>
        {inlineError && (
          <Alert severity="error" onClose={() => setInlineError(null)} sx={{ mb: 2 }}>
            {inlineError}
          </Alert>
        )}

        {/* Escalation level info card */}
        <Box
          sx={{
            border: 1,
            borderColor: "divider",
            borderRadius: 2,
            p: 2,
            mb: 2.5,
          }}
        >
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              mb: 1,
            }}
          >
            <Typography variant="body2" color="text.secondary">
              Current Level:
            </Typography>
            <Chip
              label={escalationLevelLabel}
              size="small"
              variant="outlined"
              sx={{ fontWeight: 600, fontSize: "0.75rem", height: 24 }}
            />
          </Box>
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography variant="body2" color="text.secondary">
              Next Level:
            </Typography>
            <Chip
              label={nextLevel?.nextLabel ?? "—"}
              size="small"
              variant="outlined"
              sx={{
                fontWeight: 600,
                fontSize: "0.75rem",
                height: 24,
                color: amberColor,
                borderColor: alpha(amberColor, 0.5),
                bgcolor: alpha(amberLight, 0.12),
              }}
            />
          </Box>

          <Divider sx={{ my: 1.5 }} />

          <Typography variant="body2" fontWeight={600} sx={{ mb: 0.75 }}>
            Who will be notified:
          </Typography>
          <Typography variant="body2" color="text.secondary">
            • {nextLevel?.nextLabel} – {nextLevel?.notifiedLabel}
          </Typography>
        </Box>

        {/* Reason field */}
        <Typography variant="body2" fontWeight={600} sx={{ mb: 1 }}>
          Reason for Escalation{" "}
          <Box component="span" sx={{ color: "error.main" }}>
            *
          </Box>
        </Typography>
        <TextField
          id="escalation-reason"
          placeholder="Please provide detailed context about why this case needs escalation. Include any business impact, urgency, or specific issues that require higher-level attention..."
          value={reason}
          onChange={handleReasonChange}
          fullWidth
          multiline
          rows={5}
          disabled={isPending}
          inputProps={{ "aria-label": "Reason for escalation" }}
          sx={{ mb: 2 }}
        />

        {/* What happens next info box */}
        <Box
          sx={{
            bgcolor: alpha(amberLight, 0.18),
            border: 1,
            borderColor: alpha(amberColor, 0.3),
            borderRadius: 2,
            p: 1.5,
            display: "flex",
            gap: 1.25,
            alignItems: "flex-start",
          }}
        >
          <AlertCircle size={18} color={amberColor} style={{ flexShrink: 0, marginTop: 2 }} />
          <Box>
            <Typography variant="body2" fontWeight={700} color="warning.dark" sx={{ mb: 0.5 }}>
              What happens next?
            </Typography>
            <Typography variant="body2" color="warning.dark">
              • {nextLevel?.notifiedLabel} will be notified immediately
            </Typography>
          </Box>
        </Box>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2.5, gap: 1 }}>
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleConfirm}
          disabled={!canConfirm}
          startIcon={
            isPending ? (
              <CircularProgress size={16} color="inherit" />
            ) : (
              <TriangleAlert size={16} />
            )
          }
          sx={{
            bgcolor: amberColor,
            color: theme.palette.warning.contrastText,
            "&:hover": { bgcolor: theme.palette.warning.dark },
            "&:disabled": { bgcolor: alpha(amberColor, 0.4) },
          }}
        >
          {isPending ? "Escalating..." : "Confirm Escalation"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
