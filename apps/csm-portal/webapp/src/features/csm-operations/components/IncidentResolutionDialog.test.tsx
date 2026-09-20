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
import IncidentResolutionDialog from "@features/csm-operations/components/IncidentResolutionDialog";

/** Opens a Select and picks the option by visible text. */
function chooseOption(labelId: string, optionText: RegExp): void {
  fireEvent.mouseDown(document.getElementById(labelId)!.parentElement!.querySelector('[role="combobox"]')!);
  const listbox = screen.getByRole("listbox");
  fireEvent.click(within(listbox).getByText(optionText));
}

describe("IncidentResolutionDialog", () => {
  it("disables submit until a resolution code and resolution notes are both provided", () => {
    render(
      <IncidentResolutionDialog
        target="RESOLVED"
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /move to resolved/i })).toBeDisabled();

    chooseOption("incident-resolution-code-label", /^solved \(work around\)$/i);
    expect(screen.getByRole("button", { name: /move to resolved/i })).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/resolution notes/i), {
      target: { value: "Restarted the affected node." },
    });
    expect(screen.getByRole("button", { name: /move to resolved/i })).not.toBeDisabled();
  });

  it("submits the chosen resolution code and notes", () => {
    const onSubmit = vi.fn();
    render(
      <IncidentResolutionDialog
        target="CLOSED"
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    chooseOption("incident-resolution-code-label", /^duplicate$/i);
    fireEvent.change(screen.getByLabelText(/resolution notes/i), {
      target: { value: "Duplicate of INC0012345." },
    });
    fireEvent.click(screen.getByRole("button", { name: /move to closed/i }));
    expect(onSubmit).toHaveBeenCalledWith({
      resolutionCode: "DUPLICATE",
      resolutionNotes: "Duplicate of INC0012345.",
    });
  });

  it("calls onClose when cancelled", () => {
    const onClose = vi.fn();
    render(
      <IncidentResolutionDialog
        target="RESOLVED"
        isSubmitting={false}
        onClose={onClose}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
