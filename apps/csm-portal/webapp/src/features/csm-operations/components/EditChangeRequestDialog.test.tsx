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

import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { clearUserPreferredTimeZone, setUserPreferredTimeZone } from "@utils/dateTime";
import type { BeChangeRequestDetail, BePatchChangeRequestPayload } from "@api/backend/types";


// The "Assignment group" picker goes through useSearchGroups, which hits the
// backend client via react-query — stub it out (same approach as
// EditIncidentDialog.test.tsx).
const useSearchGroupsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (...args: unknown[]) => useSearchGroupsMock(...(args as [])),
}));

// The "Requested by" picker (added for CR field parity) goes through the
// same kind of backend-client-backed hook — stub it out identically.
const useSearchUsersByNameMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
vi.mock("@api/useSearchUsersByName", () => ({
  useSearchInternalUsersByName: (...args: unknown[]) => useSearchUsersByNameMock(...(args as [])),
}));

// The "Originating service request" picker searches service requests through
// the backend client — stub it with one result.
vi.mock("@features/csm-operations/api/useSearchServiceRequestsForSelect", () => ({
  useSearchServiceRequestsForSelect: () => ({
    data: [{ id: "case-2", number: "CS0002", subject: "Rotate the certificates" }],
    isFetching: false,
    isError: false,
  }),
}));

// Customer Project picker: a plain labelled input that reports the typed id and
// its display name; exposes whether it may be cleared.
const PROJECT_NAMES: Record<string, string> = { "proj-a": "Acme Project", "proj-b": "Beta Project" };
vi.mock("@features/csm-cases/components/AsyncProjectSelect", () => ({
  default: ({
    label,
    value,
    knownLabel,
    disableClearable,
    disabled,
    accountId,
    onChange,
  }: {
    label: string;
    value: string;
    knownLabel?: string;
    disableClearable?: boolean;
    disabled?: boolean;
    accountId?: string;
    onChange: (next: string, name?: string) => void;
  }) => (
    <input
      aria-label={label}
      data-known-label={knownLabel ?? ""}
      data-disable-clearable={String(!!disableClearable)}
      // The customer the real picker narrows its search to (server-side).
      data-account-id={accountId ?? ""}
      disabled={disabled}
      value={value}
      onChange={(e) => onChange(e.target.value, PROJECT_NAMES[e.target.value])}
    />
  ),
}));
// project -> deployments / deployment products / customer contacts lookup, same
// contract as the real hook: a project's deployments always come back, products
// only for the chosen ones, and the contacts (the read-only Customer Group) are
// the project's own.
const SCOPE_FIXTURE: Record<
  string,
  Array<{ id: string; label: string; products: Array<{ id: string; label: string }> }>
> = {
  "proj-a": [
    {
      id: "dep-prod",
      label: "Acme Production",
      products: [
        { id: "dp-apim", label: "API Manager 4.3.0" },
        { id: "dp-is", label: "Identity Server 7.0.0" },
      ],
    },
    {
      id: "dep-stg",
      label: "Acme Staging",
      products: [{ id: "dp-apim-stg", label: "API Manager 4.2.0" }],
    },
  ],
  "proj-b": [
    {
      id: "dep-b",
      label: "Beta Development",
      products: [{ id: "dp-b", label: "Choreo 1.0" }],
    },
  ],
};
const CONTACTS_FIXTURE: Record<string, Array<{ id: string; name: string }>> = {
  "proj-a": [
    { id: "pc-1", name: "Alice Aaron" },
    { id: "pc-2", name: "Bob Bell" },
  ],
  "proj-b": [{ id: "pc-9", name: "Carol Cook" }],
};
vi.mock("@features/csm-operations/api/useChangeRequestScopeLookups", () => ({
  useChangeRequestScopeLookups: (projectId: string | undefined, deploymentIds: string[]) => ({
    deployments: (projectId ? (SCOPE_FIXTURE[projectId] ?? []) : []).map((d) => ({
      id: d.id,
      label: d.label,
      products: deploymentIds.includes(d.id) ? d.products : undefined,
    })),
    customerContacts: projectId ? (CONTACTS_FIXTURE[projectId] ?? []) : [],
    contactsReady: !!projectId,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));

/**
 * Stand-in for the rich-text editor: a textarea whose value is the HTML.
 *
 * It deliberately reproduces the two behaviours the dialog's dirty-tracking
 * has to cope with, because a simpler stub would let the "untouched field
 * stays out of the patch" tests pass for the wrong reason:
 *
 *  - it rewrites the markup it loads (the real editor wraps text in
 *    `<span style="white-space: pre-wrap;">`), so the HTML coming out never
 *    equals the stored HTML going in, and
 *  - it emits that rewritten value once on mount — but only when there was
 *    something to load, exactly as the real one does.
 */
const normalizeLikeEditor = (html: string): string =>
  html.replace(
    /<p>([\s\S]*?)<\/p>/g,
    '<p><span style="white-space: pre-wrap;">$1</span></p>',
  );

vi.mock("@components/rich-text-editor/Editor", async () => {
  const { useEffect, useRef, useState } = await import("react");
  function EditorStub({
    value,
    onChange,
    disabled,
  }: {
    value?: string;
    onChange?: (html: string) => void;
    disabled?: boolean;
  }) {
    const [html, setHtml] = useState(value ? normalizeLikeEditor(value) : "");
    const seededRef = useRef(false);
    useEffect(() => {
      if (seededRef.current || !value?.trim()) return;
      seededRef.current = true;
      onChange?.(normalizeLikeEditor(value));
    }, [value, onChange]);
    return (
      <textarea
        value={html}
        disabled={disabled}
        onChange={(e) => {
          setHtml(e.target.value);
          onChange?.(e.target.value);
        }}
      />
    );
  }
  return { default: EditorStub };
});

import EditChangeRequestDialog from "@features/csm-operations/components/EditChangeRequestDialog";

const BASE_CR: BeChangeRequestDetail = {
  id: "chg-1",
  number: "CHG0009988",
  subject: "Upgrade the gateway cluster",
  createdOn: "2026-01-01T00:00:00Z",
  state: "assess",
  type: "normal",
  assignedTeam: { id: "team-1", name: "Platform" },
  hasCustomerApproved: false,
  hasCustomerReviewed: false,
};

/**
 * Render the dialog over `BASE_CR` with the given field overrides, returning the
 * `onSave`/`onClose` spies so a test can assert exactly which fields were
 * submitted — these tests are mostly about the dialog sending *only* changed
 * fields, so the payload passed to `onSave` is the assertion target.
 */
function renderDialog(
  overrides: Partial<BeChangeRequestDetail> = {},
  onSave = vi.fn<(patch: BePatchChangeRequestPayload) => void>(),
): { onSave: typeof onSave; onClose: ReturnType<typeof vi.fn> } {
  const onClose = vi.fn();
  render(
    <EditChangeRequestDialog
      cr={{ ...BASE_CR, ...overrides }}
      isSaving={false}
      onClose={onClose}
      onSave={onSave}
    />,
  );
  return { onSave, onClose };
}

/** The dialog's Save button ("Add work note" once the change request has been approved). */
const saveButton = (): HTMLElement => screen.getByRole("button", { name: /^(save|add work note)$/i });

// "Customer approved"/"Customer reviewed" are deliberately not rendered as
// switches in this dialog — see EditChangeRequestDialog.tsx's doc comment for
// why (they drive a gated SN state transition, not a boolean field, and the
// "off" direction is destructive). This test file only covers what the
// dialog actually renders.

describe("EditChangeRequestDialog — Customer approved / reviewed are not editable here", () => {
  it("renders no 'Customer approved' or 'Customer reviewed' control", () => {
    renderDialog();
    expect(screen.queryByLabelText(/customer approved/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/customer reviewed/i)).not.toBeInTheDocument();
  });
});

describe("EditChangeRequestDialog — save error surfacing", () => {
  it("renders no error alert by default", () => {
    renderDialog();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the given saveError as a visible alert inside the dialog", () => {
    const onClose = vi.fn();
    render(
      <EditChangeRequestDialog
        cr={BASE_CR}
        isSaving={false}
        saveError="Could not update the change request."
        onClose={onClose}
        onSave={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("alert"),
    ).toHaveTextContent(/could not update/i);
  });
});

// ---------------------------------------------------------------------------
// Fields added so the plan and schedule can actually be entered somewhere:
// rollback plan, test plan, and planned end.
// ---------------------------------------------------------------------------

/** The "Rollback plan" textarea. */
// The editor takes no native label, so each plan is reached through the
// labelled group wrapping it — the same association assistive tech uses.
const planEditor = (name: RegExp): HTMLElement =>
  within(screen.getByRole("group", { name })).getByRole("textbox");

const rollbackPlanField = (): HTMLElement => planEditor(/rollback plan/i);
/** The "Test plan" textarea. */
const testPlanField = (): HTMLElement => planEditor(/test plan/i);

describe("EditChangeRequestDialog — rollback and test plans", () => {
  it("seeds each plan field from the stored rich text, markup and all", () => {
    renderDialog({
      rollbackPlan: "<p>Restore the previous release.</p>",
      testPlan: "<p>Smoke the gateway health endpoint.</p>",
    });
    // Whatever the editor makes of the stored HTML, the stored content is
    // what it was handed — no plain-text round trip in between.
    expect(rollbackPlanField()).toHaveValue(
      normalizeLikeEditor("<p>Restore the previous release.</p>"),
    );
    expect(testPlanField()).toHaveValue(
      normalizeLikeEditor("<p>Smoke the gateway health endpoint.</p>"),
    );
  });

  it("renders both fields empty when the change request has no plans yet", () => {
    renderDialog();
    expect(rollbackPlanField()).toHaveValue("");
    expect(testPlanField()).toHaveValue("");
  });

  it("sends only the rollback plan when only that field was edited", () => {
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), {
      target: { value: "<p>Redeploy the previous image tag.</p>" },
    });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      rollbackPlan: "<p>Redeploy the previous image tag.</p>",
    });
  });

  it("sends only the test plan when only that field was edited", () => {
    const { onSave } = renderDialog();
    fireEvent.change(testPlanField(), {
      target: { value: "<p>Run the regression suite.</p>" },
    });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      testPlan: "<p>Run the regression suite.</p>",
    });
  });

  it("sends both plans, and nothing else, when both were edited", () => {
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), {
      target: { value: "<p>Roll the image back.</p>" },
    });
    fireEvent.change(testPlanField(), {
      target: { value: "<p>Run the regression suite.</p>" },
    });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      rollbackPlan: "<p>Roll the image back.</p>",
      testPlan: "<p>Run the regression suite.</p>",
    });
  });

  it("leaves Save disabled on open when a plan is already stored", () => {
    renderDialog({ rollbackPlan: "<p>Restore the previous release.</p>" });
    expect(saveButton()).toBeDisabled();
  });

  // An intentional clear must stay an empty string, not become the editor's
  // empty paragraph: `<p><br></p>` would read as "this plan says nothing"
  // rather than "there is no plan".
  it("treats clearing a stored plan as a real edit and sends the empty value", () => {
    const { onSave } = renderDialog({ rollbackPlan: "<p>Restore the previous release.</p>" });
    fireEvent.change(rollbackPlanField(), { target: { value: "" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ rollbackPlan: "" });
  });

  it("sends \"\" rather than an empty paragraph when the editor is emptied", () => {
    const { onSave } = renderDialog({ rollbackPlan: "<p>Restore the previous release.</p>" });
    fireEvent.change(rollbackPlanField(), { target: { value: "<p><br></p>" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ rollbackPlan: "" });
  });

  it("does not treat emptying an already-empty plan as an edit", () => {
    renderDialog();
    fireEvent.change(rollbackPlanField(), { target: { value: "<p><br></p>" } });
    expect(saveButton()).toBeDisabled();
  });

  // The point of editing these as rich text: the markup the engineer applies
  // is what gets stored, with no lossy conversion on either side.
  it("keeps the markup of an edited plan intact, through the sanitizer", () => {
    const authored =
      "<p><strong>Stop</strong> the rollout.</p><ul><li>Redeploy the previous tag.</li></ul>";
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), { target: { value: authored } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ rollbackPlan: authored });
  });

  it("keeps escaped entities escaped rather than letting them collapse into markup", () => {
    const authored = "<p>Stop if error rate &lt; 1% &amp; rising.</p>";
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), { target: { value: authored } });
    fireEvent.click(saveButton());

    const patch = onSave.mock.calls[0][0];
    const rendered = document.createElement("div");
    rendered.innerHTML = patch.rollbackPlan ?? "";
    expect(rendered.textContent).toBe("Stop if error rate < 1% & rising.");
  });

  it("strips anything the sanitizer would reject before it is stored", () => {
    const { onSave } = renderDialog();
    fireEvent.change(rollbackPlanField(), {
      target: { value: "<p>Roll back.</p><script>steal()</script>" },
    });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ rollbackPlan: "<p>Roll back.</p>" });
  });
});

