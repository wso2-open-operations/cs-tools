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
import EditDeployedProductDialog from "@features/csm-projects/components/EditDeployedProductDialog";
import type { BeDeployedProduct } from "@api/backend/types";

const DEPLOYED_PRODUCT: BeDeployedProduct = {
  id: "dp-1",
  product: { id: "prod-1", name: "API Manager" },
  cores: 4,
  tps: 100,
  updates: [{ updateLevel: 1, date: "2026-01-01", details: "Initial rollout" }],
};

function renderDialog(overrides?: Partial<BeDeployedProduct>) {
  const onSaveDetails = vi.fn();
  const onSaveHistory = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <EditDeployedProductDialog
      deployedProduct={{ ...DEPLOYED_PRODUCT, ...overrides }}
      isSaving={false}
      onClose={onClose}
      onSaveDetails={onSaveDetails}
      onSaveHistory={onSaveHistory}
    />,
  );
  return { onSaveDetails, onSaveHistory, onClose };
}

const saveDetailsButton = (): HTMLElement =>
  screen.getByRole("button", { name: /save changes/i });

const switchToHistoryTab = (): void => {
  fireEvent.click(screen.getByRole("tab", { name: /update history/i }));
};

describe("EditDeployedProductDialog — Details tab", () => {
  it("disables Save until a field changes", () => {
    renderDialog();
    expect(saveDetailsButton()).toBeDisabled();
  });

  it("renders numeric cores/tps directly (no re-parse of strings)", () => {
    renderDialog();
    expect(screen.getByLabelText(/cores/i)).toHaveValue(4);
    expect(screen.getByLabelText(/tps/i)).toHaveValue(100);
  });

  it("sends only the changed cores as a number, and does not touch updates", () => {
    const { onSaveDetails, onSaveHistory } = renderDialog();
    fireEvent.change(screen.getByLabelText(/cores/i), { target: { value: "8" } });
    fireEvent.click(saveDetailsButton());
    expect(onSaveDetails).toHaveBeenCalledWith({ cores: 8 });
    expect(onSaveHistory).not.toHaveBeenCalled();
  });

  it("sends null when cores is cleared", () => {
    const { onSaveDetails } = renderDialog();
    fireEvent.change(screen.getByLabelText(/cores/i), { target: { value: "" } });
    fireEvent.click(saveDetailsButton());
    expect(onSaveDetails).toHaveBeenCalledWith({ cores: null });
  });

  it("calling onSaveDetails is the only way the dialog would close (parent decides, not asserted here)", () => {
    // The dialog itself never calls onClose from a save — DeployedProductsPanel
    // decides to close on the Details-save success callback. Confirm no
    // automatic onClose call happens as a side effect of clicking Save.
    const { onClose } = renderDialog();
    fireEvent.change(screen.getByLabelText(/cores/i), { target: { value: "8" } });
    fireEvent.click(saveDetailsButton());
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("EditDeployedProductDialog — Update History tab", () => {
  it("switches to the Update History tab and lists existing entries", () => {
    renderDialog();
    switchToHistoryTab();
    expect(screen.getByText("Initial rollout")).toBeInTheDocument();
    expect(screen.getByText(/date: 2026-01-01/i)).toBeInTheDocument();
  });

  it("shows the footer 'Add update' button only on the Update History tab", () => {
    renderDialog();
    expect(screen.queryByRole("button", { name: /^add update$/i })).not.toBeInTheDocument();
    switchToHistoryTab();
    expect(screen.getByRole("button", { name: /^add update$/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /save changes/i })).not.toBeInTheDocument();
  });

  it("adds a new update-history entry immediately via the footer button, without closing the dialog", async () => {
    const { onSaveHistory, onClose } = renderDialog();
    switchToHistoryTab();

    fireEvent.change(screen.getByLabelText(/^update level$/i), { target: { value: "2" } });
    fireEvent.change(screen.getByLabelText(/^date$/i), { target: { value: "2026-02-01" } });
    fireEvent.click(screen.getByRole("button", { name: /^add update$/i }));

    await waitFor(() =>
      expect(onSaveHistory).toHaveBeenCalledWith([
        { updateLevel: 1, date: "2026-01-01", details: "Initial rollout" },
        { updateLevel: 2, date: "2026-02-01", details: undefined },
      ]),
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("deletes an update-history entry immediately, without a confirm step, and without closing", async () => {
    const { onSaveHistory, onClose } = renderDialog();
    switchToHistoryTab();

    fireEvent.click(screen.getByRole("button", { name: /delete update level 1/i }));

    await waitFor(() => expect(onSaveHistory).toHaveBeenCalledWith([]));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("only the latest (highest updateLevel) entry is editable", () => {
    renderDialog({
      updates: [
        { updateLevel: 1, date: "2026-01-01", details: "Initial rollout" },
        { updateLevel: 2, date: "2026-02-01", details: "Second update" },
      ],
    });
    switchToHistoryTab();

    expect(screen.getByRole("button", { name: /edit update level 2/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /edit update level 1/i })).not.toBeInTheDocument();
  });

  it("edits the latest entry in place and PATCHes the replaced array via the footer button", async () => {
    const { onSaveHistory, onClose } = renderDialog();
    switchToHistoryTab();

    fireEvent.click(screen.getByRole("button", { name: /edit update level 1/i }));
    // Both the in-place edit form and the always-present "Add an update"
    // form section render a "Details" field; the edit form's timeline card
    // appears first in DOM order.
    fireEvent.change(screen.getAllByLabelText(/^details$/i)[0], {
      target: { value: "Revised notes" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));

    await waitFor(() =>
      expect(onSaveHistory).toHaveBeenCalledWith([
        { updateLevel: 1, date: "2026-01-01", details: "Revised notes" },
      ]),
    );
    expect(onClose).not.toHaveBeenCalled();
  });
});
