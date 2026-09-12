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
import { afterEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import LogTimeCardDialog from "@features/csm-timecards/components/LogTimeCardDialog";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import { useRecentApprovers } from "@features/csm-timecards/api/useTimeSheets";
import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

// Not under test here — stubbed to a plain textarea, same technique used by
// EditCaseDetailsDialog.test.tsx for the same dependency.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <textarea
      aria-label="work-log-editor"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "engineer@example.test", given_name: "Jane" }),
}));
vi.mock("@features/csm-users/api/useSearchUsers", () => ({
  useSearchUsers: vi.fn(),
}));
// Only `useRecentApprovers` is used by the component from this module; the
// rest of it reaches the runtime-config-reading backend client at import
// time, which isn't present under vitest (same approach as
// ChangeRequestApprovals.test.tsx's `@api/backend/client` stub).
vi.mock("@features/csm-timecards/api/useTimeSheets", () => ({
  useRecentApprovers: vi.fn(),
}));

const mockedUseSearchUsers = vi.mocked(useSearchUsers);
const mockedUseRecentApprovers = vi.mocked(useRecentApprovers);

type MockUser = { id: string; name: string; userName: string; email: string };

// The component calls `useSearchUsers` twice with different intent: once for
// the live typed-search candidates, once (filters.userIds set) to re-check
// recentApprovers against current eligibility. Both calls hit this same
// mocked function, so the mock discriminates on `filters.userIds` rather than
// call order, and each call site's data is configured independently.
function setSearchUsersData(opts: { live?: MockUser[]; eligible?: MockUser[] }): void {
  mockedUseSearchUsers.mockImplementation(
    (request) =>
      ({
        data: {
          users: request.filters?.userIds ? (opts.eligible ?? []) : (opts.live ?? []),
        },
      }) as unknown as ReturnType<typeof useSearchUsers>,
  );
}

// Tests below override these with `mockReturnValue`/`mockImplementation` (not
// a `*Once` variant — the dialog re-renders more than once per interaction,
// e.g. on every keystroke into the approver search box, so a one-shot mock
// would only satisfy the first render and silently fall back to the empty
// default on the next) — reset back to the shared empty defaults afterwards
// so later tests aren't affected by an earlier test's override.
function resetMocks(): void {
  setSearchUsersData({});
  mockedUseRecentApprovers.mockReturnValue({
    data: [],
  } as unknown as ReturnType<typeof useRecentApprovers>);
}

resetMocks();
afterEach(resetMocks);

const EDITING_CARD: CsmTimeCard = {
  id: "card-1",
  caseId: "case-1",
  caseNumber: "CS0000001",
  projectId: "proj-1",
  projectName: "Acme",
  workDate: "2026-07-13",
  userId: "user-1",
  userName: "Jane Doe",
  state: "submitted",
  billable: true,
  totalMinutes: 165,
  approvers: [{ id: "lead-1", name: "Lead Approver" }],
  workLogComment: "<p>Investigated the reported latency issue.</p>",
  issueComplexity: "High",
  breakdown: {
    analysisDebugging: 11,
    reproduce: 33,
    settingUp: 22,
    providingSolution: 44,
    answering: 55,
  },
};

describe("LogTimeCardDialog — create mode", () => {
  it("shows the log-time title and submit label, with an editable approver search", () => {
    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        caseSeverity="S3"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Log time · CS0000001")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Submit for review" })).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("Search engineers by name or email…"),
    ).toBeInTheDocument();
  });

  it("shows previously-selected approvers before the engineer types anything", () => {
    mockedUseRecentApprovers.mockReturnValue({
      data: [
        { id: "lead-1", name: "Priya Lead" },
        { id: "lead-2", name: "Sam Approver" },
      ],
    } as unknown as ReturnType<typeof useRecentApprovers>);
    setSearchUsersData({
      eligible: [
        { id: "lead-1", name: "Priya Lead", userName: "priya.lead", email: "priya.lead@example.test" },
        { id: "lead-2", name: "Sam Approver", userName: "sam.approver", email: "sam.approver@example.test" },
      ],
    });

    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        caseSeverity="S3"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Recently selected")).toBeInTheDocument();
    // textContent includes the avatar's initials fallback ahead of the name
    // (e.g. "PLPriya Lead") since the Avatar and name share one button —
    // asserting via the visible name text alone is enough to confirm order.
    expect(screen.getByText("Priya Lead")).toBeInTheDocument();
    expect(screen.getByText("Sam Approver")).toBeInTheDocument();
    const shown = screen.getAllByTestId("approver-candidate").map((el) => el.textContent);
    expect(shown).toEqual(["PLPriya Lead", "SASam Approver"]);
    expect(
      screen.queryByText("Start typing to search for an approver."),
    ).not.toBeInTheDocument();
  });

  it("prioritizes a matching recent approver ahead of live search results when typing", () => {
    mockedUseRecentApprovers.mockReturnValue({
      data: [{ id: "lead-1", name: "Priya Lead" }],
    } as unknown as ReturnType<typeof useRecentApprovers>);
    setSearchUsersData({
      live: [
        { id: "lead-3", name: "Other Lead", userName: "other.lead", email: "other.lead@example.test" },
        { id: "lead-1", name: "Priya Lead", userName: "priya.lead", email: "priya.lead@example.test" },
      ],
      eligible: [{ id: "lead-1", name: "Priya Lead", userName: "priya.lead", email: "priya.lead@example.test" }],
    });

    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        caseSeverity="S3"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText("Search engineers by name or email…"), {
      target: { value: "lead" },
    });

    const shown = screen.getAllByTestId("approver-candidate").map((el) => el.textContent);
    // "Priya Lead" appears once (deduped), and first (prioritized as a
    // recent) — textContent also carries the avatar initials fallback and,
    // for the live-search-only result, its email.
    expect(shown).toEqual(["PLPriya Lead", "OLOther Leadother.lead@example.test"]);
  });

  it("excludes a recent approver who is no longer active/eligible", () => {
    mockedUseRecentApprovers.mockReturnValue({
      data: [
        { id: "lead-1", name: "Priya Lead" },
        { id: "lead-2", name: "Departed Lead" },
      ],
    } as unknown as ReturnType<typeof useRecentApprovers>);
    // Only lead-1 comes back as still active/in the approver role — lead-2
    // has since been deactivated or removed from TIMECARD_APPROVER_GROUP.
    setSearchUsersData({
      eligible: [{ id: "lead-1", name: "Priya Lead", userName: "priya.lead", email: "priya.lead@example.test" }],
    });

    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        caseSeverity="S3"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Priya Lead")).toBeInTheDocument();
    expect(screen.queryByText("Departed Lead")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("approver-candidate")).toHaveLength(1);
  });

  it("falls back to the ordinary empty-search prompt when there is no recent history", () => {
    mockedUseRecentApprovers.mockReturnValueOnce({
      data: [],
    } as unknown as ReturnType<typeof useRecentApprovers>);

    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        caseSeverity="S3"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Start typing to search for an approver."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Recently selected")).not.toBeInTheDocument();
  });
});

