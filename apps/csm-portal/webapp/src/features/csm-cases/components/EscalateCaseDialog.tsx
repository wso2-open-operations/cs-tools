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
import EscalationLevelChip from "@components/EscalationLevelChip";
import { escalationLevelLabel } from "@features/csm-cases/utils/escalationLevel";

interface EscalateCaseDialogProps {
  /** Which action this confirmation is for — drives the copy and whether a
   * reason is required. */
  action: "ESCALATE" | "DEESCALATE";
  /** Raw escalation-level id ("0"-"5") the case is at right now. */
  currentLevel: string;
  isSaving: boolean;
  /** Message from the last failed attempt, if any — shown inline so the
   * dialog stays open with the reason text preserved on error. */
  errorMessage?: string | null;
  onClose: () => void;
  onSave: (reason: string | undefined) => void;
}

/**
 * Confirms an escalate or de-escalate step (`POST /cases/{id}/escalations`)
 * and collects the reason — required when escalating, optional when
 * de-escalating (matches the backend's own validation, which rejects an
 * escalate call with no reason). Plain-text `reason` only (a `TextField`,
 * not a rich-text editor) — there's no evidence this field needs to support
 * HTML, and rendering it as plain text elsewhere avoids needing to sanitize
 * it at all.
 */
export default function EscalateCaseDialog({
  action,
  currentLevel,
  isSaving,
  errorMessage,
  onClose,
  onSave,
}: EscalateCaseDialogProps): JSX.Element {
  const [reason, setReason] = useState("");
  const isEscalate = action === "ESCALATE";
  const trimmedReason = reason.trim();
  const canSubmit = isEscalate ? trimmedReason.length > 0 : true;

  const handleSubmit = (): void => {
    if (!canSubmit) return;
    onSave(trimmedReason.length > 0 ? trimmedReason : undefined);
  };

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{isEscalate ? "Escalate case" : "De-escalate case"}</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="body2" color="text.secondary">
              Current level
            </Typography>
            <EscalationLevelChip level={currentLevel} />
          </Box>
          <Typography variant="body2">
            {isEscalate
              ? `This raises the case to ${escalationLevelLabel(String(Number(currentLevel) + 1))}.`
              : `This lowers the case to ${escalationLevelLabel(String(Math.max(0, Number(currentLevel) - 1)))}.`}
          </Typography>
          <TextField
            label="Reason"
            placeholder={
              isEscalate
                ? "Why is this case being escalated?"
                : "Why is this case being de-escalated? (optional)"
            }
            multiline
            minRows={3}
            fullWidth
            autoFocus
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            disabled={isSaving}
            required={isEscalate}
            error={isEscalate && reason.length > 0 && trimmedReason.length === 0}
            helperText={
              isEscalate ? "Required when escalating." : undefined
            }
          />
          {errorMessage && (
            <Typography variant="body2" color="error">
              {errorMessage}
            </Typography>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={!canSubmit || isSaving}
          loading={isSaving}
          onClick={handleSubmit}
        >
          {isEscalate ? "Escalate" : "De-escalate"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
