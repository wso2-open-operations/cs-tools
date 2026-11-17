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
import { CreateCallRequestDialog } from "@features/csm-cases/components/CreateCallRequestDialog";

function renderDialog(): void {
  render(
    <CreateCallRequestDialog
      open
      submitting={false}
      error={null}
      severity="S2"
      onClose={vi.fn()}
      onSubmit={vi.fn()}
    />,
  );
}

describe("CreateCallRequestDialog — required fields", () => {
  it("marks Duration, Reason and the Preferred times caption required", () => {
    renderDialog();
    expect(screen.getByRole("spinbutton", { name: /^duration \(minutes\)$/i })).toBeRequired();
    expect(screen.getByRole("textbox", { name: /^reason for the call$/i })).toBeRequired();
    // The time-slot pickers have no label of their own: the caption above them
    // carries the asterisk for "at least one preferred time".
    const caption = screen.getByText(/^Preferred times/);
    expect(caption.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
  });

  it("keeps Submit disabled with a reason but no preferred time", () => {
    renderDialog();
    fireEvent.change(screen.getByRole("textbox", { name: /^reason for the call$/i }), {
      target: { value: "Walk through the upgrade" },
    });
    expect(screen.getByRole("button", { name: /^submit request$/i })).toBeDisabled();
  });
});
