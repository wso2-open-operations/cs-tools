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
import UpdateHistoryPanel, {
  type UpdateHistoryFormState,
} from "@features/csm-projects/components/UpdateHistoryPanel";

function renderPanel() {
  let formState: UpdateHistoryFormState | null = null;
  render(
    <UpdateHistoryPanel
      updates={[{ updateLevel: 1, date: "2026-01-01", details: "Initial rollout" }]}
      onSaveUpdates={vi.fn().mockResolvedValue(undefined)}
      onFormStateChange={(s) => {
        formState = s;
      }}
    />,
  );
  return { canAdd: () => formState?.canAdd };
}

describe("UpdateHistoryPanel — required fields", () => {
  it("marks Update level and Date required on the add form, and Details optional", () => {
    const { canAdd } = renderPanel();

    expect(screen.getByRole("spinbutton", { name: "Update level" })).toBeRequired();
    expect(screen.getByLabelText(/^date\b/i)).toBeRequired();
    expect(screen.getByRole("textbox", { name: "Details" })).not.toBeRequired();
    expect(canAdd()).toBe(false);

    fireEvent.change(screen.getByRole("spinbutton", { name: "Update level" }), {
      target: { value: "2" },
    });
    expect(canAdd()).toBe(false);
    fireEvent.change(screen.getByLabelText(/^date\b/i), { target: { value: "2026-02-01" } });
    expect(canAdd()).toBe(true);
  });

  it("marks them required on the in-place edit form too", () => {
    renderPanel();
    fireEvent.click(screen.getByRole("button", { name: /edit update level 1/i }));

    // The edit form renders first, above the always-present add form.
    const [editLevel] = screen.getAllByRole("spinbutton", { name: "Update level" });
    const [editDate] = screen.getAllByLabelText(/^date\b/i);
    expect(editLevel).toBeRequired();
    expect(editDate).toBeRequired();

    fireEvent.change(editDate, { target: { value: "" } });
    expect(screen.getByRole("button", { name: /^save$/i })).toBeDisabled();
  });
});
