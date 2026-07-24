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

import { Button, FormControl, MenuItem, Select } from "@wso2/oxygen-ui";
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
}: CaseActionBarProps) {
  const targets = caseDetail.nextStates.filter((s) => s !== "reopened");

  if (targets.length === 0) return null;

  const runTarget = (target: CaseState) => {
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
      <Button
        variant="contained"
        size="small"
        disabled={isPending}
        onClick={() => runTarget(targets[0])}
        sx={{ borderRadius: 999 }}
      >
        {labelFor(targets[0])}
      </Button>
    );
  }

  return (
    <FormControl size="small" sx={{ minWidth: 150, flexShrink: 0 }}>
      <Select
        value=""
        displayEmpty
        disabled={isPending}
        renderValue={() => "Change state"}
        sx={{
          color: "#fff",
          borderRadius: 999,
          // Reuse the theme's own `gradient.primary` token (same one MuiButton/MuiFab
          // containedPrimary use) instead of hand-deriving a gradient from
          // palette.primary.light/main/dark — that diverged in direction, stops, and
          // didn't track the theme (e.g. the high-contrast theme sets gradient.primary
          // to "none").
          background: (theme) => `${theme.gradient.primary} !important`,
          "& .MuiSelect-icon": {
            color: "inherit",
          },
          "& .MuiOutlinedInput-notchedOutline": {
            border: "none",
            borderRadius: 999,
          },
          "&:hover": {
            background: (theme) => `${theme.gradient.primary} !important`,
            opacity: 0.9,
          },
          "&.Mui-focused .MuiOutlinedInput-notchedOutline": {
            border: "none",
          },
          "&.Mui-disabled": {
            color: "action.disabled",
            background: "action.disabledBackground",
            opacity: 1,
          },
        }}
        onChange={(e) => runTarget(e.target.value as CaseState)}
      >
        {targets.map((target) => (
          <MenuItem key={target} value={target}>
            {labelFor(target)}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
