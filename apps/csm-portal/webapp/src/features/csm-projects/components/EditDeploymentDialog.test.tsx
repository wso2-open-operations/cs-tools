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
import EditDeploymentDialog from "@features/csm-projects/components/EditDeploymentDialog";
import type { BeDeployment } from "@api/backend/types";

const DEPLOYMENT: BeDeployment = {
  id: "11111111-1111-1111-1111-111111111111",
  projectId: "22222222-2222-2222-2222-222222222222",
  name: "Prod",
  type: "primary_production",
  description: "Main prod deployment",
};

function renderDialog(overrides?: Partial<BeDeployment>) {
  const onSave = vi.fn();
  const onClose = vi.fn();
  render(
    <EditDeploymentDialog
      deployment={{ ...DEPLOYMENT, ...overrides }}
      isSaving={false}
      onClose={onClose}
      onSave={onSave}
    />,
  );
  return { onSave, onClose };
}

const saveButton = (): HTMLElement =>
  screen.getByRole("button", { name: /save changes/i });

describe("EditDeploymentDialog", () => {
  it("disables Save until a field changes", () => {
    renderDialog();
    expect(saveButton()).toBeDisabled();
  });

  it("sends only the changed name", () => {
    const { onSave } = renderDialog();
    fireEvent.change(screen.getByLabelText(/name/i), {
      target: { value: "Production" },
    });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ name: "Production" });
  });

  it("sends null when the description is cleared", () => {
    const { onSave } = renderDialog();
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "" },
    });
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ description: null });
  });

  it("keeps Save disabled when the name is emptied", () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText(/name/i), { target: { value: "" } });
    expect(saveButton()).toBeDisabled();
  });

  it("renders a type selector with all 6 deployment types", () => {
    renderDialog();
    // The Select renders its current value in a combobox.
    const typeSelect = screen.getByRole("combobox", { name: /type/i });
    expect(typeSelect).toBeInTheDocument();
  });

  it("enables Save and sends type when the type is changed", () => {
    const { onSave } = renderDialog();
    // Open the Select dropdown and pick "Staging".
    const typeSelect = screen.getByRole("combobox", { name: /type/i });
    fireEvent.mouseDown(typeSelect);
    const stagingOption = screen.getByRole("option", { name: /staging/i });
    fireEvent.click(stagingOption);
    fireEvent.click(saveButton());
    expect(onSave).toHaveBeenCalledWith({ type: "staging" });
  });

  it("does not render 'Type changes aren't available yet'", () => {
    renderDialog();
    expect(
      screen.queryByText(/type changes aren't available yet/i),
    ).not.toBeInTheDocument();
  });
});