describe("EditChangeRequestDialog — assigned engineer", () => {
  it("renders an Assigned to picker alongside Assignment group", () => {
    renderDialog();
    expect(screen.getByLabelText(/^assigned to$/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/^assignment group$/i)).toBeInTheDocument();
  });

  it("seeds the known assignee's name when the CR already has one", () => {
    renderDialog({ assignedEngineer: { id: "user-1", name: "Jane Doe" } });
    expect(screen.getByLabelText(/^assigned to$/i)).toHaveValue("Jane Doe");
  });

  it("leaves Save disabled when nothing changed, even with an existing assignee", () => {
    renderDialog({ assignedEngineer: { id: "user-1", name: "Jane Doe" } });
    expect(saveButton()).toBeDisabled();
  });
});

describe("EditChangeRequestDialog — originating service request", () => {
  const picker = (): HTMLElement => screen.getByLabelText(/^originating service request$/i);
  const pick = (): void => {
    fireEvent.change(picker(), { target: { value: "CS0002" } });
    fireEvent.click(screen.getByRole("option", { name: "CS0002 — Rotate the certificates" }));
  };

  it("links a change request raised without one", () => {
    const { onSave } = renderDialog();
    expect(picker()).toHaveValue("");
    pick();
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ caseId: "case-2" });
  });

  it("shows the current link and replaces it", () => {
    const { onSave } = renderDialog({ case: { id: "case-1", name: "CS0001" } });
    expect(picker()).toHaveValue("CS0001");
    expect(saveButton()).toBeDisabled();
    pick();
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ caseId: "case-2" });
  });
});

describe("EditChangeRequestDialog — planned end must be after planned start", () => {
  it("renders a Planned end picker alongside Planned start", () => {
    renderDialog();
    // The MUI date-time picker renders a segmented group (day/month/year/…),
    // so each picker matches `getByLabelText` several times over, and the
    // outlined field renders its label twice (visible label plus the fieldset
    // legend) — hence `getAllByText`.
    expect(screen.getAllByText("Planned start").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Planned end").length).toBeGreaterThan(0);
  });

  // The patch payload cannot express "remove the planned date" (an omitted key
  // means "leave it alone"), so a clear affordance would look like it worked
  // and then save nothing. Removed from both pickers until the payload can
  // express it.
  it("offers no clear affordance on either picker", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 12:00:00",
    });
    expect(screen.queryByRole("button", { name: /clear/i })).not.toBeInTheDocument();
  });

  it("flags an end that is before the start, and blocks the save", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 09:00:00",
    });
    // Make the form dirty so Save would otherwise be enabled — this proves the
    // date check, not the dirty check, is what disables it.
    fireEvent.change(rollbackPlanField(), { target: { value: "<p>dirty</p>" } });
    expect(
      screen.getByText(/planned end must be after planned start/i),
    ).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it("flags an end equal to the start — a zero-length change window is not a window", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 10:00:00",
    });
    fireEvent.change(rollbackPlanField(), { target: { value: "<p>dirty</p>" } });
    expect(saveButton()).toBeDisabled();
  });

  it("accepts an end after the start", () => {
    renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 12:00:00",
    });
    fireEvent.change(rollbackPlanField(), { target: { value: "<p>dirty</p>" } });
    expect(
      screen.queryByText(/planned end must be after planned start/i),
    ).not.toBeInTheDocument();
    expect(saveButton()).toBeEnabled();
  });

  it("does not flag anything when only one end of the window is set", () => {
    renderDialog({ plannedStartOn: "2026-03-01 10:00:00" });
    fireEvent.change(rollbackPlanField(), { target: { value: "<p>dirty</p>" } });
    expect(
      screen.queryByText(/planned end must be after planned start/i),
    ).not.toBeInTheDocument();
    expect(saveButton()).toBeEnabled();
  });
});

