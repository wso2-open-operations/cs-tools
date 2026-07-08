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
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { BeCaseCause, BeCaseResolutionCode } from "@api/backend/types";
import {
  CASE_CAUSES,
  RESOLUTION_CODES,
  humanizeResolutionEnum,
} from "@features/csm-cases/utils/caseResolution";

interface ResolutionDialogProps {
  /** Which lifecycle transition this dialog is confirming. */
  kind: "close" | "propose_solution";
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (fields: {
    resolutionCode: BeCaseResolutionCode;
    cause: BeCaseCause;
    closeNotes: string;
  }) => void;
}

const COPY: Record<
  ResolutionDialogProps["kind"],
  { title: string; body: string; confirmLabel: string }
> = {
  close: {
    title: "Close this case",
    body: "The customer receives a closure notification and the case moves to “Closed”. Record the Post Resolution Activity before closing.",
    confirmLabel: "Close case",
  },
  propose_solution: {
    title: "Propose a solution",
    body: "The customer is notified that a solution has been proposed and the case moves to “Solution proposed”. Record the Post Resolution Activity before proposing.",
    confirmLabel: "Propose solution",
  },
};

/**
 * Collects the Post Resolution Activity (ISSU-026) — resolution code, root
 * cause, and free-text close notes — before closing a case or proposing a
 * solution. The backend accepts these as optional fields alongside `state:
 * "closed"` or `"solution_proposed"` on `PATCH /cases/{id}`; this dialog is
 * the only place in the portal that collects them, and it doubles as the
 * confirmation step for these two customer-notifying transitions (there is
 * no separate plain confirm dialog for them — see CaseActionBar).
 */
export default function ResolutionDialog({
  kind,
  isSubmitting,
  onClose,
  onSubmit,
}: ResolutionDialogProps): JSX.Element {
  const [resolutionCode, setResolutionCode] = useState<BeCaseResolutionCode | "">("");
  const [cause, setCause] = useState<BeCaseCause | "">("");
  const [closeNotes, setCloseNotes] = useState("");
  const copy = COPY[kind];
  const canSubmit = !!resolutionCode && !!cause && !isSubmitting;

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{copy.title}</DialogTitle>
      <DialogContent
        dividers
        sx={{ display: "flex", flexDirection: "column", gap: 2 }}
      >
        <Typography variant="body2" color="text.secondary">
          {copy.body}
        </Typography>

        <FormControl fullWidth size="small" required>
          <InputLabel id="resolution-code-label">Resolution code</InputLabel>
          <Select
            labelId="resolution-code-label"
            label="Resolution code"
            value={resolutionCode}
            onChange={(e) =>
              setResolutionCode(e.target.value as BeCaseResolutionCode)
            }
          >
            {RESOLUTION_CODES.map((code) => (
              <MenuItem key={code} value={code}>
                {humanizeResolutionEnum(code)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <FormControl fullWidth size="small" required>
          <InputLabel id="case-cause-label">Cause</InputLabel>
          <Select
            labelId="case-cause-label"
            label="Cause"
            value={cause}
            onChange={(e) => setCause(e.target.value as BeCaseCause)}
          >
            {CASE_CAUSES.map((c) => (
              <MenuItem key={c} value={c}>
                {humanizeResolutionEnum(c)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <TextField
          label="Close notes"
          size="small"
          fullWidth
          multiline
          minRows={3}
          value={closeNotes}
          onChange={(e) => setCloseNotes(e.target.value)}
          placeholder="Optional free-text notes about the resolution…"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color={kind === "close" ? "warning" : "primary"}
          disabled={!canSubmit}
          onClick={() => {
            if (!resolutionCode || !cause) return;
            onSubmit({ resolutionCode, cause, closeNotes });
          }}
        >
          {copy.confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
