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
import type { BeProblemDetail } from "@api/backend/types";

// The dialog's async reference pickers (Assigned to, Assignment group) go
// through `useSearch*` hooks that hit the backend client via react-query.
// Stub them out — same approach as EditIncidentDialog.test.tsx.
const useSearchUsersByNameMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));
const useSearchGroupsMock = vi.fn(() => ({ data: [], isFetching: false, isError: false }));

vi.mock("@api/useSearchUsersByName", () => ({
  useSearchUsersByName: (...args: unknown[]) => useSearchUsersByNameMock(...(args as [])),
}));
vi.mock("@api/useSearchGroups", () => ({
  useSearchGroups: (...args: unknown[]) => useSearchGroupsMock(...(args as [])),
}));

import EditProblemDialog from "@features/csm-operations/components/EditProblemDialog";

const BASE_PROBLEM: BeProblemDetail = {
  id: "prb-1",
  number: "PRB0040157",
  subject: "Intermittent 502s",
  state: "NEW",
  assignedTo: { id: "user-1", name: "Jane Doe" },
  workaround: "Restart the pod.",
};

describe("EditProblemDialog", () => {
  it("disables Save when nothing has changed", () => {
    render(<EditProblemDialog problem={BASE_PROBLEM} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("enables Save and submits only the changed workaround", () => {
    const onSave = vi.fn();
    render(<EditProblemDialog problem={BASE_PROBLEM} isSaving={false} onClose={vi.fn()} onSave={onSave} />);
    fireEvent.change(screen.getByLabelText("Workaround"), {
      target: { value: "Restart the pod, then clear the cache." },
    });
    const saveBtn = screen.getByRole("button", { name: "Save" });
    expect(saveBtn).not.toBeDisabled();
    fireEvent.click(saveBtn);
    expect(onSave).toHaveBeenCalledWith({ workaround: "Restart the pod, then clear the cache." });
  });

  it("does not pre-fill assignmentGroupId or targetResolutionDate (not returned by GET)", () => {
    render(<EditProblemDialog problem={BASE_PROBLEM} isSaving={false} onClose={vi.fn()} onSave={vi.fn()} />);
    expect(
      screen.getByText(
        "Not shown pre-filled — the portal can't yet read a problem's current assignment group back.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the passed-in save error", () => {
    render(
      <EditProblemDialog
        problem={BASE_PROBLEM}
        isSaving={false}
        saveError="Could not update the problem."
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByText("Could not update the problem.")).toBeInTheDocument();
  });

  it("disables actions while saving", () => {
    render(<EditProblemDialog problem={BASE_PROBLEM} isSaving onClose={vi.fn()} onSave={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Saving…" })).toBeDisabled();
  });

  it("calls onClose when Cancel is clicked", () => {
    const onClose = vi.fn();
    render(<EditProblemDialog problem={BASE_PROBLEM} isSaving={false} onClose={onClose} onSave={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
  });
});
