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

import EscalateCaseDialog from "@features/csm-cases/components/EscalateCaseDialog";

describe("EscalateCaseDialog", () => {
  it("requires a reason to escalate — Escalate stays disabled until one is entered", () => {
    const onSave = vi.fn();
    render(
      <EscalateCaseDialog
        action="ESCALATE"
        currentLevel="1"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    const submit = screen.getByRole("button", { name: "Escalate" });
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/^Reason/), {
      target: { value: "Customer is blocked in production." },
    });
    expect(submit).not.toBeDisabled();
    fireEvent.click(submit);
    expect(onSave).toHaveBeenCalledWith("Customer is blocked in production.");
  });

  it("does not submit a whitespace-only reason while escalating", () => {
    render(
      <EscalateCaseDialog
        action="ESCALATE"
        currentLevel="1"
        isSaving={false}
        onClose={() => {}}
        onSave={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByLabelText(/^Reason/), {
      target: { value: "   " },
    });
    expect(screen.getByRole("button", { name: "Escalate" })).toBeDisabled();
  });

  it("allows de-escalating with no reason at all", () => {
    const onSave = vi.fn();
    render(
      <EscalateCaseDialog
        action="DEESCALATE"
        currentLevel="2"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    const submit = screen.getByRole("button", { name: "De-escalate" });
    expect(submit).not.toBeDisabled();
    fireEvent.click(submit);
    expect(onSave).toHaveBeenCalledWith(undefined);
  });

  it("passes a trimmed, non-empty reason through even when de-escalating", () => {
    const onSave = vi.fn();
    render(
      <EscalateCaseDialog
        action="DEESCALATE"
        currentLevel="2"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByLabelText(/^Reason/), {
      target: { value: "  Resolved on a call.  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "De-escalate" }));
    expect(onSave).toHaveBeenCalledWith("Resolved on a call.");
  });

  it("shows the inline error message from a failed attempt", () => {
    render(
      <EscalateCaseDialog
        action="ESCALATE"
        currentLevel="5"
        isSaving={false}
        errorMessage="This case is already at the maximum escalation level."
        onClose={() => {}}
        onSave={vi.fn()}
      />,
    );
    expect(
      screen.getByText("This case is already at the maximum escalation level."),
    ).toBeInTheDocument();
  });

  it("calls onClose on Cancel without calling onSave", () => {
    const onSave = vi.fn();
    const onClose = vi.fn();
    render(
      <EscalateCaseDialog
        action="ESCALATE"
        currentLevel="1"
        isSaving={false}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });
});
