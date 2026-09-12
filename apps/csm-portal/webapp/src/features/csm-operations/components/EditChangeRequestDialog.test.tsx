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
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { BeChangeRequestDetail, BePatchChangeRequestPayload } from "@api/backend/types";


// The "Assignment group" picker goes through useSearchGroups, which hits the
// backend client via react-query — stub it out (same approach as
// EditIncidentDialog.test.tsx).
const useSearchGroupsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (...args: unknown[]) => useSearchGroupsMock(...(args as [])),
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