describe("EditChangeRequestDialog — planned dates round-trip through the user's time zone", () => {
  afterEach(() => {
    clearUserPreferredTimeZone();
  });

  it("does not re-send an untouched planned date: the UTC value read in converts back to itself", () => {
    // Asia/Colombo is UTC+05:30. Without the time zone conversion, the stored
    // UTC digits were treated as wall-clock and a save would have shifted them.
    setUserPreferredTimeZone("Asia/Colombo");
    const { onSave } = renderDialog({
      plannedStartOn: "2026-03-01 10:00:00",
      plannedEndOn: "2026-03-01 12:00:00",
    });
    fireEvent.change(planEditor(/rollback plan/i), { target: { value: "<p>dirty</p>" } });
    fireEvent.click(saveButton());

    expect(onSave).toHaveBeenCalledTimes(1);
    const patch = onSave.mock.calls[0][0];
    expect(patch).not.toHaveProperty("plannedStartOn");
    expect(patch).not.toHaveProperty("plannedEndOn");
  });

  it("shows the stored UTC start in the user's time zone", () => {
    setUserPreferredTimeZone("Asia/Colombo");
    renderDialog({ plannedStartOn: "2026-03-01 10:00:00", plannedEndOn: "2026-03-01 12:00:00" });
    // 10:00 UTC is 15:30 (03:30 PM) in Colombo.
    const start = screen.getByRole("group", { name: /planned start/i });
    expect(start).toHaveTextContent("03/01/2026 03:30 PM");
  });
});

describe("EditChangeRequestDialog — the 'in the past' hint follows the profile time zone", () => {
  const PAST_HINT = /this date is in the past/i;

  beforeAll(() => {
    // Pin the browser zone so it differs from the profile zone under test.
    vi.stubEnv("TZ", "UTC");
  });

  afterAll(() => {
    vi.unstubAllEnvs();
  });

  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2030-03-01T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
    clearUserPreferredTimeZone();
  });

  it("warns when the start is past in the profile zone but would look future in the browser zone", () => {
    setUserPreferredTimeZone("Asia/Colombo");
    // 10:00Z is before now (12:00Z); the picker shows 15:30, which read as
    // browser (UTC) digits would be 15:30Z, i.e. later than now.
    renderDialog({ plannedStartOn: "2030-03-01 10:00:00", plannedEndOn: "2030-03-01 20:00:00" });
    expect(screen.getByText(PAST_HINT)).toBeInTheDocument();
  });

  it("does not warn when the start is future in the profile zone but would look past in the browser zone", () => {
    setUserPreferredTimeZone("America/Los_Angeles");
    // 16:00Z is after now; the picker shows 08:00, which read as browser (UTC)
    // digits would be 08:00Z, i.e. earlier than now.
    renderDialog({ plannedStartOn: "2030-03-01 16:00:00", plannedEndOn: "2030-03-01 20:00:00" });
    expect(screen.queryByText(PAST_HINT)).not.toBeInTheDocument();
  });
});

