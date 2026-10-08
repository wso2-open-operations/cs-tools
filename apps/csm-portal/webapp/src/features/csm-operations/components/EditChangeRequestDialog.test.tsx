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

import { fireEvent, render, screen, within } from "@testing-library/react";
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
    onChange,
  }: {
    label: string;
    value: string;
    knownLabel?: string;
    disableClearable?: boolean;
    disabled?: boolean;
    onChange: (next: string, name?: string) => void;
  }) => (
    <input
      aria-label={label}
      data-known-label={knownLabel ?? ""}
      data-disable-clearable={String(!!disableClearable)}
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

/** The dialog's Save button. */
const saveButton = (): HTMLElement => screen.getByRole("button", { name: /save/i });

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

    it.each(["assess", "authorize", "customer_approval", "scheduled", "implement", "review"])(
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

    it("Customer Review can still be added while Customer Approval is past its gate, and sends only that", () => {
      const { onSave } = renderDialog({
        state: "scheduled",
        project: ACME,
        customerApprovalRequired: true,
        customerReviewRequired: false,
      });
      expect(approvalBox()).toBeDisabled();
      fireEvent.click(reviewBox());
      fireEvent.click(saveButton());
      expect(onSave).toHaveBeenCalledWith({ customerReviewRequired: true });
    });
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

const SCOPED_CR: Partial<BeChangeRequestDetail> = {
  state: "scheduled",
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

  it.each(["implement", "review", "customer_review", "closed", "canceled"])(
    "locks the deployments as well, with their reason, once the change request is in %s",
    (state) => {
      const { onSave } = renderDialog({ ...SCOPED_CR, state });
      expect(screen.getByText(/deployments can't be changed once implementation has started/i)).toBeInTheDocument();
      expect(screen.getByLabelText("Customer Project")).toBeDisabled();
      expect(screen.getByRole("combobox", { name: "Deployments" })).toBeDisabled();
      expect(saveButton()).toBeDisabled();
      expect(onSave).not.toHaveBeenCalled();
    },
  );

  it("keeps the deployments editable up to and including scheduled", () => {
    renderDialog({ ...SCOPED_CR, state: "scheduled" });
    expect(screen.queryByText(/can't be changed once implementation/i)).not.toBeInTheDocument();
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

  it.each(["assess", "authorize", "customer_approval", "scheduled"])(
    "is read-only in %s with the reason, while the deployments stay editable",
    (state) => {
      renderDialog({ ...SCOPED_CR, state });
      expect(project()).toBeDisabled();
      expect(project()).toHaveValue("proj-a");
      expect(screen.getByText(FROZEN)).toBeInTheDocument();
      expect(screen.getByRole("combobox", { name: "Deployments" })).toBeEnabled();
    },
  );

  it.each(["implement", "review", "customer_review", "closed", "rollback", "canceled"])(
    "is read-only in %s too, with its reason beside the deployments'",
    (state) => {
      renderDialog({ ...SCOPED_CR, state });
      expect(project()).toBeDisabled();
      expect(screen.getByText(FROZEN)).toBeInTheDocument();
      expect(screen.getByText(/deployments can't be changed once implementation has started/i)).toBeInTheDocument();
    },
  );

  it("sends the stored project along with changed deployments, which is an accepted no-op, never a changed project", () => {
    const { onSave } = renderDialog({ ...SCOPED_CR, state: "scheduled" });
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
