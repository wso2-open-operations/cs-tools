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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import UpdateHistoryPanel from "@features/csm-projects/components/UpdateHistoryPanel";

const UPDATES = [
  { updateLevel: 10, date: "2026-01-01" },
  { updateLevel: 20, date: "2026-02-01" },
];

function renderPanel(onSaveUpdates = vi.fn().mockResolvedValue(undefined)) {
  const onFormStateChange = vi.fn();
  render(
    <UpdateHistoryPanel
      updates={UPDATES}
      onSaveUpdates={onSaveUpdates}
      onFormStateChange={onFormStateChange}
    />,
  );
  return { onSaveUpdates, onFormStateChange };
}

describe("UpdateHistoryPanel delete", () => {
  it("asks for confirmation and does not save when cancelled", () => {
    const { onSaveUpdates } = renderPanel();
    fireEvent.click(screen.getByLabelText("Delete update level 10"));

    expect(screen.getByText("Delete this update?")).toBeInTheDocument();
    expect(onSaveUpdates).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onSaveUpdates).not.toHaveBeenCalled();
  });

  it("saves the array without the entry once confirmed", async () => {
    const { onSaveUpdates } = renderPanel();
    fireEvent.click(screen.getByLabelText("Delete update level 10"));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(onSaveUpdates).toHaveBeenCalledWith([UPDATES[1]]));
  });

  it("lifts an add handler to the dialog footer that submits a valid form", async () => {
    const { onSaveUpdates, onFormStateChange } = renderPanel();
    fireEvent.change(screen.getByLabelText("Update level"), { target: { value: "30" } });
    fireEvent.change(screen.getByLabelText("Date"), { target: { value: "2026-03-01" } });

    const lifted = onFormStateChange.mock.calls.map((c) => c[0]).filter(Boolean).pop();
    expect(lifted.canAdd).toBe(true);
    lifted.handleAdd();
    await waitFor(() =>
      expect(onSaveUpdates).toHaveBeenCalledWith([
        ...UPDATES,
        { updateLevel: 30, date: "2026-03-01", details: undefined },
      ]),
    );
  });
});