describe("EditChangeRequestDialog — Customer Approval / Customer Review checkboxes", () => {
  const approvalBox = (): HTMLElement => screen.getByRole("checkbox", { name: "Customer Approval" });
  const reviewBox = (): HTMLElement => screen.getByRole("checkbox", { name: "Customer Review" });
  const ACME = { id: "proj-a", name: "Acme Project" };

  describe("in New (the creation phase) everything is editable", () => {
    it("renders both as checkboxes reflecting the stored flags, enabled, even with no project", () => {
      renderDialog({ state: "new", customerApprovalRequired: true, customerReviewRequired: false });
      expect(approvalBox()).toBeChecked();
      expect(approvalBox()).toBeEnabled();
      expect(reviewBox()).not.toBeChecked();
      expect(reviewBox()).toBeEnabled();
      expect((approvalBox() as HTMLInputElement).type).toBe("checkbox");
    });

    it("treats absent flags as unchecked and leaves Save disabled with no change", () => {
      renderDialog({ state: "new" });
      expect(approvalBox()).not.toBeChecked();
      expect(reviewBox()).not.toBeChecked();
      expect(saveButton()).toBeDisabled();
    });

    it("sends only customerApprovalRequired when only Customer Approval was toggled", () => {
      const { onSave } = renderDialog({ state: "new", customerApprovalRequired: false });
      fireEvent.click(approvalBox());
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ customerApprovalRequired: true });
    });

    it("sends only customerReviewRequired when only Customer Review was toggled", () => {
      const { onSave } = renderDialog({ state: "new", customerReviewRequired: false });
      fireEvent.click(reviewBox());
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ customerReviewRequired: true });
    });

    it("sends both, and can switch a ticked box OFF (nothing has been requested yet)", () => {
      const { onSave } = renderDialog({
        state: "new",
        customerApprovalRequired: true,
        customerReviewRequired: false,
      });
      fireEvent.click(approvalBox());
      fireEvent.click(reviewBox());
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({
        customerApprovalRequired: false,
        customerReviewRequired: true,
      });
    });

    it("does not send a flag that was toggled back to its original value", () => {
      renderDialog({ state: "new", customerApprovalRequired: false });
      fireEvent.click(approvalBox());
      fireEvent.click(approvalBox());
      expect(saveButton()).toBeDisabled();
    });

    it("says nothing about the box being final: it is not, yet", () => {
      renderDialog({ state: "new" });
      expect(screen.queryByText(/can't be removed/i)).not.toBeInTheDocument();
    });
  });

  describe("an Emergency change acts without customer consent: both boxes are disabled and unticked, with one line saying so", () => {
    const EMERGENCY_LINE = "Emergency changes proceed without customer approval or review.";

    it.each(["new", "assess", "authorize", "scheduled", "implement", "review", "closed", "canceled"])(
      "in %s, with a project: both boxes are disabled and unticked, with nothing to save",
      (state) => {
        const { onSave } = renderDialog({ type: "emergency", state, project: ACME });
        for (const box of [approvalBox(), reviewBox()]) {
          expect(box).toBeDisabled();
          expect(box).not.toBeChecked();
          expect(box).toHaveAccessibleDescription(new RegExp(EMERGENCY_LINE.replace(/\./g, "\\.")));
        }
        expect(screen.getAllByText(EMERGENCY_LINE)).toHaveLength(1);
        // Nothing differs from the record, so there is nothing to save.
        expect(saveButton()).toBeDisabled();
        expect(onSave).not.toHaveBeenCalled();
      },
    );

    it("never offers the add-only note, the gate lock or the missing-project lock on the boxes it turns off", () => {
      renderDialog({ type: "emergency", state: "assess" });
      expect(screen.queryByText(/can't be removed/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Needs a Customer Project/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/Locked: the change request has already reached/i)).not.toBeInTheDocument();
    });

    it("an Emergency change in New that still has a box ticked from before the rule opens with it off, and saving clears it", () => {
      const { onSave } = renderDialog({
        type: "emergency",
        state: "new",
        customerApprovalRequired: true,
        customerReviewRequired: true,
      });
      expect(approvalBox()).not.toBeChecked();
      expect(reviewBox()).not.toBeChecked();
      expect(approvalBox()).toBeDisabled();
      // The form differs from the record, so there is something to save.
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ customerApprovalRequired: false, customerReviewRequired: false });
    });

    it("an Emergency change past New that still has a box ticked shows it as stored, disabled, with the add-only reason (nothing can take it back)", () => {
      const { onSave } = renderDialog({
        type: "emergency",
        state: "customer_approval",
        project: ACME,
        customerApprovalRequired: true,
      });
      expect(approvalBox()).toBeChecked();
      expect(approvalBox()).toBeDisabled();
      expect(approvalBox()).toHaveAccessibleDescription(/added but never removed/);
      expect(reviewBox()).not.toBeChecked();
      expect(saveButton()).toBeDisabled();
      expect(onSave).not.toHaveBeenCalled();
    });

    it("caps the rollback duration at the 255 characters the backend stores", () => {
      const { onSave } = renderDialog({ type: "normal", state: "new" });
      fireEvent.change(screen.getByLabelText(/rollback duration/i), { target: { value: "x".repeat(300) } });
      fireEvent.click(saveButton());
      expect(onSave.mock.calls[0]![0].rollbackDurationText).toBe("x".repeat(255));
    });

    it("leaves the other fields editable: a planned-window change still saves, with no customer flag in the patch", () => {
      const { onSave } = renderDialog({ type: "emergency", state: "new" });
      fireEvent.change(screen.getByLabelText(/rollback duration/i), { target: { value: "15 mins" } });
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledTimes(1);
      const patch = onSave.mock.calls[0]![0];
      expect(patch).not.toHaveProperty("customerApprovalRequired");
      expect(patch).not.toHaveProperty("customerReviewRequired");
    });

    it.each(["normal", "standard"])("says nothing about Emergency on a %s change, whose boxes stay live", (type) => {
      renderDialog({ type, state: "new" });
      expect(screen.queryByText(EMERGENCY_LINE)).not.toBeInTheDocument();
      expect(approvalBox()).toBeEnabled();
      expect(reviewBox()).toBeEnabled();
    });
  });

  describe("after New a ticked box is read-only: a customer requirement can be added but never removed", () => {
    it.each(["assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"])(
      "Customer Approval, ticked, in %s",
      (state) => {
        const { onSave } = renderDialog({ state, project: ACME, customerApprovalRequired: true });
        expect(approvalBox()).toBeChecked();
        expect(approvalBox()).toBeDisabled();
        expect(approvalBox()).toHaveAccessibleDescription(
          "Once approval has been requested a customer requirement can be added but never removed.",
        );
        // Nothing to save: a ticked box cannot be taken back, so no patch can carry it.
        expect(saveButton()).toBeDisabled();
        expect(onSave).not.toHaveBeenCalled();
      },
    );

    it.each(["assess", "authorize", "customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"])(
      "Customer Review, ticked, in %s",
      (state) => {
        renderDialog({ state, project: ACME, customerReviewRequired: true });
        expect(reviewBox()).toBeChecked();
        expect(reviewBox()).toBeDisabled();
        expect(reviewBox()).toHaveAccessibleDescription(/added but never removed/);
      },
    );

    it("is read-only even when the change request has no project (a stored flag is never taken back)", () => {
      renderDialog({ state: "assess", customerApprovalRequired: true });
      expect(approvalBox()).toBeDisabled();
    });
  });

  describe("after New an unticked box can still be ticked until its gate, with a note that it can't be removed", () => {
    it.each(["assess", "authorize"])("Customer Approval stays tickable in %s, and sends only that", (state) => {
      const { onSave } = renderDialog({ state, project: ACME, customerApprovalRequired: false });
      expect(approvalBox()).toBeEnabled();
      expect(approvalBox()).toHaveAccessibleDescription(/Once saved this can't be removed\./);
      fireEvent.click(approvalBox());
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ customerApprovalRequired: true });
    });

    it.each(["assess", "authorize"])(
      "Customer Review stays tickable in %s, and sends only that",
      (state) => {
        const { onSave } = renderDialog({ state, project: ACME, customerReviewRequired: false });
        expect(reviewBox()).toBeEnabled();
        expect(reviewBox()).toHaveAccessibleDescription(/Once saved this can't be removed\./);
        fireEvent.click(reviewBox());
        fireEvent.click(saveButton());
        expect(onSave).toHaveBeenCalledWith({ customerReviewRequired: true });
      },
    );

    it("a box that was ticked here and not saved can be unticked again (it is not stored yet)", () => {
      renderDialog({ state: "assess", project: ACME, customerApprovalRequired: false });
      fireEvent.click(approvalBox());
      expect(approvalBox()).toBeChecked();
      fireEvent.click(approvalBox());
      expect(approvalBox()).not.toBeChecked();
      expect(saveButton()).toBeDisabled();
    });
  });

  describe("past its gate an unticked box is disabled with the existing reason", () => {
    it.each(["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"])(
      "Customer Approval in %s",
      (state) => {
        renderDialog({ state, project: ACME, customerApprovalRequired: false });
        expect(approvalBox()).not.toBeChecked();
        expect(approvalBox()).toBeDisabled();
        expect(approvalBox()).toHaveAccessibleDescription(
          "Locked: the change request has already reached the customer approval step or later.",
        );
      },
    );

    it.each(["customer_review", "closed", "rollback", "canceled"])("Customer Review in %s", (state) => {
      renderDialog({ state, project: ACME, customerReviewRequired: false });
      expect(reviewBox()).not.toBeChecked();
      expect(reviewBox()).toBeDisabled();
      expect(reviewBox()).toHaveAccessibleDescription(
        "Locked: the change request has already reached the customer review step or later.",
      );
    });

    // Once approved every field is read-only, the review box included (it used to stay
    // tickable until the customer review step).
    it.each(["customer_approval", "scheduled", "implement", "review"])(
      "Customer Review can no longer be added once the change request has been approved (%s)",
      (state) => {
        renderDialog({ state, project: ACME, customerApprovalRequired: true, customerReviewRequired: false });
        expect(approvalBox()).toBeDisabled();
        expect(reviewBox()).toBeDisabled();
        expect(screen.getByText(/This change request has been approved/)).toBeInTheDocument();
        // Nothing can be saved: the box does not promise an edit the form no longer offers.
        expect(reviewBox()).not.toHaveAccessibleDescription(/Once saved this can't be removed/);
      },
    );
  });

  describe("after New an unticked box cannot be added without a Customer Project, and none can be set any more", () => {
    it.each(["assess", "authorize"])("Customer Approval in %s", (state) => {
      renderDialog({ state, customerApprovalRequired: false });
      expect(approvalBox()).toBeDisabled();
      expect(approvalBox()).toHaveAccessibleDescription("Needs a Customer Project, which can no longer be set. Cancel and clone.");
    });

    it("Customer Review in assess", () => {
      renderDialog({ state: "assess", customerReviewRequired: false });
      expect(reviewBox()).toBeDisabled();
      expect(reviewBox()).toHaveAccessibleDescription(/Needs a Customer Project/);
    });

    it("and in New the same box is tickable (the project can still be chosen there)", () => {
      renderDialog({ state: "new", customerApprovalRequired: false });
      expect(approvalBox()).toBeEnabled();
    });
  });

  describe("ticking a box on when nobody on the project can be asked: the backend refuses, and its words show (the dialog does not guess who can be asked)", () => {
    // The backend names the box turned on (entity-service `nobodyToAskMsg`).
    const nobodyCanBeAsked = (what: string): string =>
      `${what} required but nobody on this project can be asked (no registered contact other than the requester): register a contact for the project first`;

    it.each([
      ["authorize", "Customer Approval", { customerApprovalRequired: true }, nobodyCanBeAsked("customer approval is")],
      ["assess", "Customer Review", { customerReviewRequired: true }, nobodyCanBeAsked("customer review is")],
    ] as const)("in %s the unticked %s box stays tickable and is sent; the refusal then shows verbatim in the dialog", (state, label, sent, refusal) => {
      const onSave = vi.fn<(patch: BePatchChangeRequestPayload) => void>();
      const cr: BeChangeRequestDetail = { ...BASE_CR, state, project: ACME };
      const { rerender } = render(<EditChangeRequestDialog cr={cr} isSaving={false} onClose={vi.fn()} onSave={onSave} />);
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("checkbox", { name: label }));
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith(sent);
      // The page hands the backend's 400 back as `saveError`.
      rerender(<EditChangeRequestDialog cr={cr} isSaving={false} saveError={refusal} onClose={vi.fn()} onSave={onSave} />);
      expect(screen.getByRole("alert")).toHaveTextContent(refusal);
      // The box is still there to untick (nothing was saved), so the dialog is not stuck.
      expect(screen.getByRole("checkbox", { name: label })).toBeChecked();
    });
  });

  it("shows the backend's refusal message when it still answers one (400), whatever the rule said", () => {
    render(
      <EditChangeRequestDialog
        cr={{ ...BASE_CR, state: "assess" }}
        isSaving={false}
        saveError="customerApprovalRequired can no longer be turned off: once approval has been requested a customer requirement can be added but never removed (current state: assess). Cancel and clone to correct it."
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/can no longer be turned off/i);
  });
});

// ---------------------------------------------------------------------------
// Customer Project / Deployments / Deployment products / Customer Group / Category
// ---------------------------------------------------------------------------

// Authorize: after New (the project is frozen) and before the approval (the
// deployments are still editable).
const SCOPED_CR: Partial<BeChangeRequestDetail> = {
  state: "authorize",
  project: { id: "proj-a", name: "Acme Project" },
  deployments: [{ id: "dep-prod", name: "Acme Production" }],
  deploymentProducts: [
    { id: "dp-apim", name: "API Manager 4.3.0" },
    { id: "dp-is", name: "Identity Server 7.0.0" },
  ],
  customerContacts: [
    { id: "pc-1", name: "Alice Aaron" },
    { id: "pc-2", name: "Bob Bell" },
  ],
};

