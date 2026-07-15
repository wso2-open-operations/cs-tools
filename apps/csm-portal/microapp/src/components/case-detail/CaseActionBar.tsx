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

import { Fragment, useState } from "react";
import { Button, Card, Dialog, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import type { CaseDetail, CaseState } from "@src/types";
import { STATE_LABELS } from "@components/support/config";

interface CaseActionBarProps {
  caseDetail: CaseDetail;
  currentUserId: string | null | undefined;
  isPending: boolean;
  /** Bare `PATCH { state }` — for any target that isn't `work_in_progress` or a
   * resolution-collecting target (`closed`/`solution_proposed`). */
  onTransition: (target: CaseState) => void;
  /** Any transition into `work_in_progress` — assigned-to-me-already or not. Mirrors the webapp's
   * onAction: `targetState === "work_in_progress"` always routes through `startWork()`
   * unconditionally, never a bare state PATCH, since starting is also the point where the case
   * should go `ongoing` (checked for the single-active-case conflict first — see
   * CaseDetailPage's attemptGoOngoing). The handler itself decides whether an `assigneeEmail`
   * PATCH is needed first, based on whether the case is already the caller's. */
  onAssignAndStart: () => void;
  /** `closed` / `solution_proposed`: these may carry `resolutionCode`/`cause`/`closeNotes`
   * alongside `state`, so the caller opens a dialog to collect them before PATCHing. */
  onNeedsResolution: (target: "closed" | "solution_proposed") => void;
  /** `PATCH { workState }` — toggles ongoing/paused. Only offered to the case's own assignee
   * while it's `work_in_progress`, mirroring the webapp's gating (pausing/resuming is the
   * assignee's own workflow control, not something to do to someone else's active case). This is
   * also the *only* way to reach `ongoing` — without it, an assigned-but-never-started case (or
   * one with no workState at all) has no path to satisfying the comment gate in CommentComposer. */
  onToggleWorkState: () => void;
}

/**
 * Lifecycle action bar for the case detail page. Mirrors the webapp's CaseActionBar: buttons are
 * driven entirely by the backend's `nextStates` on the case — no separate state graph is
 * maintained here. `reopened` is excluded: the backend puts it in a closed case's `nextStates`
 * only as a signal that a related case may still be created (there's no real reopen transition),
 * and the microapp has no "create related case" flow yet.
 */
export function CaseActionBar({
  caseDetail,
  currentUserId,
  isPending,
  onTransition,
  onAssignAndStart,
  onNeedsResolution,
  onToggleWorkState,
}: CaseActionBarProps) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const targets = caseDetail.nextStates.filter((s) => s !== "reopened");

  // Only `ongoing` is pausable; anything else (paused, or a null workState on a case that was
  // never explicitly started) is resumable — otherwise the only action that can ever set
  // `ongoing` would be hidden for a case whose workState has never been touched.
  const canToggleWorkState =
    caseDetail.state === "work_in_progress" && caseDetail.assignedEngineer?.id === currentUserId;
  const isOngoing = caseDetail.workState === "ongoing";
  const workStateToggle = canToggleWorkState && (
    <Button variant="outlined" size="small" disabled={isPending} onClick={onToggleWorkState}>
      {isOngoing ? "Pause work" : "Resume work"}
    </Button>
  );

  if (targets.length === 0) return workStateToggle || null;

  const runTarget = (target: CaseState) => {
    setPickerOpen(false);
    if (target === "work_in_progress") {
      onAssignAndStart();
      return;
    }
    if (target === "closed" || target === "solution_proposed") {
      onNeedsResolution(target);
      return;
    }
    onTransition(target);
  };

  const labelFor = (target: CaseState): string =>
    target === "work_in_progress" && caseDetail.assignedEngineer?.id !== currentUserId
      ? "Assign to me"
      : STATE_LABELS[target];

  if (targets.length === 1) {
    return (
      <Fragment>
        <Button variant="contained" size="small" disabled={isPending} onClick={() => runTarget(targets[0])}>
          {labelFor(targets[0])}
        </Button>
        {workStateToggle}
      </Fragment>
    );
  }

  return (
    <>
      <Button
        variant="contained"
        size="small"
        disabled={isPending}
        endIcon={<ChevronDown size={16} />}
        onClick={() => setPickerOpen(true)}
      >
        Change state
      </Button>
      {workStateToggle}

      <Dialog
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        slots={{ paper: (props) => <Card component={Stack} {...props} /> }}
        slotProps={{ paper: { sx: { bgcolor: "background.paper", p: 1.5, gap: 1, m: 2 } } }}
      >
        <Typography variant="h6" fontWeight={650}>
          Change state
        </Typography>
        {targets.map((target) => (
          <Button key={target} variant="outlined" onClick={() => runTarget(target)} disabled={isPending}>
            {labelFor(target)}
          </Button>
        ))}
      </Dialog>
    </>
  );
}
