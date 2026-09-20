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
import { ThemeProvider, createTheme } from "@wso2/oxygen-ui";
import RejectSolutionDialog from "../RejectSolutionDialog";

function renderDialog(
  props: Partial<Parameters<typeof RejectSolutionDialog>[0]> = {},
) {
  const onClose = vi.fn();
  const onConfirm = vi.fn();
  render(
    <ThemeProvider theme={createTheme()}>
      <RejectSolutionDialog
        open
        isPending={false}
        onClose={onClose}
        onConfirm={onConfirm}
        {...props}
      />
    </ThemeProvider>,
  );
  return { onClose, onConfirm };
}

describe("RejectSolutionDialog", () => {
  it("confirms with an empty reason when submitted blank (no Skip button)", () => {
    const { onConfirm } = renderDialog();

    expect(
      screen.queryByRole("button", { name: /skip/i }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm).toHaveBeenCalledWith("");
  });

  it("confirms with the trimmed reason text entered by the customer", () => {
    const { onConfirm } = renderDialog();

    fireEvent.change(
      screen.getByLabelText("Reason for rejecting the solution"),
      { target: { value: "  Still throwing the same error  " } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Reject Solution" }));

    expect(onConfirm).toHaveBeenCalledWith("Still throwing the same error");
  });

  it("aborts without confirming when Cancel is clicked", () => {
    const { onClose, onConfirm } = renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("disables actions and shows a pending label while submitting", () => {
    renderDialog({ isPending: true });

    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Rejecting…" }),
    ).toBeDisabled();
  });
});