function pickOptions(field: string, names: string[]): void {
  const input = screen.getByRole("combobox", { name: field });
  fireEvent.mouseDown(input);
  for (const name of names) fireEvent.click(screen.getByRole("option", { name }));
  fireEvent.keyDown(input, { key: "Escape" });
}

function chips(field: string): string[] {
  const root = screen.getByRole("combobox", { name: field }).closest(".MuiInputBase-root");
  return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
}

function groupChips(): string[] {
  const root = screen.getByLabelText("Customer Group").closest(".MuiInputBase-root");
  return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
}

function productChips(): string[] {
  const root = screen.getByLabelText("Deployment products").closest(".MuiInputBase-root");
  return Array.from(root?.querySelectorAll(".MuiChip-label") ?? []).map((c) => c.textContent ?? "");
}

describe("EditChangeRequestDialog — customer project, deployments, deployment products", () => {
  it("seeds them from the record, with names rather than ids", () => {
    renderDialog(SCOPED_CR);
    expect(screen.getByLabelText("Customer Project")).toHaveValue("proj-a");
    expect(screen.getByLabelText("Customer Project")).toHaveAttribute("data-known-label", "Acme Project");
    expect(chips("Deployments")).toEqual(["Acme Production"]);
    expect(productChips()).toEqual(["API Manager 4.3.0", "Identity Server 7.0.0"]);
  });

  it("has no Environments field", () => {
    renderDialog(SCOPED_CR);
    expect(screen.queryByRole("combobox", { name: "Environments" })).not.toBeInTheDocument();
  });

  it("sends nothing for them, and keeps Save disabled, when none was touched", () => {
    renderDialog(SCOPED_CR);
    expect(saveButton()).toBeDisabled();
  });

  it("renders them empty (Deployments disabled) for a change request with no project", () => {
    renderDialog({ state: "new" });
    expect(screen.getByLabelText("Customer Project")).toHaveValue("");
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeDisabled();
    expect(saveButton()).toBeDisabled();
  });

  it("cascades a deployment change and sends the whole scope together, deployment products included", () => {
    const { onSave } = renderDialog(SCOPED_CR);
    pickOptions("Deployments", ["Acme Staging"]);
    expect(chips("Deployments")).toEqual(["Acme Production", "Acme Staging"]);
    expect(productChips()).toEqual(["API Manager 4.3.0", "Identity Server 7.0.0", "API Manager 4.2.0"]);

    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith({
      projectId: "proj-a",
      deploymentIds: ["dep-prod", "dep-stg"],
      deploymentProductIds: ["dp-apim", "dp-is", "dp-apim-stg"],
    });
  });

  it("drops a removed deployment's products", () => {
    const { onSave } = renderDialog({
      ...SCOPED_CR,
      deployments: [
        { id: "dep-prod", name: "Acme Production" },
        { id: "dep-stg", name: "Acme Staging" },
      ],
      deploymentProducts: [
        { id: "dp-apim", name: "API Manager 4.3.0" },
        { id: "dp-is", name: "Identity Server 7.0.0" },
        { id: "dp-apim-stg", name: "API Manager 4.2.0" },
      ],
    });
    pickOptions("Deployments", ["Acme Production"]); // toggles Production off
    expect(productChips()).toEqual(["API Manager 4.2.0"]);
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      projectId: "proj-a",
      deploymentIds: ["dep-stg"],
      deploymentProductIds: ["dp-apim-stg"],
    });
  });

  it("clears the dependents when the project changes (in New), and sends the new project with empty lists", () => {
    const { onSave } = renderDialog({ ...SCOPED_CR, state: "new" });
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: "proj-b" } });
    expect(chips("Deployments")).toEqual([]);
    expect(productChips()).toEqual([]);

    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      projectId: "proj-b",
      deploymentIds: [],
      deploymentProductIds: [],
    });
  });

  it("sends the chosen project and deployments when a project is set for the first time (in New)", () => {
    const { onSave } = renderDialog({ state: "new" });
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: "proj-b" } });
    pickOptions("Deployments", ["Beta Development"]);
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      projectId: "proj-b",
      deploymentIds: ["dep-b"],
      deploymentProductIds: ["dp-b"],
    });
  });

  it("does not let a saved project be cleared (the patch cannot express it), but does when none is saved", () => {
    const first = render(
      <EditChangeRequestDialog
        cr={{ ...BASE_CR, ...SCOPED_CR, state: "new" }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByLabelText("Customer Project")).toHaveAttribute("data-disable-clearable", "true");
    first.unmount();
    renderDialog({ state: "new" });
    expect(screen.getByLabelText("Customer Project")).toHaveAttribute("data-disable-clearable", "false");
  });

  it.each(["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "canceled"])(
    "locks the deployments as well, with their reason, once the change request is in %s",
    (state) => {
      const { onSave } = renderDialog({ ...SCOPED_CR, state });
      expect(
        screen.getByText(
          state === "canceled"
            ? /deployments can't be changed once the change request has been cancelled/i
            : /deployments can't be changed once the change request has been approved/i,
        ),
      ).toBeInTheDocument();
      expect(screen.getByLabelText("Customer Project")).toBeDisabled();
      expect(screen.getByRole("combobox", { name: "Deployments" })).toBeDisabled();
      expect(saveButton()).toBeDisabled();
      expect(onSave).not.toHaveBeenCalled();
    },
  );

  it("keeps the deployments editable up to and including authorize", () => {
    renderDialog({ ...SCOPED_CR, state: "authorize" });
    expect(screen.queryByText(/can't be changed once the change request has been approved/i)).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeEnabled();
  });
});

describe("EditChangeRequestDialog — the Customer Project is fixed once approval was requested", () => {
  const FROZEN = "Fixed when approval was requested. Cancel and clone to change it.";
  const project = (): HTMLElement => screen.getByLabelText("Customer Project");

  it("is editable in New, with no note about it being fixed", () => {
    renderDialog({ ...SCOPED_CR, state: "new" });
    expect(project()).toBeEnabled();
    expect(screen.queryByText(FROZEN)).not.toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeEnabled();
  });

  it.each(["assess", "authorize"])(
    "is read-only in %s with the reason, while the deployments stay editable",
    (state) => {
      renderDialog({ ...SCOPED_CR, state });
      expect(project()).toBeDisabled();
      expect(project()).toHaveValue("proj-a");
      expect(screen.getByText(FROZEN)).toBeInTheDocument();
      expect(screen.getByRole("combobox", { name: "Deployments" })).toBeEnabled();
    },
  );

  it.each(["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback", "canceled"])(
    "is read-only in %s too, with its reason beside the deployments'",
    (state) => {
      renderDialog({ ...SCOPED_CR, state });
      expect(project()).toBeDisabled();
      expect(screen.getByText(FROZEN)).toBeInTheDocument();
      expect(screen.getByText(/deployments can't be changed once the change request has been (approved|cancelled)/i)).toBeInTheDocument();
    },
  );

  it("sends the stored project along with changed deployments, which is an accepted no-op, never a changed project", () => {
    const { onSave } = renderDialog({ ...SCOPED_CR, state: "authorize" });
    pickOptions("Deployments", ["Acme Staging"]);
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      projectId: "proj-a",
      deploymentIds: ["dep-prod", "dep-stg"],
      deploymentProductIds: ["dp-apim", "dp-is", "dp-apim-stg"],
    });
  });

  it("does not take a typed project as an edit after New (the picker is disabled and the patch has no project)", () => {
    const { onSave } = renderDialog({ ...SCOPED_CR, state: "assess" });
    expect(project()).toBeDisabled();
    expect(saveButton()).toBeDisabled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("is read-only and empty for a change request that has none (it can no longer be set), with Deployments disabled", () => {
    renderDialog({ state: "assess" });
    expect(project()).toBeDisabled();
    expect(project()).toHaveValue("");
    expect(screen.getByText(FROZEN)).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Deployments" })).toBeDisabled();
  });

  it("shows the backend's refusal of an inconsistent combination verbatim", () => {
    const message = "deploymentIds: deployment dep-x does not belong to project proj-a";
    render(
      <EditChangeRequestDialog
        cr={{ ...BASE_CR, ...SCOPED_CR }}
        isSaving={false}
        saveError={message}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(message);
  });
});

describe("EditChangeRequestDialog — Customer Group (the project's registered contacts, read-only)", () => {
  it("shows the record's project's registered contacts as a locked, read-only field", () => {
    renderDialog(SCOPED_CR);
    expect(groupChips()).toEqual(["Alice Aaron", "Bob Bell"]);
    const field = screen.getByLabelText("Customer Group");
    expect(field).toHaveAttribute("readonly");
    expect(field).toHaveAttribute("aria-readonly", "true");
    expect(screen.getByText("Derived from the customer project's registered contacts")).toBeInTheDocument();
  });

  it("is not a picker: no Customer group search field, and typing changes nothing", () => {
    renderDialog(SCOPED_CR);
    expect(screen.queryByLabelText("Customer group")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Customer Group"), { target: { value: "x" } });
    expect(screen.getByLabelText("Customer Group")).toHaveValue("");
    expect(saveButton()).toBeDisabled();
  });

  it("says to choose a project first for a change request that has none", () => {
    renderDialog({ state: "new" });
    expect(groupChips()).toEqual([]);
    expect(screen.getAllByText("Select a Customer Project first.").length).toBeGreaterThan(0);
  });

  it("re-derives when the project changes (in New), and sends no group with the new project", () => {
    const { onSave } = renderDialog({ ...SCOPED_CR, state: "new" });
    fireEvent.change(screen.getByLabelText("Customer Project"), { target: { value: "proj-b" } });
    expect(groupChips()).toEqual(["Carol Cook"]);
    fireEvent.click(saveButton());
    const sent = onSave.mock.calls[0]![0] as Record<string, unknown>;
    expect(sent).toHaveProperty("projectId", "proj-b");
    expect(sent).not.toHaveProperty("customerGroupId");
    expect(sent).not.toHaveProperty("environmentIds");
  });

  it("keeps showing the contacts when the scope is locked (the group is information, not an edit)", () => {
    renderDialog({ ...SCOPED_CR, state: "implement" });
    expect(groupChips()).toEqual(["Alice Aaron", "Bob Bell"]);
  });

  it("shows the backend's 400 about the removed customerGroupId verbatim", () => {
    const message =
      "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts";
    render(
      <EditChangeRequestDialog
        cr={{ ...BASE_CR, ...SCOPED_CR }}
        isSaving={false}
        saveError={message}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(message);
  });
});

describe("EditChangeRequestDialog — Category", () => {
  it("seeds from a plain category value, and leaves Save disabled until it changes", () => {
    renderDialog({ category: "devops" });
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("DevOps");
    expect(saveButton()).toBeDisabled();
  });

  it("seeds from an entity-ref category too", () => {
    renderDialog({ category: { id: "network", name: "Network" } });
    expect(screen.getByRole("combobox", { name: "Category" })).toHaveTextContent("Network");
  });

  it("sends only the new category when it is changed", () => {
    const { onSave } = renderDialog({ category: "devops" });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Category" }));
    fireEvent.click(screen.getByRole("option", { name: "Hotfix Release - Cloud" }));
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ category: "hotfix_release_cloud" });
  });

  it("does not send a category when it is cleared (the patch cannot express it)", () => {
    renderDialog({ category: "devops" });
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Category" }));
    fireEvent.click(screen.getByRole("option", { name: "-- Select --" }));
    expect(saveButton()).toBeDisabled();
  });
});

// ---------------------------------------------------------------------------
// Every field the create form sets can be edited here too (the description used
// not to be, nor the subject, type, impact, priority, justification and risk).
// ---------------------------------------------------------------------------

describe("EditChangeRequestDialog — the fields the create form sets", () => {
  const approvalBox = (): HTMLElement => screen.getByRole("checkbox", { name: "Customer Approval" });
  const reviewBox = (): HTMLElement => screen.getByRole("checkbox", { name: "Customer Review" });
  const subject = (): HTMLElement => screen.getByRole("textbox", { name: /^subject/i });
  const select = (name: string): HTMLElement => screen.getByRole("combobox", { name });
  const choose = (name: string, option: string): void => {
    fireEvent.mouseDown(select(name));
    fireEvent.click(screen.getByRole("option", { name: option }));
  };

  it("seeds each from the record and sends nothing until one changes", () => {
    renderDialog({
      state: "new",
      description: "<p>Move the gateway to the new cluster.</p>",
      justification: "<p>The old cluster is end of life.</p>",
      impactDescription: "<p>Ten minutes of read-only traffic.</p>",
      impact: "medium",
      priority: "moderate",
    });
    expect(subject()).toHaveValue("Upgrade the gateway cluster");
    expect(planEditor(/^description/i)).toHaveValue(normalizeLikeEditor("<p>Move the gateway to the new cluster.</p>"));
    expect(planEditor(/^justification/i)).toHaveValue(normalizeLikeEditor("<p>The old cluster is end of life.</p>"));
    expect(planEditor(/risk and impact analysis/i)).toHaveValue(normalizeLikeEditor("<p>Ten minutes of read-only traffic.</p>"));
    expect(select("Type")).toHaveTextContent("Normal");
    expect(select("Impact")).toHaveTextContent("Medium");
    expect(select("Priority")).toHaveTextContent("Moderate");
    expect(saveButton()).toBeDisabled();
  });

  it("offers no value for planning, which only a migrated change request carries, and sends no priority for it", () => {
    const { onSave } = renderDialog({ priority: "planning" });
    expect(select("Priority")).toHaveTextContent("-- Select --");
    fireEvent.change(subject(), { target: { value: "Upgrade the gateway cluster to 4.3" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ title: "Upgrade the gateway cluster to 4.3" });
  });

  it.each([
    ["description", /^description/i],
    ["justification", /^justification/i],
    ["impactDescription", /risk and impact analysis/i],
  ] as const)("sends only %s when only it was edited", (key, label) => {
    const { onSave } = renderDialog();
    fireEvent.change(planEditor(label), { target: { value: "<p>Revised.</p>" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ [key]: "<p>Revised.</p>" });
  });

  it("sends the subject trimmed, and refuses an empty one", () => {
    const { onSave } = renderDialog();
    fireEvent.change(subject(), { target: { value: "   " } });
    expect(screen.getByText("A change request needs a subject.")).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
    fireEvent.change(subject(), { target: { value: "  Upgrade the gateway cluster to 4.3  " } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ title: "Upgrade the gateway cluster to 4.3" });
  });

  it("sends only the impact, or only the priority, when that is what changed", () => {
    const first = renderDialog({ impact: "low" });
    choose("Impact", "High");
    fireEvent.click(saveButton());
    expect(first.onSave).toHaveBeenCalledWith({ impact: "high" });
    cleanup();
    const second = renderDialog();
    choose("Priority", "Critical");
    fireEvent.click(saveButton());
    expect(second.onSave).toHaveBeenCalledWith({ priority: "critical" });
  });

  it("changes the type in New, and sends only that", () => {
    const { onSave } = renderDialog({ state: "new", type: "normal" });
    expect(select("Type")).not.toHaveAttribute("aria-disabled", "true");
    choose("Type", "Emergency");
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ type: "emergency" });
  });

  it("turns both customer boxes off when the type becomes Emergency in New, in the same request", () => {
    const { onSave } = renderDialog({ state: "new", customerApprovalRequired: true, customerReviewRequired: true });
    choose("Type", "Emergency");
    expect(approvalBox()).not.toBeChecked();
    expect(reviewBox()).not.toBeChecked();
    expect(approvalBox()).toBeDisabled();
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      type: "emergency",
      customerApprovalRequired: false,
      customerReviewRequired: false,
    });
  });

  it.each(["assess", "authorize"])("keeps the type read-only after New (%s), with the reason", (state) => {
    renderDialog({ state });
    expect(select("Type")).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText(/The type decides the approval flow, so it is fixed once approval has been requested/)).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Once the change request has been approved every field is read-only and only a
// work note can be added; before, a work note can ride with any save.
// ---------------------------------------------------------------------------

describe("EditChangeRequestDialog — read-only once approved, work notes only", () => {
  const ACME = { id: "proj-a", name: "Acme Project" };
  const workNote = (): HTMLElement => screen.getByRole("textbox", { name: /work note/i });
  const APPROVED = ["customer_approval", "scheduled", "implement", "review", "customer_review", "closed", "rollback"];

  it.each(APPROVED)("renders every field read-only in %s, with the reason, and saves only a work note", (state) => {
    const { onSave } = renderDialog({
      state,
      project: ACME,
      implementationPlan: "<p>Run the upgrade playbook.</p>",
      category: "devops",
    });
    expect(screen.getByText(/This change request has been approved, so its fields are read-only/)).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /^subject/i })).toBeDisabled();
    for (const name of [/^description/i, /^justification/i, /risk and impact analysis/i, /^implementation plan/i, /rollback plan/i, /test plan/i]) {
      expect(planEditor(name)).toBeDisabled();
    }
    for (const name of ["Type", "Impact", "Priority", "Category"]) {
      expect(screen.getByRole("combobox", { name })).toHaveAttribute("aria-disabled", "true");
    }
    expect(screen.getByLabelText(/^assignment group$/i)).toBeDisabled();
    expect(screen.getByLabelText(/^assigned to$/i)).toBeDisabled();
    expect(screen.getByLabelText(/^requested by$/i)).toBeDisabled();
    expect(screen.getByLabelText(/^originating service request$/i)).toBeDisabled();
    expect(screen.getByRole("textbox", { name: /rollback duration/i })).toBeDisabled();
    expect(screen.getByLabelText("Implementation Plan visible to customers")).toBeDisabled();
    for (const section of within(screen.getByRole("group", { name: /planned start/i })).getAllByRole("spinbutton")) {
      expect(section).toHaveAttribute("aria-disabled", "true");
    }

    expect(workNote()).toBeEnabled();
    expect(saveButton()).toHaveTextContent("Add work note");
    expect(saveButton()).toBeDisabled();
    fireEvent.change(workNote(), { target: { value: "  Implementation started on the primary cluster.  " } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ workNote: "Implementation started on the primary cluster." });
  });

  it.each(["new", "assess", "authorize", "canceled"])("leaves the fields editable in %s (not approved), with no notice", (state) => {
    renderDialog({ state });
    expect(screen.queryByText(/This change request has been approved/)).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /^subject/i })).toBeEnabled();
    expect(planEditor(/^implementation plan/i)).toBeEnabled();
    expect(saveButton()).toHaveTextContent("Save");
  });

  it("sends a work note together with an edit before the approval", () => {
    const { onSave } = renderDialog({ state: "authorize" });
    fireEvent.change(planEditor(/^implementation plan/i), { target: { value: "<p>Step 1.</p>" } });
    fireEvent.change(workNote(), { target: { value: "Added the first step." } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ implementationPlan: "<p>Step 1.</p>", workNote: "Added the first step." });
  });

  it("does not send a blank work note", () => {
    renderDialog({ state: "scheduled" });
    fireEvent.change(workNote(), { target: { value: "   " } });
    expect(saveButton()).toBeDisabled();
  });
});

