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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import CaseActionBar from "@features/csm-cases/components/CaseActionBar";
import { getMockCsmCaseDetailById } from "@features/csm-cases/api/mocks/casesMocks";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

/**
 * Build a case-detail fixture in a given state with an explicit `nextStates`,
 * reusing the seeded mock so every other required field is realistic.
 */
function caseInState(
  state: CaseState,
  nextStates: CaseState[] | undefined,
): CsmCaseDetail {
  const base = getMockCsmCaseDetailById("case-1001");
  if (!base) throw new Error("mock case-1001 missing");
  return { ...base, state, nextStates };
}

describe("CaseActionBar — nextStates-driven buttons", () => {
  it("renders one button per backend nextState", () => {
    // The reported bug: a solution_proposed case returns
    // nextStates [closed, waiting_on_wso2] but only "Close" showed because the
    // resume action was hardwired to work_in_progress. Both must now appear.
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", [
          "closed",
          "waiting_on_wso2",
        ])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /resume work/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^close$/i })).toBeInTheDocument();
  });

  it("shows exactly the transitions the backend permits, nothing more", () => {
    // work_in_progress could move many ways, but the backend here only allows
    // two — so only those two buttons render.
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", [
          "solution_proposed",
          "awaiting_info",
        ])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /propose solution/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /request info/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /wait on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
  });

  it("labels the move into Waiting on WSO2 by source: pause vs resume", () => {
    // From WIP it is a deliberate pause...
    const { unmount } = render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["waiting_on_wso2"])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /wait on wso2/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /resume work/i })).not.toBeInTheDocument();
    unmount();

    // ...but from a paused state (awaiting_info) the same target reads "Resume".
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /resume work/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /wait on wso2/i })).not.toBeInTheDocument();
  });

  it("dispatches the action with the backend nextState as the PATCH target", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", [
          "closed",
          "waiting_on_wso2",
        ])}
        onAction={onAction}
      />,
    );
    // "Resume work" has no confirm dialog, so it dispatches immediately. The
    // target must be the real backend nextState (waiting_on_wso2), not a
    // re-derived guess (work_in_progress).
    fireEvent.click(screen.getByRole("button", { name: /resume work/i }));
    expect(onAction).toHaveBeenCalledWith("resume_work", "waiting_on_wso2");
  });

  it("falls back to the known graph when nextStates is absent", () => {
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", undefined)}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /propose solution/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /request info/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /wait on wso2/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^close$/i })).toBeInTheDocument();
  });

  it("shows no state-change buttons when nextStates is empty (terminal case)", () => {
    // An empty nextStates is meaningful: a closed/terminal case has no next
    // step, so it must offer none and not fall back to the graph.
    render(
      <CaseActionBar
        caseDetail={caseInState("closed", [])}
        onAction={() => {}}
      />,
    );
    expect(screen.queryByRole("button", { name: /resume work/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reopen/i })).not.toBeInTheDocument();
    // The state-independent "More" overflow is unaffected.
    expect(screen.getByRole("button", { name: /more/i })).toBeInTheDocument();
  });

  it("keeps the lead-only Reopen override on a closed case regardless of nextStates", () => {
    render(
      <CaseActionBar
        caseDetail={caseInState("closed", [])}
        onAction={() => {}}
        canReopenClosed
      />,
    );
    expect(screen.getByRole("button", { name: /reopen/i })).toBeInTheDocument();
  });
});