describe("LogTimeCardDialog — edit mode", () => {
  it("shows the edit title and Save changes label instead of the create ones", () => {
    render(
      <LogTimeCardDialog
        caseId={EDITING_CARD.caseId}
        caseNumber={EDITING_CARD.caseNumber}
        projectId={EDITING_CARD.projectId}
        projectName={EDITING_CARD.projectName}
        editingCard={EDITING_CARD}
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Edit time card · CS0000001")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Submit for review" })).not.toBeInTheDocument();
  });

  it("prefills the activity breakdown and work-log comment from the card being edited", () => {
    render(
      <LogTimeCardDialog
        caseId={EDITING_CARD.caseId}
        caseNumber={EDITING_CARD.caseNumber}
        projectId={EDITING_CARD.projectId}
        projectName={EDITING_CARD.projectName}
        editingCard={EDITING_CARD}
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByLabelText("Analysis and debugging")).toHaveValue(11);
    expect(screen.getByLabelText("Reproduce")).toHaveValue(33);
    expect(screen.getByLabelText("Setting up")).toHaveValue(22);
    expect(screen.getByLabelText("Providing solution")).toHaveValue(44);
    expect(screen.getByLabelText("Answering")).toHaveValue(55);
    expect(screen.getByLabelText("work-log-editor")).toHaveValue(
      "<p>Investigated the reported latency issue.</p>",
    );
    expect(screen.getByText(`${11 + 33 + 22 + 44 + 55} min total`)).toBeInTheDocument();
  });

  it("shows the approver read-only, with no delete affordance and no search box", () => {
    render(
      <LogTimeCardDialog
        caseId={EDITING_CARD.caseId}
        caseNumber={EDITING_CARD.caseNumber}
        projectId={EDITING_CARD.projectId}
        projectName={EDITING_CARD.projectName}
        editingCard={EDITING_CARD}
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByText("Lead Approver")).toBeInTheDocument();
    expect(
      screen.queryByPlaceholderText("Search engineers by name or email…"),
    ).not.toBeInTheDocument();
  });

  it("submits an UpdateTimeCardInput (cardId set, no approver field) on Save changes", () => {
    const onSubmit = vi.fn();
    render(
      <LogTimeCardDialog
        caseId={EDITING_CARD.caseId}
        caseNumber={EDITING_CARD.caseNumber}
        projectId={EDITING_CARD.projectId}
        projectName={EDITING_CARD.projectName}
        editingCard={EDITING_CARD}
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText("work-log-editor"), {
      target: { value: "<p>Updated comment.</p>" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const input = onSubmit.mock.calls[0][0];
    expect(input).toMatchObject({
      cardId: "card-1",
      date: "2026-07-13",
      billable: true,
      issueComplexity: "High",
      workLogComment: "<p>Updated comment.</p>",
      breakdown: {
        analysisDebugging: 11,
        reproduce: 33,
        settingUp: 22,
        providingSolution: 44,
        answering: 55,
      },
    });
    expect(input).not.toHaveProperty("approver");
    expect(input).not.toHaveProperty("caseId");
  });

  // Regression: approvers is optional on a card. Requiring an approver in edit
  // mode made the form invalid and then disabled Save, blocking the edit
  // outright for a submitted card that has no mapped approver — even though the
  // field is read-only here and never sent.
  it("still saves a card that came back with no approver", () => {
    const onSubmit = vi.fn();
    const { approvers: _omitted, ...cardWithoutApprover } = EDITING_CARD;
    render(
      <LogTimeCardDialog
        caseId={EDITING_CARD.caseId}
        caseNumber={EDITING_CARD.caseNumber}
        projectId={EDITING_CARD.projectId}
        projectName={EDITING_CARD.projectName}
        editingCard={cardWithoutApprover as CsmTimeCard}
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText("work-log-editor"), {
      target: { value: "<p>Updated with no approver on the card.</p>" },
    });

    const save = screen.getByRole("button", { name: "Save changes" });
    expect(save).not.toBeDisabled();

    fireEvent.click(save);

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const input = onSubmit.mock.calls[0][0];
    expect(input).toMatchObject({
      cardId: "card-1",
      workLogComment: "<p>Updated with no approver on the card.</p>",
    });
    expect(input).not.toHaveProperty("approver");
  });
});
