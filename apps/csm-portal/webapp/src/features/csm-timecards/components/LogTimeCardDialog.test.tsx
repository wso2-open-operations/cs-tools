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
    ariaRequired,
  }: {
    value: string;
    onChange: (v: string) => void;
    ariaRequired?: boolean;
  }) => (
    <textarea
      aria-label="work-log-editor"
      aria-required={ariaRequired || undefined}
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

  it("selects an activity minutes field's whole value on focus, so typing replaces the default 0 instead of appending to it", () => {
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

    const input = screen.getByLabelText("Analysis and debugging") as HTMLInputElement;

    // This field must be type="text" (with inputMode="numeric" for the mobile
    // numeric keypad), not type="number": per the HTML spec,
    // HTMLInputElement.select()/selectionStart/selectionEnd are no-ops/throw
    // for type="number" (MDN, the WHATWG spec, and Chromium's own source all
    // confirm this — verified independently), so calling .select() in the
    // onFocus handler would silently do nothing in a real browser if this
    // were still type="number", even though this exact assertion still
    // passes under jsdom for a number input (jsdom doesn't enforce the
    // spec's restriction) — a false negative this test must not reproduce.
    expect(input.type).toBe("text");

    fireEvent.focus(input);

    // Confirms the selection actually happened (every character selected),
    // not merely that `select()` was invoked.
    expect(input.selectionStart).toBe(0);
    expect(input.selectionEnd).toBe(input.value.length);
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

  it("excludes the signed-in user from the recently-selected approvers list", () => {
    mockedUseRecentApprovers.mockReturnValue({
      data: [
        { id: "me-1", name: "Jane Self" },
        { id: "lead-1", name: "Priya Lead" },
      ],
    } as unknown as ReturnType<typeof useRecentApprovers>);
    // The signed-in user (useIdTokenClaims mock: "engineer@example.test")
    // still comes back from the eligibility re-check — a past card could
    // have recorded them as their own approver before this rule existed.
    setSearchUsersData({
      eligible: [
        { id: "me-1", name: "Jane Self", userName: "jane", email: "engineer@example.test" },
        { id: "lead-1", name: "Priya Lead", userName: "priya.lead", email: "priya.lead@example.test" },
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

    expect(screen.getByText("Priya Lead")).toBeInTheDocument();
    expect(screen.queryByText("Jane Self")).not.toBeInTheDocument();
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

describe("LogTimeCardDialog — per-card 8 hour cap", () => {
  const CAP_MESSAGE = "Total logged time cannot exceed 8 hours (480 minutes) per time card.";

  function renderCreate(onSubmit = vi.fn()): ReturnType<typeof vi.fn> {
    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        caseSeverity="S3"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );
    return onSubmit;
  }

  it("shows the cap message on blur and blocks submit when one card totals over 480 minutes", () => {
    const onSubmit = renderCreate();
    const input = screen.getByLabelText("Analysis and debugging");
    fireEvent.change(input, { target: { value: "481" } });
    // Not shown until the field is touched (blurred) or submit is attempted.
    expect(screen.queryByText(CAP_MESSAGE)).not.toBeInTheDocument();
    fireEvent.blur(input);
    expect(screen.getByText(CAP_MESSAGE)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Submit for review" })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("shows the cap message when submit is clicked without blurring first, and does not submit", () => {
    const onSubmit = renderCreate();
    fireEvent.change(screen.getByLabelText("Analysis and debugging"), {
      target: { value: "481" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Submit for review" }));
    expect(screen.getByText(CAP_MESSAGE)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("does not show the cap message at exactly 480 minutes", () => {
    renderCreate();
    const input = screen.getByLabelText("Analysis and debugging");
    fireEvent.change(input, { target: { value: "480" } });
    fireEvent.blur(input);
    expect(screen.queryByText(CAP_MESSAGE)).not.toBeInTheDocument();
  });

  it("applies the same cap in edit mode", () => {
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
    const input = screen.getByLabelText("Analysis and debugging");
    fireEvent.change(input, { target: { value: "481" } });
    fireEvent.blur(input);
    expect(screen.getByText(CAP_MESSAGE)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save changes" })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
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

    // The activity-minutes fields are type="text" (not type="number" — see
    // the onFocus select() comment above), so their DOM value is a string.
    expect(screen.getByLabelText("Analysis and debugging")).toHaveValue("11");
    expect(screen.getByLabelText("Reproduce")).toHaveValue("33");
    expect(screen.getByLabelText("Setting up")).toHaveValue("22");
    expect(screen.getByLabelText("Providing solution")).toHaveValue("44");
    expect(screen.getByLabelText("Answering")).toHaveValue("55");
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

describe("LogTimeCardDialog — billable switch", () => {
  function renderDialog(
    props: Partial<Parameters<typeof LogTimeCardDialog>[0]> = {},
  ): void {
    render(
      <LogTimeCardDialog
        caseId="case-1"
        caseNumber="CS0000001"
        projectId="proj-1"
        projectName="Acme"
        isSubmitting={false}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        {...props}
      />,
    );
  }

  it("defaults to billable and stays editable for an S4 support case", () => {
    renderDialog({ caseSeverity: "S4", caseType: "case" });

    const toggle = screen.getByLabelText("Billable");
    expect(toggle).toBeChecked();
    expect(toggle).not.toBeDisabled();
  });

  it("defaults a paid engagement to non-billable but leaves the switch editable", () => {
    renderDialog({ caseType: "engagement", engagementPaymentType: "Paid" });

    const toggle = screen.getByLabelText("Non-billable");
    expect(toggle).not.toBeChecked();
    expect(toggle).not.toBeDisabled();

    fireEvent.click(toggle);
    expect(screen.getByLabelText("Billable")).toBeChecked();
  });

  it("treats an engagement with no payment type like a paid one", () => {
    renderDialog({ caseType: "engagement" });

    const toggle = screen.getByLabelText("Non-billable");
    expect(toggle).not.toBeChecked();
    expect(toggle).not.toBeDisabled();
  });

  it.each(["FOC", "foc", "Free of Charge", "FOC (Free of Charge)"])(
    "locks an FOC engagement (%s) to non-billable",
    (paymentType) => {
      renderDialog({ caseType: "engagement", engagementPaymentType: paymentType });

      const toggle = screen.getByLabelText("Non-billable");
      expect(toggle).not.toBeChecked();
      expect(toggle).toBeDisabled();
      expect(
        screen.getByText("Always non-billable for FOC engagements."),
      ).toBeInTheDocument();
    },
  );

  it("locks an FOC engagement's card to non-billable in edit mode, even if saved as billable", () => {
    renderDialog({
      caseType: "engagement",
      engagementPaymentType: "FOC",
      editingCard: { ...EDITING_CARD, billable: true },
    });

    const toggle = screen.getByLabelText("Non-billable");
    expect(toggle).not.toBeChecked();
    expect(toggle).toBeDisabled();
  });

  it("keeps a paid engagement card's saved billable value in edit mode", () => {
    renderDialog({
      caseType: "engagement",
      engagementPaymentType: "Paid",
      editingCard: { ...EDITING_CARD, billable: true },
    });

    const toggle = screen.getByLabelText("Billable");
    expect(toggle).toBeChecked();
    expect(toggle).not.toBeDisabled();
  });
});

describe("LogTimeCardDialog — required marks", () => {
  /** The asterisk RequiredMark draws inside a caption the dialog renders itself. */
  const captionHasMark = (text: string): boolean =>
    screen.getByText(text, { selector: "h6, span" }).querySelector(".MuiFormLabel-asterisk") !== null;

  it("marks the time breakdown, work log comment and approver as required on create", () => {
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

    expect(captionHasMark("Time breakdown (minutes)")).toBe(true);
    expect(captionHasMark("Work log comment")).toBe(true);
    expect(screen.getByLabelText("work-log-editor")).toHaveAttribute("aria-required", "true");
    expect(captionHasMark("Approver (team lead)")).toBe(true);
    // The search box is named by that caption (the asterisk is aria-hidden), not
    // just its placeholder.
    const approverSearch = screen.getByRole("textbox", { name: "Approver (team lead)" });
    expect(approverSearch).toBe(screen.getByPlaceholderText("Search engineers by name or email…"));
    expect(approverSearch).toHaveAttribute("aria-required", "true");
    // One asterisk each: the work log caption no longer carries a typed "*".
    expect(screen.queryByText(/Work log comment \*/)).not.toBeInTheDocument();
  });

  it("drops the approver asterisk in edit mode, where the approver is read-only and never sent", () => {
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

    expect(captionHasMark("Approver (team lead)")).toBe(false);
    expect(captionHasMark("Time breakdown (minutes)")).toBe(true);
    expect(captionHasMark("Work log comment")).toBe(true);
  });
});
