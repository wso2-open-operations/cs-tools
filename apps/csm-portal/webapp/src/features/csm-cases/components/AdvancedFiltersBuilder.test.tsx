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

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AdvancedFiltersBuilder from "@features/csm-cases/components/AdvancedFiltersBuilder";
import type { UnifiedFilterRow } from "@features/csm-cases/utils/filterFieldAdapters";

// AdvancedFiltersBuilder transitively imports several Async*MultiSelect
// components, each ultimately reaching the real backend API client/config,
// which reads `window.config` at module load time -- not present under
// Vitest. None of them render for the single `createdOn` row this file
// exercises, but the import chain is still walked, so it still needs a safe
// stand-in (see this app's own CLAUDE.md, "Testing").
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ post: vi.fn() }) }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

/** One `createdOn`/`gte` row -- the shape that renders `DateOrPresetValueInput`. */
function createdOnRow(values: string[] = []): UnifiedFilterRow {
  return { field: "createdOn", op: "gte", values, origin: "typed" };
}

function renderWithRow(
  row: UnifiedFilterRow,
): { onUpdateRow: ReturnType<typeof vi.fn>; rerenderWithRow: (next: UnifiedFilterRow) => void } {
  const onUpdateRow = vi.fn();
  const { rerender } = render(
    <AdvancedFiltersBuilder
      rows={[row]}
      onUpdateRow={onUpdateRow}
      onRemoveRow={vi.fn()}
      onAddRow={vi.fn()}
      creTeamOptions={[]}
      sreTeamOptions={[]}
    />,
  );
  const rerenderWithRow = (next: UnifiedFilterRow): void => {
    rerender(
      <AdvancedFiltersBuilder
        rows={[next]}
        onUpdateRow={onUpdateRow}
        onRemoveRow={vi.fn()}
        onAddRow={vi.fn()}
        creTeamOptions={[]}
        sreTeamOptions={[]}
      />,
    );
  };
  return { onUpdateRow, rerenderWithRow };
}

/** The custom date picker's own `FormControl` group. MUI's notched-outline
 * legend duplicates the label text, so this (like CreateOutagePage.test.tsx's
 * own `pickerInput` helper) takes the first match rather than a single-match
 * query. */
function dateFieldGroup(): HTMLElement {
  return screen.getAllByText("Exact date")[0].closest(".MuiFormControl-root") as HTMLElement;
}
/** The hidden text input mirroring the whole date as "MM/DD/YYYY" -- same
 * technique this app's own CreateOutagePage.test.tsx uses for a sectioned
 * MUI DatePicker: a `fireEvent.change` on it enters a complete date without
 * driving each section by hand. */
function dateHiddenInput(): HTMLInputElement {
  return dateFieldGroup().querySelector('input[aria-hidden="true"]') as HTMLInputElement;
}
/** The first (month) spinbutton section, for typing an incomplete date one
 * section at a time. */
function dateMonthSection(): HTMLElement {
  return within(dateFieldGroup()).getAllByRole("spinbutton")[0];
}

function switchToCustomDate(): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: "Date" }));
  fireEvent.click(within(screen.getByRole("listbox")).getByText("Custom date…"));
}

