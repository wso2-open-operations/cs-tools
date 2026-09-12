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

import { Button } from "@wso2/oxygen-ui";
import { ArrowRight, CheckCircle } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { getNextProblemTransition, problemStateLabel } from "@features/csm-operations/utils/problems";
import type { BeProblemDetail } from "@api/backend/types";

interface ProblemActionBarProps {
  problem: BeProblemDetail;
  /** True while a `PATCH /problems/{id}` transition is in flight. */
  isPending: boolean;
  /**
   * Fired with the `transition` key (`assess`/`confirm`/`fix`/`resolve`/
   * `close`) the engineer picked. The caller decides how to apply it — a
   * direct `PATCH { transition }` for most, or (for `fix`) opening a dialog
   * to optionally collect `causeNotes`/`fixNotes` first — this bar has no
   * opinion on that, same split of responsibility as `IncidentActionBar` +
   * `CsmIncidentDetailPage.onIncidentAction`.
   */
  onAction: (transition: string) => void;
}

/**
 * Lifecycle action bar for the problem detail page. Unlike
 * `IncidentActionBar`/`ChangeRequestActionBar`, a problem's live state
 * machine is a strictly linear forward chain (`New -> Assess -> Root Cause
 * Analysis -> Fix in Progress -> Resolved -> Closed`, one legal next step at
 * a time — see `getNextProblemTransition`), so there's never more than one
 * button and no dropdown menu is needed. Renders nothing once the problem is
 * `Closed` (terminal) or in a state with no modeled forward transition.
 */
export default function ProblemActionBar({
  problem,
  isPending,
  onAction,
}: ProblemActionBarProps): JSX.Element | null {
  const next = getNextProblemTransition(problem.state);
  if (!next) return null;

  const isTerminalTarget = next.target === "CLOSED" || next.target === "RESOLVED";

  return (
    <Button
      size="small"
      variant="contained"
      color={isTerminalTarget ? "success" : "primary"}
      startIcon={isTerminalTarget ? <CheckCircle size={16} /> : <ArrowRight size={16} />}
      disabled={isPending}
      onClick={() => onAction(next.transition)}
    >
      Move to {problemStateLabel(next.target)}
    </Button>
  );
}
