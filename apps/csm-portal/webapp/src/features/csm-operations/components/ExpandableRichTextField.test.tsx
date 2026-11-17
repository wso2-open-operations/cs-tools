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

import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { useEffect, useState, type JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ExpandableRichTextField, {
  RICH_TEXT_MAX_HEIGHT,
  RICH_TEXT_RESIZE_STEP,
} from "@features/csm-operations/components/ExpandableRichTextField";

// The real editor (Lexical) can't be typed into under jsdom. The stand-in keeps what this
// wrapper relies on: it reads `value` once, when it mounts, emits it once on load (as the
// real one does when it loads non-empty text), and emits on every edit.
const mounts: Array<{ seed?: string; minHeight?: unknown; maxHeight?: unknown; disabled?: boolean }> = [];
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: function FakeEditor(props: {
    value?: string;
    onChange?: (html: string) => void;
    minHeight?: number | string;
    maxHeight?: number | string;
    disabled?: boolean;
    ariaRequired?: boolean;
  }): JSX.Element {
    const [seed] = useState(props.value ?? "");
    const { onChange } = props;
    useEffect(() => {
      mounts.push({ seed, minHeight: props.minHeight, maxHeight: props.maxHeight, disabled: props.disabled });
      if (seed) onChange?.(seed);
      // Mount only, like the real editor's initial value.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return (
      <textarea
        aria-label="editor"
        data-min-height={String(props.minHeight)}
        data-max-height={String(props.maxHeight)}
        defaultValue={seed}
        disabled={props.disabled}
        aria-required={props.ariaRequired || undefined}
        onChange={(e) => onChange?.(e.target.value)}
      />
    );
  },
}));

/** The field as a form uses it: the form owns the value. */
function Form({ initial = "<p>Plan</p>", disabled = false, onChange = vi.fn() }: { initial?: string; disabled?: boolean; onChange?: (v: string) => void }): JSX.Element {
  const [value, setValue] = useState(initial);
  return (
    <>
      <ExpandableRichTextField
        id="cr-backout-plan"
        label="Backout plan"
        value={value}
        onChange={(v) => {
          onChange(v);
          setValue(v);
        }}
        disabled={disabled}
        helperText="How the change is undone."
      />
      <output aria-label="form value">{value}</output>
    </>
  );
}

beforeEach(() => {
  mounts.length = 0;
});

describe("ExpandableRichTextField — required", () => {
  it("draws MUI's asterisk after the label, hidden from assistive tech, and marks the editor aria-required", () => {
    render(<ExpandableRichTextField id="cr-plan" label="Implementation plan" value="" onChange={vi.fn()} required />);
    const group = screen.getByRole("group", { name: "Implementation plan" });
    const label = document.getElementById("cr-plan-label")!;
    const mark = label.querySelector(".MuiFormLabel-asterisk");
    expect(mark).toHaveTextContent("*");
    expect(mark).toHaveAttribute("aria-hidden", "true");
    expect(within(group).getByLabelText("editor")).toHaveAttribute("aria-required", "true");
  });

  it("draws no asterisk and no aria-required when the field is optional", () => {
    render(<ExpandableRichTextField id="cr-plan" label="Implementation plan" value="" onChange={vi.fn()} />);
    expect(document.getElementById("cr-plan-label")!.querySelector(".MuiFormLabel-asterisk")).toBeNull();
    expect(screen.getByLabelText("editor")).not.toHaveAttribute("aria-required");
  });
});

