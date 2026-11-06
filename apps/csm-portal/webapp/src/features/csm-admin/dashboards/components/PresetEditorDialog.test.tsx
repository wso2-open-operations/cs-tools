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
import PresetEditorDialog from "@features/csm-admin/dashboards/components/PresetEditorDialog";

describe("PresetEditorDialog — required fields", () => {
  it("marks the preset name and its one condition as required", () => {
    render(<PresetEditorDialog takenNames={[]} onClose={vi.fn()} onSave={vi.fn()} />);

    expect(screen.getByLabelText("Preset name")).toBeRequired();
    const conditionHeading = screen.getByText("Condition", { selector: "h6" });
    expect(conditionHeading.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
  });

  it("keeps Save disabled without a name or without a condition", () => {
    render(<PresetEditorDialog takenNames={[]} onClose={vi.fn()} onSave={vi.fn()} />);
    const save = screen.getByRole("button", { name: "Save preset" });

    fireEvent.change(screen.getByLabelText("Preset name"), { target: { value: "openOnly" } });
    // A name alone is not enough: there is no condition yet.
    expect(save).toBeDisabled();
  });

  it("saves once both are given", () => {
    const onSave = vi.fn();
    render(
      <PresetEditorDialog
        preset={{ name: "", filter: { field: "state", op: "in", values: ["open"] } }}
        takenNames={[]}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );
    const save = screen.getByRole("button", { name: "Save preset" });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Preset name"), { target: { value: "openOnly" } });
    expect(save).toBeEnabled();
    fireEvent.click(save);
    expect(onSave).toHaveBeenCalledWith({
      name: "openOnly",
      filter: { field: "state", op: "in", values: ["open"] },
    });
  });
});