// ---------------------------------------------------------------------------
// The deviation from the implementation plan: recorded in implement, review and
// customer_review (the plan is read-only by then); before that shown read-only
// only when one is stored -- in New, where only a clone can have carried one over,
// with "Clear deviation" -- and final once the change is closed, canceled or
// rolled back.
// ---------------------------------------------------------------------------

describe("EditChangeRequestDialog — deviation from the implementation plan", () => {
  const deviation = (): HTMLElement => planEditor(/^deviation from the implementation plan/i);
  const deviationGroup = (): HTMLElement | null =>
    screen.queryByRole("group", { name: /^deviation from the implementation plan/i });
  const workNote = (): HTMLElement => screen.getByRole("textbox", { name: /work note/i });
  const clearButton = (): HTMLElement | null => screen.queryByRole("button", { name: "Clear deviation" });
  const RECORDABLE = ["implement", "review", "customer_review"];

  it.each(RECORDABLE)("can be recorded in %s while the plan itself stays read-only", (state) => {
    const { onSave } = renderDialog({ state, implementationPlan: "<p>Run the upgrade playbook.</p>" });
    expect(
      screen.getByText(/fields are read-only\. You can still record a deviation from the implementation plan and add work notes\./),
    ).toBeInTheDocument();
    expect(planEditor(/^implementation plan/i)).toBeDisabled();
    expect(deviation()).toBeEnabled();
    expect(saveButton()).toHaveTextContent("Add work note");
    // At the top with the work note (the inputs left), not among the disabled plan fields.
    const group = screen.getByRole("group", { name: /^deviation from the implementation plan/i });
    const following = (a: Node, b: Node): boolean => !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);
    expect(following(group, workNote())).toBe(true);
    expect(following(workNote(), screen.getByRole("textbox", { name: /^subject/i }))).toBe(true);

    fireEvent.change(deviation(), { target: { value: "<p>Restarted the nodes one at a time.</p>" } });
    expect(saveButton()).toHaveTextContent("Save");
    fireEvent.change(workNote(), { target: { value: "Recorded the deviation." } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({
      implementationDeviation: "<p>Restarted the nodes one at a time.</p>",
      workNote: "Recorded the deviation.",
    });
  });

  it("clears a stored deviation with an empty value", () => {
    const { onSave } = renderDialog({ state: "implement", implementationDeviation: "<p>Used the standby.</p>" });
    fireEvent.change(deviation(), { target: { value: "" } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ implementationDeviation: "" });
  });

  it.each(["customer_approval", "scheduled"])(
    "cannot be recorded yet in %s (approved, implementation not started): only work notes",
    (state) => {
      renderDialog({ state });
      expect(screen.getByText(/fields are read-only\. Only work notes can be added\./)).toBeInTheDocument();
      expect(deviationGroup()).not.toBeInTheDocument();
      expect(saveButton()).toHaveTextContent("Add work note");
    },
  );

  it.each(["assess", "authorize", "customer_approval", "scheduled"])(
    "shows one stored in %s read-only, with why, and offers no way to change it",
    (state) => {
      const { onSave } = renderDialog({ state, implementationDeviation: "<p>Used the standby.</p>" });
      expect(deviation()).toBeDisabled();
      expect(
        screen.getByText("A deviation from the implementation plan can be recorded once implementation has started."),
      ).toBeInTheDocument();
      expect(clearButton()).not.toBeInTheDocument();
      if (state === "customer_approval" || state === "scheduled") {
        expect(saveButton()).toHaveTextContent("Add work note");
        fireEvent.change(workNote(), { target: { value: "Waiting for the window." } });
        fireEvent.click(saveButton());
        expect(onSave).toHaveBeenCalledWith({ workNote: "Waiting for the window." });
      }
    },
  );

  it.each([
    ["closed", "The change request is closed, so its deviation from the implementation plan is final."],
    ["rollback", "The change request is rolled back, so its deviation from the implementation plan is final."],
    ["canceled", "The change request is canceled, so its deviation from the implementation plan is final."],
  ])("is final once the change request is %s: shown read-only with the reason", (state, reason) => {
    renderDialog({ state, implementationDeviation: "<p>Used the standby.</p>" });
    expect(deviation()).toBeDisabled();
    expect(screen.getByText(reason)).toBeInTheDocument();
    expect(clearButton()).not.toBeInTheDocument();
    if (state !== "canceled") {
      expect(screen.getByText(/fields are read-only\. Only work notes can be added\./)).toBeInTheDocument();
    }
  });

  it.each(["closed", "rollback", "canceled"])("is not offered in %s when none was recorded", (state) => {
    renderDialog({ state });
    expect(deviationGroup()).not.toBeInTheDocument();
  });

  it.each(["new", "assess", "authorize"])("is not offered before implementation (%s) unless one is stored", (state) => {
    renderDialog({ state });
    expect(deviationGroup()).not.toBeInTheDocument();
    expect(clearButton()).not.toBeInTheDocument();
  });

  describe("in New, carried over by a clone", () => {
    const CARRIED = "<p>From the source change.</p>";

    it("is shown read-only under the plan, with what to do and a Clear action that the save sends", () => {
      const { onSave } = renderDialog({ state: "new", implementationDeviation: CARRIED });
      // Right under the implementation plan, before the rollback plan.
      const plan = screen.getByRole("group", { name: /^implementation plan/i });
      const dev = screen.getByRole("group", { name: /^deviation from the implementation plan/i });
      const rollback = screen.getByRole("group", { name: /^rollback plan/i });
      expect(plan.compareDocumentPosition(dev) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(dev.compareDocumentPosition(rollback) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(deviation()).toBeDisabled();
      expect((deviation() as HTMLTextAreaElement).value).toContain("From the source change.");
      expect(
        screen.getByText(/^Carried over from the cloned change request\. Fold it into the implementation plan, then clear it before requesting approval\./),
      ).toBeInTheDocument();
      expect(saveButton()).toBeDisabled();

      fireEvent.click(clearButton()!);
      // The text stays on screen (it may still be copied into the plan); the save clears it.
      expect((deviation() as HTMLTextAreaElement).value).toContain("From the source change.");
      expect(screen.getByText(/^It is cleared when you save\./)).toBeInTheDocument();
      expect(saveButton()).toHaveTextContent("Save");
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ implementationDeviation: "" });
    });

    it("folds it into the plan and clears it in one save", () => {
      const { onSave } = renderDialog({ state: "new", implementationDeviation: CARRIED });
      fireEvent.change(planEditor(/^implementation plan/i), { target: { value: "<p>Step 1, as the source change did it.</p>" } });
      fireEvent.click(clearButton()!);
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({
        implementationPlan: "<p>Step 1, as the source change did it.</p>",
        implementationDeviation: "",
      });
    });

    it("keeps it when Keep deviation undoes the clear, and sends nothing for it", () => {
      const { onSave } = renderDialog({ state: "new", implementationDeviation: CARRIED });
      fireEvent.click(clearButton()!);
      fireEvent.click(screen.getByRole("button", { name: "Keep deviation" }));
      expect(clearButton()).toBeInTheDocument();
      expect(saveButton()).toBeDisabled();
      fireEvent.change(planEditor(/^implementation plan/i), { target: { value: "<p>Step 1.</p>" } });
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ implementationPlan: "<p>Step 1.</p>" });
    });

    it("works the same with no state recorded yet", () => {
      const { onSave } = renderDialog({ state: undefined, implementationDeviation: CARRIED });
      fireEvent.click(clearButton()!);
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ implementationDeviation: "" });
    });
  });

  it("is not sent when only reformatted by the editor", () => {
    renderDialog({ state: "implement", implementationDeviation: "<p>Used the standby.</p>" });
    // The editor's own re-serialisation on load is not an edit.
    expect(saveButton()).toBeDisabled();
  });
});

