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

import type { ReactElement } from "react";
import {
  fireEvent,
  render as rtlRender,
  screen,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import "@testing-library/jest-dom/vitest";

// CaseActionBar renders `UserRefLink` (assignee), which resolves an unknown
// id through `useResolvedUserId`, which needs the real API client — same
// approach as CaseMetaBand.test.tsx.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn().mockResolvedValue({ users: [] }) }),
}));

import CaseActionBar from "@features/csm-cases/components/CaseActionBar";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";

/**
 * Local `render` override: every case in this file renders `CaseActionBar`,
 * which (via `UserRefLink`) needs both a `QueryClientProvider` (for
 * `useResolvedUserId`'s `useQuery`) and a router context (for the profile
 * `<Link>`) — wrapping here keeps every existing `render(<CaseActionBar ... />)`
 * call site unchanged.
 */
function render(ui: ReactElement): ReturnType<typeof rtlRender> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return rtlRender(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

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
  it("renders one menu item per backend nextState, labelled by the target state", () => {
    // The reported bug: a solution_proposed case returns
    // nextStates [closed, waiting_on_wso2] but only one button showed. Both
    // must appear, each named after the backend state it moves into. With
    // more than one reachable state the bar now consolidates them behind a
    // single "Change state" trigger (see the "advisory close-gate" describe
    // block below), so both are asserted as menu items, not top-level
    // buttons.
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", [
          "closed",
          "waiting_on_wso2",
        ])}
        onAction={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    expect(screen.getByRole("menuitem", { name: /wait on wso2/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /^close$/i })).toBeInTheDocument();
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
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    expect(screen.getByRole("menuitem", { name: /propose solution/i })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /request information/i })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /wait on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: /^close$/i })).not.toBeInTheDocument();
  });

  it("labels a target the same regardless of source state (no UI-invented verbs)", () => {
    // From WIP, moving to waiting_on_wso2...
    const { unmount } = render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["waiting_on_wso2"])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /wait on wso2/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /resume/i })).not.toBeInTheDocument();
    unmount();

    // ...and from a paused state, the SAME target reads the same — "Wait on
    // WSO2", not a fabricated "Resume work".
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /wait on wso2/i })).toBeInTheDocument();
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
    // "Wait on WSO2" has no confirm dialog, so it dispatches immediately, and
    // the target must be the real backend nextState.
    fireEvent.click(screen.getByRole("button", { name: /wait on wso2/i }));
    expect(onAction).toHaveBeenCalledWith("wait_on_wso2", "waiting_on_wso2");
  });

  it("dispatches Close immediately — confirmation happens via the Post Resolution Activity dialog upstream", () => {
    // CaseActionBar itself no longer gates close/propose-solution behind a
    // confirm dialog; CsmCaseDetailPage's onAction opens the resolution
    // dialog for these two, which doubles as the confirmation step.
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["closed"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^close$/i }));
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
    expect(screen.queryByRole("button", { name: /propose solution/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /request information/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /wait on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^close$/i })).not.toBeInTheDocument();
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
    expect(screen.queryByRole("button", { name: /wait on wso2/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /reopened/i })).not.toBeInTheDocument();
    // The state-independent "More" overflow is unaffected.
    expect(screen.getByRole("button", { name: /more/i })).toBeInTheDocument();
  });
});

describe("CaseActionBar — isPending shows a spinner and disables the click", () => {
  it("shows a spinner and disables the single primary button (e.g. Assign to me) while isPending", () => {
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("open", ["work_in_progress"]),
          assigneeIsMe: false,
        }}
        onAction={() => {}}
        isPending
      />,
    );
    const button = screen.getByRole("button", { name: /assign to me/i });
    expect(button).toBeDisabled();
    expect(button.querySelector(".MuiCircularProgress-root")).toBeInTheDocument();
  });

  it("disables the Change-state trigger while isPending, when there are multiple primary buttons", () => {
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", ["closed", "waiting_on_wso2"])}
        onAction={() => {}}
        isPending
      />,
    );
    const button = screen.getByRole("button", { name: /change state/i });
    expect(button).toBeDisabled();
    expect(button.querySelector(".MuiCircularProgress-root")).toBeInTheDocument();
  });

  it("leaves the primary button enabled with its normal icon when isPending is unset", () => {
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("open", ["work_in_progress"]),
          assigneeIsMe: false,
        }}
        onAction={() => {}}
      />,
    );
    const button = screen.getByRole("button", { name: /assign to me/i });
    expect(button).not.toBeDisabled();
    expect(button.querySelector(".MuiCircularProgress-root")).not.toBeInTheDocument();
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