function advance(ms: number): void {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

describe("AdvancedFiltersBuilder — createdOn custom date", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps a half-typed date instead of resetting it", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow());
    switchToCustomDate();
    // Switching from "preset" to "custom" mode fires its own, separate
    // onChange("") (existing, unrelated behaviour: a fresh custom-date
    // session never inherits a leftover preset) -- clear it so the
    // assertions below only see what the typing itself triggers.
    onUpdateRow.mockClear();

    // Type only the month. The real bug: MUI reports this as an invalid
    // (incomplete) date on every such keystroke, and the old code treated
    // that the same as an explicit clear -- committing "" upstream, which
    // round-tripped back down through `value` and wiped the month the user
    // had just typed.
    const month = dateMonthSection();
    fireEvent.focus(month);
    month.textContent = "1";
    fireEvent.input(month);

    expect(dateHiddenInput().value).not.toBe("");
    // Nothing should be committed for an incomplete date, even once the
    // debounce window has fully elapsed.
    advance(1000);
    expect(onUpdateRow).not.toHaveBeenCalled();
    // And the field must still show what was typed -- the actual regression.
    expect(dateHiddenInput().value).not.toBe("");
  });

  it("commits a complete date only after the debounce settles, not immediately", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow());
    switchToCustomDate();
    onUpdateRow.mockClear(); // see the first test's own comment on this call

    fireEvent.change(dateHiddenInput(), { target: { value: "01/15/2026" } });
    expect(onUpdateRow).not.toHaveBeenCalled();

    advance(299);
    expect(onUpdateRow).not.toHaveBeenCalled();

    advance(1);
    expect(onUpdateRow).toHaveBeenCalledTimes(1);
    const [, nextRow] = onUpdateRow.mock.calls[0];
    expect(nextRow.values).toEqual(["2026-01-15"]);
  });

  it("commits an explicit clear immediately, without waiting for the debounce", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow(["2026-01-15"]));
    switchToCustomDate();

    const clearButton = within(dateFieldGroup()).getByRole("button", { name: /clear/i });
    fireEvent.click(clearButton);

    expect(onUpdateRow).toHaveBeenCalledTimes(1);
    const [, nextRow] = onUpdateRow.mock.calls[0];
    expect(nextRow.values).toEqual([]);
  });

  it("switching to a preset cancels a pending custom-date commit", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow());
    switchToCustomDate();
    onUpdateRow.mockClear(); // see the first test's own comment on this call

    // A complete date is typed but its debounce hasn't settled yet.
    fireEvent.change(dateHiddenInput(), { target: { value: "01/15/2026" } });

    // Before it settles, the user picks a preset instead.
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Date" }));
    fireEvent.click(within(screen.getByRole("listbox")).getByText("Today"));

    advance(1000);

    // Only the preset's own, immediate commit should have happened -- the
    // stale custom date must never land afterwards.
    expect(onUpdateRow).toHaveBeenCalledTimes(1);
    const [, nextRow] = onUpdateRow.mock.calls[0];
    expect(nextRow.values).toEqual(["__today__"]);
  });

  it("supersedes a still-pending commit with whatever the user edits to next, valid or not", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow());
    switchToCustomDate();
    onUpdateRow.mockClear(); // see the first test's own comment on this call

    // A complete date is typed but its debounce hasn't settled yet.
    fireEvent.change(dateHiddenInput(), { target: { value: "01/15/2026" } });
    advance(100);

    // The user keeps editing before it fires, landing on a different
    // complete date rather than the one already scheduled.
    fireEvent.change(dateHiddenInput(), { target: { value: "02/20/2026" } });
    advance(1000);

    // Only the latest edit commits -- the superseded "01/15/2026" must never
    // land, not even as an earlier call before the real one.
    expect(onUpdateRow).toHaveBeenCalledTimes(1);
    const [, nextRow] = onUpdateRow.mock.calls[0];
    expect(nextRow.values).toEqual(["2026-02-20"]);
  });

  it("never commits a stale valid date once the user has moved on to an incomplete edit", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow());
    switchToCustomDate();
    onUpdateRow.mockClear(); // see the first test's own comment on this call

    // A complete date is typed but its debounce hasn't settled yet.
    fireEvent.change(dateHiddenInput(), { target: { value: "01/15/2026" } });
    advance(100);

    // The user clears the field down to nothing before it fires -- the
    // field's underlying text clearing (not the dedicated clear button),
    // which reports through the same `null`/incomplete path a section
    // deletion does.
    fireEvent.change(dateHiddenInput(), { target: { value: "" } });

    // The now-superseded "01/15/2026" must never land, however long is
    // waited, and nothing else should commit either (see the dedicated
    // "ignores a section being cleared" test for why `null` itself never
    // commits a clear on its own).
    advance(1000);
    expect(onUpdateRow).not.toHaveBeenCalled();
  });

  it("stays in sync when the parent changes this row's value externally", () => {
    const { onUpdateRow, rerenderWithRow } = renderWithRow(createdOnRow());
    switchToCustomDate();
    onUpdateRow.mockClear(); // see the first test's own comment on this call

    // A date is typed but its debounce hasn't settled yet.
    fireEvent.change(dateHiddenInput(), { target: { value: "01/15/2026" } });

    // The parent resets this exact field/op's value some other way (e.g. a
    // "clear all filters" action) while the row stays mounted -- same key,
    // so this component's own local state does not reset on its own.
    rerenderWithRow(createdOnRow(["2026-02-01"]));

    // The field must show the externally-set date, not the half-settled one.
    expect(dateHiddenInput().value).toBe("02/01/2026");

    // And the earlier, now-superseded commit must never land afterwards.
    advance(1000);
    expect(onUpdateRow).not.toHaveBeenCalled();
  });

  it("ignores a section being cleared while editing -- only the clear button commits an actual clear", () => {
    const { onUpdateRow } = renderWithRow(createdOnRow(["2026-01-15"]));
    switchToCustomDate();
    onUpdateRow.mockClear(); // see the first test's own comment on this call

    // Clearing the underlying field's text (as removing one of its sections
    // while editing does) reports `null` through the picker's main
    // `onChange` -- the same value the dedicated clear button reports
    // through `onClear`. Only the latter may commit a clear.
    fireEvent.change(dateHiddenInput(), { target: { value: "" } });

    advance(1000);
    expect(onUpdateRow).not.toHaveBeenCalled();
  });
});