describe("EditChangeRequestDialog — inline variant", () => {
  it("renders the same form in the page, not a modal, and saves only what changed", () => {
    const onSave = vi.fn<(patch: BePatchChangeRequestPayload) => void>();
    render(
      <EditChangeRequestDialog
        variant="inline"
        cr={BASE_CR}
        isSaving={false}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    const region = screen.getByRole("region", { name: "Edit change request" });
    const save = within(region).getByRole("button", { name: /save/i });
    expect(save).toBeDisabled();

    fireEvent.change(within(region).getByLabelText("Rollback duration"), { target: { value: "30 mins" } });
    fireEvent.click(save);
    expect(onSave).toHaveBeenCalledWith({ rollbackDurationText: "30 mins" });
  });

  it("keeps the edit rules: the Customer Project is read-only once approval was requested", () => {
    render(
      <EditChangeRequestDialog
        variant="inline"
        cr={{ ...BASE_CR, state: "assess", project: { id: "proj-a", name: "Acme Project" } }}
        isSaving={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(
      within(screen.getByRole("region", { name: "Edit change request" })).getByText(
        "Fixed when approval was requested. Cancel and clone to change it.",
      ),
    ).toBeInTheDocument();
  });
});

describe("EditChangeRequestDialog — Standard is offered only to a change that already is one", () => {
  const typeOptions = (): string[] => {
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /^type/i }));
    return screen.getAllByRole("option").map((o) => o.textContent ?? "");
  };

  it("lists Normal and Emergency, not Standard, for a Normal change in New", () => {
    renderDialog({ state: "new", type: "normal" });
    const options = typeOptions();
    expect(options).toEqual(expect.arrayContaining(["Normal", "Emergency"]));
    expect(options).not.toContain("Standard");
  });

  it("keeps Standard for an older Standard change in New (not raised from a template)", () => {
    renderDialog({ state: "new", type: "standard" });
    expect(typeOptions()).toContain("Standard");
  });
});