describe("CaseActionBar — create related case (closed-case reopen replacement)", () => {
  it("is not offered on a closed case the backend has not flagged eligible (empty nextStates)", () => {
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={() => {}} />,
    );
    expect(
      screen.queryByRole("button", { name: /create related case/i }),
    ).not.toBeInTheDocument();
  });

  it("renders as a single primary button when the backend flags the case eligible", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("closed", ["reopened"])}
        onAction={onAction}
      />,
    );
    const button = screen.getByRole("button", { name: /create related case/i });
    expect(button).toBeInTheDocument();
    // Must dispatch the create_related_case action, never a real "reopened"
    // state PATCH — the data source has no such transition.
    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledWith("create_related_case", "reopened");
  });

  it("never renders a literal 'Reopened' state-transition button", () => {
    // Guards against regressing to the pre-fix behavior where a closed case's
    // stray `reopened` nextState rendered as a generic (broken) reopen action.
    render(
      <CaseActionBar
        caseDetail={caseInState("closed", ["reopened"])}
        onAction={() => {}}
      />,
    );
    expect(screen.queryByRole("button", { name: /^reopened$/i })).not.toBeInTheDocument();
  });
});

describe("CaseActionBar — Create incident from case / Create service request (ISSU-021)", () => {
  // Both now have a real backend flow (CsmCaseDetailPage.tsx dispatches
  // "create_incident" and "create_service_request" to their respective
  // create form's nav state), so they follow the same closed-case read-only
  // gate as every other secondary item rather than staying permanently
  // disabled. "Link to incident" used to be a third item here — it's been
  // relocated to the Related tab's LinkedIncidentWidget, see that widget's
  // own test file.
  const ITEMS: [RegExp, string][] = [
    [/create incident from case/i, "create_incident"],
    [/create service request/i, "create_service_request"],
  ];

  it.each(ITEMS)(
    "dispatches %s as a secondary action when the case is open",
    (name, expectedAction) => {
      const onAction = vi.fn();
      render(
        <CaseActionBar
          caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
          onAction={onAction}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: /more/i }));
      const item = screen.getByRole("menuitem", { name });
      expect(item).not.toHaveAttribute("aria-disabled", "true");
      fireEvent.click(item);
      expect(onAction).toHaveBeenCalledTimes(1);
      expect(onAction).toHaveBeenCalledWith({ secondary: expectedAction });
    },
  );

  it.each(ITEMS)("disables %s once the case is closed", (name) => {
    const onAction = vi.fn();
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe("CaseActionBar — Raise internal Git issue is blocked on a closed case", () => {
  it("disables the menu item once the case is closed", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /raise internal git issue/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("keeps it enabled for a case in an SN-actionable state", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("waiting_on_wso2", ["awaiting_info"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /raise internal git issue/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "raise_git_issue" });
  });

  // SN's own gate (CaseGithubIssueUtils.isCaseActionable) does not include
  // Awaiting info — mirrored here so the FE doesn't offer an action SN will
  // 409 on.
  it("disables the menu item while the case is awaiting info", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /raise internal git issue/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe("CaseActionBar — Hold auto-closure / Edit case details", () => {
  const SECONDARY_ITEMS: Array<[RegExp, string]> = [
    [/hold auto-closure/i, "hold_auto_close"],
    [/edit case details/i, "edit_case_details"],
  ];

  it.each(SECONDARY_ITEMS)(
    "dispatches %s as a secondary action when the case is open",
    (name, key) => {
      const onAction = vi.fn();
      render(
        <CaseActionBar
          caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
          onAction={onAction}
        />,
      );
      fireEvent.click(screen.getByRole("button", { name: /more/i }));
      const item = screen.getByRole("menuitem", { name });
      expect(item).not.toHaveAttribute("aria-disabled", "true");
      fireEvent.click(item);
      expect(onAction).toHaveBeenCalledWith({ secondary: key });
    },
  );

  it.each(SECONDARY_ITEMS)(
    "disables %s once the case is closed",
    (name) => {
      const onAction = vi.fn();
      render(
        <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
      );
      fireEvent.click(screen.getByRole("button", { name: /more/i }));
      const item = screen.getByRole("menuitem", { name });
      expect(item).toHaveAttribute("aria-disabled", "true");
      fireEvent.click(item);
      expect(onAction).not.toHaveBeenCalled();
    },
  );
});

describe("CaseActionBar — Set fix ETA", () => {
  const ITEMS: Array<[RegExp, string]> = [[/set fix eta/i, "set_fix_eta"]];

  it.each(ITEMS)("dispatches %s as a secondary action when the case is open", (name, key) => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: key });
  });

  it.each(ITEMS)("disables %s once the case is closed", (name) => {
    const onAction = vi.fn();
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe("CaseActionBar — Request update (enabled only in Awaiting info / Solution proposed)", () => {
  it("dispatches request_update when the case is awaiting info", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /request update/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "request_update" });
  });

  it("dispatches request_update when the case has a proposed solution", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", ["closed"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /request update/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "request_update" });
  });

  it.each<CaseState>(["work_in_progress", "waiting_on_wso2", "open", "closed"])(
    "disables request_update while the case is %s, with an explanatory tooltip",
    (state) => {
      const onAction = vi.fn();
      render(
        <CaseActionBar caseDetail={caseInState(state, [])} onAction={onAction} />,
      );
      fireEvent.click(screen.getByRole("button", { name: /more/i }));
      const item = screen.getByRole("menuitem", { name: /request update/i });
      expect(item).toHaveAttribute("aria-disabled", "true");
      fireEvent.click(item);
      expect(onAction).not.toHaveBeenCalled();
    },
  );

  it("disables request_update in an eligible state when the caller isn't the assigned engineer", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("awaiting_info", ["waiting_on_wso2"]),
          assigneeIsMe: false,
        }}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /request update/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe("CaseActionBar — advisory close-gate (open task)", () => {
  it("disables the single-button Close transition with a tooltip when closeBlockedReason is set", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["closed"])}
        onAction={onAction}
        closeBlockedReason="This case has an open task."
      />,
    );
    const button = screen.getByRole("button", { name: /^close$/i });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("leaves Close enabled when closeBlockedReason is unset", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("work_in_progress", ["closed"])}
        onAction={onAction}
      />,
    );
    const button = screen.getByRole("button", { name: /^close$/i });
    expect(button).toBeEnabled();
    fireEvent.click(button);
    expect(onAction).toHaveBeenCalledWith("close", "closed");
  });

  it("disables only Close (not the other target) in the Change-state menu", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("solution_proposed", ["closed", "waiting_on_wso2"])}
        onAction={onAction}
        closeBlockedReason="This case has an open task."
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /change state/i }));
    const closeItem = screen.getByRole("menuitem", { name: /^close$/i });
    const waitItem = screen.getByRole("menuitem", { name: /wait on wso2/i });
    expect(closeItem).toHaveAttribute("aria-disabled", "true");
    expect(waitItem).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(closeItem);
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe("CaseActionBar — Change severity is blocked on a closed case", () => {
  it("disables the menu item once the case is closed", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /change severity/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("keeps it enabled for a non-closed case", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /change severity/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "change_severity" });
  });
});

