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
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

/** A complete, minimal case-detail fixture; tests override state/nextStates. */
const BASE_CASE: CsmCaseDetail = {
  id: "case-1001",
  caseNumber: "CS-1001",
  wso2CaseId: "ACMESUB-1001",
  subject: "Identity Server token issuance latency spike",
  customer: "Acme Financial",
  accountId: "acc-001",
  projectId: "prj-acme-iam-prod",
  projectName: "IAM Production",
  product: "WSO2 Identity Server",
  severity: "S1",
  state: "work_in_progress",
  workState: "ongoing",
  assignee: "Jane Doe",
  assigneeIsMe: true,
  slaClockType: "first_response",
  minutesToBreach: 120,
  hasSla: false,
  createdAt: "2026-01-01T00:00:00.000Z",
  updatedAt: "2026-01-01T01:00:00.000Z",
  description: "Token issuance latency has spiked.",
  assignmentGroup: "grp.cre_team",
  customerContext: {
    accountName: "Acme Financial",
    tier: "enterprise",
    region: "us-east-1",
    primaryContact: "Jane Doe",
    primaryContactEmail: "jane.doe@example.com",
    accountManager: "John Roe",
    openCases: 1,
  },
  productContext: {
    product: "WSO2 Identity Server",
    version: "7.1.0",
    deployment: "IAM Production",
    environment: "prod",
  },
  watchers: [],
  linkedItems: [],
  tags: [],
  timeLogs: [],
  audit: [],
  attachments: [],
  isWatching: false,
};

/**
 * Build a case-detail fixture in a given state with an explicit `nextStates`.
 */
function caseInState(
  state: CaseState,
  nextStates: CaseState[] | undefined,
): CsmCaseDetail {
  return { ...BASE_CASE, state, nextStates };
}

describe("CaseActionBar — nextStates-driven buttons", () => {
  it("renders one button per backend nextState, labelled by the target state", () => {
    // The reported bug: a solution_proposed case returns
    // nextStates [closed, waiting_on_wso2] but only one button showed. Both
    // must appear, each named after the backend state it moves into.
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", [
          "closed",
          "waiting_on_wso2",
        ])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /waiting on wso2/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^closed$/i })).toBeInTheDocument();
  });

  it("shows exactly the transitions the backend permits, nothing more", () => {
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", [
          "solution_proposed",
          "awaiting_info",
        ])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /solution proposed/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /awaiting info/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /waiting on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^closed$/i })).not.toBeInTheDocument();
  });

  it("labels a target the same regardless of source state (no UI-invented verbs)", () => {
    // From WIP, moving to waiting_on_wso2...
    const { unmount } = render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["waiting_on_wso2"])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /waiting on wso2/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /resume/i })).not.toBeInTheDocument();
    unmount();

    // ...and from a paused state, the SAME target reads the same — "Waiting on
    // WSO2", not a fabricated "Resume work".
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /waiting on wso2/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /resume/i })).not.toBeInTheDocument();
  });

  it("dispatches the action with the backend nextState as the PATCH target", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={onAction}
      />,
    );
    // "Waiting on WSO2" has no confirm dialog, so it dispatches immediately, and
    // the target must be the real backend nextState.
    fireEvent.click(screen.getByRole("button", { name: /waiting on wso2/i }));
    expect(onAction).toHaveBeenCalledWith("wait_on_wso2", "waiting_on_wso2");
  });

  it("gates a customer-notifying transition behind a confirm dialog before dispatch", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["closed"])}
        onAction={onAction}
      />,
    );
    // Clicking "Closed" must NOT dispatch yet — it opens a confirm dialog.
    fireEvent.click(screen.getByRole("button", { name: /^closed$/i }));
    expect(onAction).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    // Confirming dispatches with the close action + closed target.
    fireEvent.click(screen.getByRole("button", { name: /close case/i }));
    expect(onAction).toHaveBeenCalledWith("close", "closed");
  });

  it("renders no lifecycle buttons when nextStates is absent (no client-side graph)", () => {
    // The bar is driven solely by the backend `nextStates`; there is no longer a
    // duplicated client-side fallback graph, so an absent field yields only the
    // state-independent "More" overflow.
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", undefined)}
        onAction={() => {}}
      />,
    );
    expect(screen.queryByRole("button", { name: /solution proposed/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /awaiting info/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /waiting on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^closed$/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /more/i })).toBeInTheDocument();
  });

  it("renders a usable button for a state it has no curated config for", () => {
    // Rollout skew / a newly added backend state: the bar must still render the
    // transition (humanized label, neutral styling) and dispatch correctly,
    // rather than building a broken button — so a new state needs no FE change.
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", [
          "pending_review" as CaseState,
        ])}
        onAction={onAction}
      />,
    );
    const button = screen.getByRole("button", { name: /pending review/i });
    expect(button).toBeInTheDocument();
    // The generic transition action drives only the toast; the PATCH target is
    // the backend state itself, so the transition still works.
    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledWith("transition", "pending_review");
  });

  it("shows no state-change buttons when nextStates is empty (terminal case)", () => {
    render(
      <CaseActionBar
        caseDetail={caseInState("closed", [])}
        onAction={() => {}}
      />,
    );
    expect(screen.queryByRole("button", { name: /waiting on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reopened/i })).not.toBeInTheDocument();
    // The state-independent "More" overflow is unaffected.
    expect(screen.getByRole("button", { name: /more/i })).toBeInTheDocument();
  });
});

describe("CaseActionBar — reassign gating for WIP-Ongoing", () => {
  /** Open the "More" overflow and return the reassign engineer menu item. */
  function openReassignItem(): HTMLElement {
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    return screen.getByRole("menuitem", { name: /assign \/ reassign engineer/i });
  }

  it("disables reassign when the case is Work in progress + Ongoing", () => {
    // The backend silently reverts an assignee change on a WIP-Ongoing case and
    // still returns success, so the action must be gated rather than fired.
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("work_in_progress", ["solution_proposed"]),
          workState: "ongoing",
        }}
        onAction={onAction}
      />,
    );
    const item = openReassignItem();
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("keeps reassign enabled when the WIP case is paused (not ongoing)", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("work_in_progress", ["solution_proposed"]),
          workState: "paused",
        }}
        onAction={onAction}
      />,
    );
    const item = openReassignItem();
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "reassign_engineer" });
  });

  it("keeps reassign enabled for a non-WIP state regardless of workState", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("awaiting_info", ["waiting_on_wso2"]),
          workState: "ongoing",
        }}
        onAction={onAction}
      />,
    );
    const item = openReassignItem();
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "reassign_engineer" });
  });
});
