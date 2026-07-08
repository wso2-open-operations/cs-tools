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
import ResolutionDialog from "@features/csm-cases/components/ResolutionDialog";

/** Opens a Select and picks the option by visible text. */
function chooseOption(labelId: string, optionText: RegExp): void {
  fireEvent.mouseDown(document.getElementById(labelId)!.parentElement!.querySelector('[role="combobox"]')!);
  const listbox = screen.getByRole("listbox");
  fireEvent.click(within(listbox).getByText(optionText));
}

describe("ResolutionDialog", () => {
  it("disables submit until both resolution code and cause are chosen", () => {
    render(
      <ResolutionDialog
        kind="close"
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /close case/i })).toBeDisabled();

    chooseOption("resolution-code-label", /solved fixed by support guidance provided/i);
    expect(screen.getByRole("button", { name: /close case/i })).toBeDisabled();

    chooseOption("case-cause-label", /application bug/i);
    expect(screen.getByRole("button", { name: /close case/i })).not.toBeDisabled();
  });

  it("submits the chosen resolution code, cause, and close notes", () => {
    const onSubmit = vi.fn();
    render(
      <ResolutionDialog
        kind="propose_solution"
        isSubmitting={false}
        onClose={() => {}}
        onSubmit={onSubmit}
      />,
    );
    chooseOption("resolution-code-label", /solved by customer/i);
    chooseOption("case-cause-label", /unknown/i);
    fireEvent.change(screen.getByLabelText(/close notes/i), {
      target: { value: "Root-caused and verified with the customer." },
    });
    fireEvent.click(screen.getByRole("button", { name: /propose solution/i }));
    expect(onSubmit).toHaveBeenCalledWith({
      resolutionCode: "SOLVED_BY_CUSTOMER",
      cause: "UNKNOWN",
      closeNotes: "Root-caused and verified with the customer.",
    });
  });

  it("calls onClose when cancelled", () => {
    const onClose = vi.fn();
    render(
      <ResolutionDialog
        kind="close"
        isSubmitting={false}
        onClose={onClose}
        onSubmit={() => {}}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