describe("CaseActionBar — Change case type is blocked on a closed case", () => {
  it("disables the menu item once the case is closed", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /change case type/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });

  it("keeps it enabled for a non-closed case", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={caseInState("awaiting_info", ["waiting_on_wso2"])}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /change case type/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "change_case_type" });
  });
});

describe("CaseActionBar — Create change request (service requests only)", () => {
  it("is offered for a service request", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{
          ...caseInState("awaiting_info", ["waiting_on_wso2"]),
          caseType: "service_request",
        }}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /create change request/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({ secondary: "create_change_request" });
  });

  it("is not offered for a plain case", () => {
    render(
      <CaseActionBar
        caseDetail={{ ...caseInState("awaiting_info", ["waiting_on_wso2"]), caseType: "case" }}
        onAction={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    expect(
      screen.queryByRole("menuitem", { name: /create change request/i }),
    ).not.toBeInTheDocument();
  });

  it("disables it once the service request is closed", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{ ...caseInState("closed", []), caseType: "service_request" }}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /create change request/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });
});

describe("acknowledge action", () => {
  const renderBar = (
    overrides: Partial<CsmCaseDetail>,
    props: { onAcknowledge?: () => void; isAcknowledging?: boolean } = {},
  ): void => {
    render(
      <CaseActionBar
        caseDetail={{ ...BASE_CASE, ...overrides }}
        onAction={vi.fn()}
        onAcknowledge={props.onAcknowledge ?? vi.fn()}
        isAcknowledging={props.isAcknowledging}
      />,
    );
  };

  it("offers acknowledge on an unacknowledged S0-S3 case", () => {
    for (const severity of ["S0", "S1", "S2", "S3"] as const) {
      const { unmount } = render(
        <CaseActionBar
          caseDetail={{ ...BASE_CASE, severity, acknowledgedBy: undefined }}
          onAction={vi.fn()}
          onAcknowledge={vi.fn()}
        />,
      );
      expect(
        screen.getByRole("button", { name: /acknowledge/i }),
        `expected the acknowledge button on a ${severity} case`,
      ).toBeInTheDocument();
      unmount();
    }
  });

  it("hides acknowledge on S4 — those cases raise no acknowledgement notification", () => {
    renderBar({ severity: "S4", acknowledgedBy: undefined });
    expect(screen.queryByRole("button", { name: /acknowledge/i })).not.toBeInTheDocument();
  });

  it("hides acknowledge once the case is acknowledged — it is first-write-wins, so there is nothing left to do", () => {
    renderBar({ severity: "S1", acknowledgedBy: { name: "Jane Doe" } });
    expect(screen.queryByRole("button", { name: /acknowledge/i })).not.toBeInTheDocument();
  });

  it("renders no acknowledge button when the caller wires no handler, rather than a dead control", () => {
    render(
      <CaseActionBar
        caseDetail={{ ...BASE_CASE, severity: "S1", acknowledgedBy: undefined }}
        onAction={vi.fn()}
      />,
    );
    expect(screen.queryByRole("button", { name: /acknowledge/i })).not.toBeInTheDocument();
  });

  it("invokes the handler on click and disables the button while in flight", () => {
    const onAcknowledge = vi.fn();
    const { unmount } = render(
      <CaseActionBar
        caseDetail={{ ...BASE_CASE, severity: "S1", acknowledgedBy: undefined }}
        onAction={vi.fn()}
        onAcknowledge={onAcknowledge}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /acknowledge/i }));
    expect(onAcknowledge).toHaveBeenCalledTimes(1);
    unmount();

    renderBar({ severity: "S1", acknowledgedBy: undefined }, { isAcknowledging: true });
    expect(screen.getByRole("button", { name: /acknowledge/i })).toBeDisabled();
  });

  it("also disables Acknowledge while a different lifecycle action's patchCase mutation is in flight", () => {
    // Guards against the two actions sharing one mutation's isPending flag:
    // a lifecycle transition (e.g. Assign to me) in flight must not leave
    // Acknowledge clickable and racing it.
    render(
      <CaseActionBar
        caseDetail={{ ...BASE_CASE, severity: "S1", acknowledgedBy: undefined }}
        onAction={vi.fn()}
        onAcknowledge={vi.fn()}
        isAcknowledging={false}
        isPending
      />,
    );
    expect(screen.getByRole("button", { name: /acknowledge/i })).toBeDisabled();
  });
});

describe("CaseActionBar — Provide / Recall workaround", () => {
  it("shows 'Provide workaround' and dispatches toggle_workaround_provided when unset", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{ ...BASE_CASE, workaroundProvidedOn: undefined }}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /provide workaround/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({
      secondary: "toggle_workaround_provided",
    });
  });

  it("shows 'Recall workaround' once the workaround is marked provided", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar
        caseDetail={{
          ...BASE_CASE,
          workaroundProvidedOn: "2026-09-02T17:11:47Z",
        }}
        onAction={onAction}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /recall workaround/i });
    expect(item).not.toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).toHaveBeenCalledWith({
      secondary: "toggle_workaround_provided",
    });
  });

  it("disables the menu item once the case is closed", () => {
    const onAction = vi.fn();
    render(
      <CaseActionBar caseDetail={caseInState("closed", [])} onAction={onAction} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /more/i }));
    const item = screen.getByRole("menuitem", { name: /provide workaround/i });
    expect(item).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(item);
    expect(onAction).not.toHaveBeenCalled();
  });
});
