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
import ProblemFixNotesDialog from "@features/csm-operations/components/ProblemFixNotesDialog";

describe("ProblemFixNotesDialog", () => {
  it("confirms with empty causeNotes/fixNotes when the engineer skips them", () => {
    const onConfirm = vi.fn();
    render(<ProblemFixNotesDialog isSubmitting={false} onClose={vi.fn()} onConfirm={onConfirm} />);
    fireEvent.click(screen.getByRole("button", { name: "Move to Fix In Progress" }));
    expect(onConfirm).toHaveBeenCalledWith({ causeNotes: "", fixNotes: "" });
  });

  it("confirms with the filled-in notes, trimmed", () => {
    const onConfirm = vi.fn();
    render(<ProblemFixNotesDialog isSubmitting={false} onClose={vi.fn()} onConfirm={onConfirm} />);
    fireEvent.change(screen.getByLabelText("Cause notes"), {
      target: { value: "  bad config push  " },
    });
    fireEvent.change(screen.getByLabelText("Fix notes"), {
      target: { value: "rolled back" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Move to Fix In Progress" }));
    expect(onConfirm).toHaveBeenCalledWith({ causeNotes: "bad config push", fixNotes: "rolled back" });
  });

  it("shows the passed-in error message", () => {
    render(
      <ProblemFixNotesDialog
        isSubmitting={false}
        error="State transition rejected"
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(screen.getByText("State transition rejected")).toBeInTheDocument();
  });

  it("disables Cancel and Confirm while submitting", () => {
    render(<ProblemFixNotesDialog isSubmitting onClose={vi.fn()} onConfirm={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Move to Fix In Progress" })).toBeDisabled();
  });

  it("calls onClose when Cancel is clicked and not submitting", () => {
    const onClose = vi.fn();
    render(<ProblemFixNotesDialog isSubmitting={false} onClose={onClose} onConfirm={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
  });
});
