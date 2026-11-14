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
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";
import { stateLabel } from "@features/csm-dashboard/utils/abtDashboard";

/** Every state a closed case may be reopened into — everything but `closed`
 * itself, matching the backend's own gate (`CaseHandler.PatchCase`: the one
 * rejected target for this transition is the case's current state). */
const REOPEN_TARGETS: CaseState[] = [
  "open",
  "work_in_progress",
  "waiting_on_wso2",
  "awaiting_info",
  "solution_proposed",
  "reopened",
];

interface ReopenCaseDialogProps {
  /** True while the reopen PATCH is in flight. */
  isReopening: boolean;
  onClose: () => void;
  /** Apply the reopen (`PATCH { state, reopenReason }`). */
  onReopen: (target: CaseState, reason: string) => void;
}

/**
 * Admin-only "reopen a closed case" dialog — only rendered when
 * `CaseActionBar`'s secondary menu offers `reopen_case`, itself gated on
 * `isAdmin` there. Closed is otherwise terminal for every state-change
 * button on this page (driven by the backend's own `nextStates`, which stays
 * empty for closed regardless of caller), so this is a deliberately separate
 * control, not an extension of the lifecycle button bar.
 *
 * `reason` is optional and, per the backend's own contract
 * (`UpdateCaseRequest.ReopenReason`), write-only: it is stored for audit
 * purposes only and never read back by this or any other screen, so this
 * dialog has no "current reason" to display either.
 */
export default function ReopenCaseDialog({
  isReopening,
  onClose,
  onReopen,
}: ReopenCaseDialogProps): JSX.Element {
  const [target, setTarget] = useState<CaseState>("work_in_progress");
  const [reason, setReason] = useState("");

  return (
    <Dialog open onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>Reopen case</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          This moves the case out of Closed. No reopen notification is sent
          to watchers, and SLA clocks are not recomputed.
        </Typography>

        <FormControl fullWidth size="small" sx={{ mb: 2 }}>
          <InputLabel id="reopen-case-target-label">New state</InputLabel>
          <Select
            labelId="reopen-case-target-label"
            label="New state"
            value={target}
            disabled={isReopening}
            onChange={(e: SelectChangeEvent<string>) =>
              setTarget(e.target.value as CaseState)
            }
          >
            {REOPEN_TARGETS.map((s) => (
              <MenuItem key={s} value={s}>
                {stateLabel(s)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <TextField
          label="Reason (optional)"
          placeholder="Why is this case being reopened?"
          multiline
          minRows={3}
          fullWidth
          size="small"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          disabled={isReopening}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={isReopening}>
          Cancel
        </Button>
        <Button
          variant="contained"
          disabled={isReopening}
          loading={isReopening}
          onClick={() => onReopen(target, reason.trim())}
        >
          Reopen case
        </Button>
      </DialogActions>
    </Dialog>
  );
}
