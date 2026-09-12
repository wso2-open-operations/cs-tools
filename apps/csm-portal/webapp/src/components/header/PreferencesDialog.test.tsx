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

import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PreferencesDialog from "@components/header/PreferencesDialog";
import { ThemePreferenceProvider } from "@context/theme/ThemePreferenceContext";
import { CaseTabsBehaviorProvider } from "@context/case-tabs/CaseTabsBehaviorContext";

function renderDialog(open = true, onClose = vi.fn()) {
  return {
    onClose,
    ...render(
      <ThemePreferenceProvider>
        <CaseTabsBehaviorProvider>
          <PreferencesDialog open={open} onClose={onClose} />
        </CaseTabsBehaviorProvider>
      </ThemePreferenceProvider>,
    ),
  };
}

describe("PreferencesDialog", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("renders nothing visible when closed", () => {
    renderDialog(false);
    expect(screen.queryByText("Preferences")).not.toBeInTheDocument();
  });

  it("shows the theme select and the case-tabs preferences when open", () => {
    renderDialog(true);
    expect(screen.getByText("Preferences")).toBeInTheDocument();
    expect(screen.getByLabelText("Select theme")).toBeInTheDocument();
    expect(screen.getByLabelText("Open cases in tabs")).toBeInTheDocument();
    expect(screen.getByLabelText("When the tab limit is reached")).toBeInTheDocument();
  });

  // Tabs are on by default (`CaseTabsBehaviorContext`'s `DEFAULT_ENABLED`),
  // so the cap-mode select starts enabled here — turning the toggle OFF is
  // what disables it, not the other way around.
  it("the cap-mode select is enabled by default (tabs on), and disables when the tabs toggle is turned off", () => {
    renderDialog(true);
    // MUI's `Select` doesn't set the native `disabled` attribute on its
    // rendered root — it applies the `Mui-disabled` class instead.
    expect(screen.getByLabelText("When the tab limit is reached")).not.toHaveClass(
      "Mui-disabled",
    );
    fireEvent.click(screen.getByLabelText("Open cases in tabs"));
    expect(screen.getByLabelText("When the tab limit is reached")).toHaveClass("Mui-disabled");
  });

  it("offers exactly the two eviction cap-mode options, with no 'block' choice", () => {
    renderDialog(true);
    // Tabs are on by default, so the select is already enabled — no need to
    // toggle anything first. MUI's `Select` opens its options popup on
    // `mousedown` of its inner `combobox`-role child, not a `click` on the
    // `aria-label`led outer root (same pattern as this codebase's other MUI
    // `Select` interaction tests, e.g. `ChangeCaseTypeDialog.test.tsx`) —
    // the `aria-label` lands on that outer root, not the combobox itself,
    // so this drills into it via the already-`aria-label`led root instead
    // of matching by name.
    fireEvent.mouseDown(
      screen
        .getByLabelText("When the tab limit is reached")
        .querySelector('[role="combobox"]')!,
    );
    expect(screen.getByRole("option", { name: "Replace the last tab" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Replace the oldest tab" })).toBeInTheDocument();
    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("calls onClose from the dialog's own close button", () => {
    const { onClose } = renderDialog(true);
    fireEvent.click(screen.getByLabelText("Close"));
    expect(onClose).toHaveBeenCalled();
  });
});
