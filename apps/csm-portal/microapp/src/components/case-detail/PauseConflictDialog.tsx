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

import { Button, Dialog, Stack, Typography } from "@wso2/oxygen-ui";
import type { MyOngoingCase } from "@src/services/cases";
import { DialogPaper } from "@components/common/DialogPaper";

interface PauseConflictDialogProps {
  otherCases: MyOngoingCase[];
  isSubmitting: boolean;
  onConfirm: () => void;
  onDecline: () => void;
}

/**
 * The backend allows one `ongoing` case per engineer — resuming/starting work on a case while
 * another is still ongoing 409s ("the assigned engineer already has an Ongoing case: <number>").
 * Mirrors the webapp's pause-conflict dialog (CsmCaseDetailPage.tsx's onConfirmStartWork /
 * onDeclineStartWork): offers to pause the other case(s) and make this one active, or leave this
 * case not-ongoing and the other(s) untouched.
 */
export function PauseConflictDialog({ otherCases, isSubmitting, onConfirm, onDecline }: PauseConflictDialogProps) {
  const plural = otherCases.length > 1;
  // A null id means that case's UUID couldn't be resolved (the search that would find it has
  // been unreliable) — it can't be auto-paused from here.
  const unresolved = otherCases.filter((c) => !c.id);

  return (
    <Dialog
      open
      // Ignore backdrop-click/Escape dismissal while the pause+continue mutation is in flight —
      // otherwise the dialog closes (and pendingOngoingAction is dropped) while the request it
      // represents is still running in the background.
      onClose={() => {
        if (!isSubmitting) onDecline();
      }}
      slots={{ paper: DialogPaper }}
      slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 1.5, m: 2 } } }}
    >
      <Typography variant="h6" fontWeight={650}>
        Pause {plural ? "other ongoing cases" : "the other ongoing case"}?
      </Typography>

      <Typography variant="body2" color="text.secondary">
        You're already actively working {plural ? "these cases" : "this case"}:
      </Typography>

      <Stack gap={0.5}>
        {otherCases.map((c) => (
          <Typography key={c.id ?? c.label} variant="body2" fontWeight={500}>
            {c.label}
          </Typography>
        ))}
      </Stack>

      {unresolved.length > 0 ? (
        <Typography variant="body2" color="warning.main">
          Couldn&apos;t automatically look {unresolved.length > 1 ? "these" : "this"} up. Open{" "}
          {unresolved.map((c) => c.label).join(", ")} from Support and pause the work there first, then try again here.
        </Typography>
      ) : (
        <Typography variant="body2" color="text.secondary">
          Only one case can be ongoing at a time. Pause {plural ? "them" : "it"} and make this case your active one?
        </Typography>
      )}

      <Stack direction="row" justifyContent="end" gap={1}>
        <Button variant="outlined" onClick={onDecline} disabled={isSubmitting}>
          Cancel
        </Button>
        {unresolved.length === 0 && (
          <Button variant="contained" onClick={onConfirm} disabled={isSubmitting}>
            Pause &amp; continue
          </Button>
        )}
      </Stack>
    </Dialog>
  );
}