describe("ExpandableRichTextField — resize", () => {
  it("grows with the text up to the default height until it is resized", () => {
    render(<Form />);
    const editor = screen.getByRole("textbox", { name: "editor" });
    expect(editor).toHaveAttribute("data-min-height", "100");
    expect(editor).toHaveAttribute("data-max-height", "300");
    expect(screen.getByRole("separator", { name: "Resize Backout plan" })).toHaveAttribute("aria-valuetext", "default size");
  });

  it("is resized with the arrow keys, within bounds, and Home restores the default", () => {
    render(<Form />);
    const handle = screen.getByRole("separator", { name: "Resize Backout plan" });
    // jsdom has no layout, so the first step starts from the default height.
    fireEvent.keyDown(handle, { key: "ArrowDown" });
    expect(handle).toHaveAttribute("aria-valuenow", String(300 + RICH_TEXT_RESIZE_STEP));
    const editor = screen.getByRole("textbox", { name: "editor" });
    expect(editor).toHaveAttribute("data-min-height", "340");
    expect(editor).toHaveAttribute("data-max-height", "340");

    fireEvent.keyDown(handle, { key: "ArrowUp" });
    fireEvent.keyDown(handle, { key: "ArrowUp" });
    expect(handle).toHaveAttribute("aria-valuenow", String(300 - RICH_TEXT_RESIZE_STEP));
    for (let i = 0; i < 20; i++) fireEvent.keyDown(handle, { key: "ArrowUp" });
    expect(handle).toHaveAttribute("aria-valuenow", "100");
    for (let i = 0; i < 40; i++) fireEvent.keyDown(handle, { key: "ArrowDown" });
    expect(handle).toHaveAttribute("aria-valuenow", String(RICH_TEXT_MAX_HEIGHT));

    fireEvent.keyDown(handle, { key: "Home" });
    expect(handle).not.toHaveAttribute("aria-valuenow");
    expect(screen.getByRole("textbox", { name: "editor" })).toHaveAttribute("data-max-height", "300");
  });

  it("is resized by dragging the handle, and a double-click restores the default", () => {
    render(<Form />);
    const handle = screen.getByRole("separator", { name: "Resize Backout plan" });
    fireEvent.pointerDown(handle, { clientY: 500, pointerId: 1 });
    fireEvent.pointerMove(handle, { clientY: 650, pointerId: 1 });
    fireEvent.pointerUp(handle, { clientY: 650, pointerId: 1 });
    expect(handle).toHaveAttribute("aria-valuenow", "450");
    // Moving after release does nothing.
    fireEvent.pointerMove(handle, { clientY: 900, pointerId: 1 });
    expect(handle).toHaveAttribute("aria-valuenow", "450");
    fireEvent.doubleClick(handle);
    expect(handle).not.toHaveAttribute("aria-valuenow");
  });

  it("describes the panel with its helper text", () => {
    render(<Form />);
    expect(screen.getByRole("group", { name: "Backout plan" })).toHaveAccessibleDescription("How the change is undone.");
  });
});

describe("ExpandableRichTextField — full screen", () => {
  it("opens full screen with the current text, edits the same field live, and the panel shows it after Done", async () => {
    const onChange = vi.fn();
    render(<Form onChange={onChange} />);
    // The inline editor's load emission.
    expect(onChange).toHaveBeenCalledTimes(1);
    onChange.mockClear();

    fireEvent.click(screen.getByRole("button", { name: "Expand Backout plan to full screen" }));
    const dialog = screen.getByRole("dialog", { name: "Backout plan" });
    const big = within(dialog).getByRole("textbox", { name: "editor" });
    expect(big).toHaveValue("<p>Plan</p>");
    expect(big).toHaveAttribute("data-max-height", "none");
    // Its own load is not an edit: nothing is forwarded.
    expect(onChange).not.toHaveBeenCalled();
    // The inline panel steps aside while full screen is open.
    expect(screen.getByText(/Open in full screen/)).toBeInTheDocument();

    fireEvent.change(big, { target: { value: "<p>Plan, longer</p>" } });
    expect(onChange).toHaveBeenLastCalledWith("<p>Plan, longer</p>");
    fireEvent.click(within(dialog).getByRole("button", { name: "Done" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(screen.getByRole("textbox", { name: "editor" })).toHaveValue("<p>Plan, longer</p>");
    expect(screen.getByLabelText("form value")).toHaveTextContent("<p>Plan, longer</p>");
  });

  it("keeps the inline editor as it was when full screen changed nothing", async () => {
    render(<Form />);
    const mountsBefore = mounts.filter((m) => m.maxHeight !== "none").length;
    fireEvent.click(screen.getByRole("button", { name: "Expand Backout plan to full screen" }));
    fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    // Remounted once (it stepped aside), seeded with the same text.
    const inlineMounts = mounts.filter((m) => m.maxHeight !== "none");
    expect(inlineMounts.length).toBe(mountsBefore + 1);
    expect(inlineMounts.at(-1)!.seed).toBe("<p>Plan</p>");
  });

  it("opens a disabled field read-only, for reviewing, and never writes to it", () => {
    const onChange = vi.fn();
    render(<Form disabled onChange={onChange} />);
    onChange.mockClear();
    fireEvent.click(screen.getByRole("button", { name: "Expand Backout plan to full screen" }));
    const dialog = screen.getByRole("dialog", { name: "Backout plan (read-only)" });
    const big = within(dialog).getByRole("textbox", { name: "editor" });
    expect(big).toBeDisabled();
    act(() => {
      fireEvent.change(big, { target: { value: "<p>changed</p>" } });
    });
    expect(onChange).not.toHaveBeenCalled();
  });
});