describe("EditChangeRequestDialog — a change raised from a standard change template", () => {
  const TEMPLATED: Partial<BeChangeRequestDetail> = {
    state: "new",
    type: "standard",
    impact: "low",
    category: "software",
    rollbackDurationText: "15 mins",
    customerApprovalRequired: false,
    customerReviewRequired: false,
    description: "<p>Rotate the certificate on gw-1.</p>",
    implementationPlan: "<p>Steps.</p>",
    rollbackPlan: "<p>Restore.</p>",
    testPlan: "<p>Check.</p>",
    standardChange: {
      template: { id: "tpl-1", name: "Rotate a TLS certificate" },
      templateNumber: "STD0000001",
      customer: { id: "acct-1", name: "Acme Corp" },
      versionId: "ver-2",
      version: 2,
      parameters: [{ key: "host", label: "Host", type: "text", required: true, value: "gw-1" }],
    },
  };
  const HINT = "Set by the standard change template.";

  it("offers only the template's customer's projects while the project can still be changed (New)", () => {
    renderDialog(TEMPLATED);
    const project = screen.getByLabelText("Customer Project");
    expect(project).toBeEnabled();
    expect(project).toHaveAttribute("data-account-id", "acct-1");
    expect(screen.getByRole("status", { name: "" })).toHaveTextContent(
      /its customer is acme corp: only acme corp's projects are offered as the customer project/i,
    );
  });

  it("says nothing about the customer's projects once the project is fixed (after New)", () => {
    renderDialog({ ...TEMPLATED, state: "scheduled", legalNextStates: [] });
    expect(screen.getByLabelText("Customer Project")).toBeDisabled();
    expect(screen.queryByText(/only acme corp's projects are offered/i)).not.toBeInTheDocument();
  });

  it("does not narrow the project of a change not raised from a template", () => {
    renderDialog({ ...TEMPLATED, standardChange: null });
    expect(screen.getByLabelText("Customer Project")).toHaveAttribute("data-account-id", "");
  });

  it("says which template the change follows and why its fields are read-only", () => {
    renderDialog(TEMPLATED);
    const notice = screen.getByRole("status", { name: "" });
    expect(notice).toHaveTextContent(/raised from standard change template STD0000001 \(version 2\)/i);
    expect(notice).toHaveTextContent(/specified by the standard change template: it cannot be changed here/i);
  });

  it("disables every field the template specifies, each with a short hint", () => {
    renderDialog(TEMPLATED);
    expect(screen.getByRole("textbox", { name: /^subject/i })).toBeDisabled();
    for (const name of ["Impact", "Category"]) {
      expect(screen.getByRole("combobox", { name }), name).toHaveAttribute("aria-disabled", "true");
    }
    expect(screen.getByRole("checkbox", { name: "Customer Approval" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "Customer Review" })).toBeDisabled();
    expect(screen.getByRole("textbox", { name: /^rollback duration/i })).toBeDisabled();
    expect(screen.getByLabelText("Implementation Plan visible to customers")).toBeDisabled();
    for (const plan of [/^description/i, /^justification/i, /^risk and impact/i, /^implementation plan/i, /^rollback plan/i, /^test plan/i]) {
      expect(planEditor(plan), String(plan)).toBeDisabled();
    }
    expect(screen.getAllByText(HINT).length).toBeGreaterThanOrEqual(10);
  });

  it("keeps the type Standard, with the reason", () => {
    renderDialog(TEMPLATED);
    expect(screen.getByRole("combobox", { name: /^type/i })).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText(/stays a standard change/i)).toBeInTheDocument();
  });

  it("leaves what the template does not specify editable", () => {
    renderDialog(TEMPLATED);
    expect(screen.getByRole("combobox", { name: /^priority/i })).not.toHaveAttribute("aria-disabled");
    expect(planEditor(/^affected services/i)).toBeEnabled();
    expect(planEditor(/^affected components/i)).toBeEnabled();
    expect(screen.getByRole("textbox", { name: /work note/i })).toBeEnabled();
    expect(screen.getByLabelText("Customer Project")).toBeEnabled();
  });

  it("never sends a template field, so a save carries only what was edited", () => {
    const { onSave } = renderDialog(TEMPLATED);
    fireEvent.change(screen.getByRole("textbox", { name: /work note/i }), { target: { value: "Checked the host." } });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ workNote: "Checked the host." });
  });

  it("does not need a subject from the form (the template gives it)", () => {
    renderDialog({ ...TEMPLATED, subject: "" });
    expect(screen.queryByText("A change request needs a subject.")).not.toBeInTheDocument();
  });

  it("leaves a change not raised from a template as it was", () => {
    renderDialog({ ...TEMPLATED, standardChange: null });
    expect(screen.getByRole("textbox", { name: /^subject/i })).toBeEnabled();
    expect(planEditor(/^implementation plan/i)).toBeEnabled();
    expect(screen.queryByText(HINT)).not.toBeInTheDocument();
  });
});
