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
import CreateDeploymentDialog from "@features/csm-projects/components/CreateDeploymentDialog";

const PROJECT_ID = "33333333-3333-3333-3333-333333333333";

function renderDialog() {
  const onCreate = vi.fn();
  const onClose = vi.fn();
  render(
    <CreateDeploymentDialog
      projectId={PROJECT_ID}
      isSaving={false}
      onClose={onClose}
      onCreate={onCreate}
    />,
  );
  return { onCreate, onClose };
}

const createButton = (): HTMLElement =>
  screen.getByRole("button", { name: /^create$/i });

describe("CreateDeploymentDialog", () => {
  it("disables Create when no fields are filled", () => {
    renderDialog();
    expect(createButton()).toBeDisabled();
  });

  it("disables Create when type is missing", () => {
    renderDialog();
    fireEvent.change(screen.getByLabelText(/name/i), {
      target: { value: "My deployment" },
    });
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Some description" },
    });
    expect(createButton()).toBeDisabled();
  });

  it("disables Create when name is missing", () => {
    renderDialog();
    // Select a type.
    const typeSelect = screen.getByRole("combobox", { name: /type/i });
    fireEvent.mouseDown(typeSelect);
    fireEvent.click(screen.getByRole("option", { name: /staging/i }));
    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "Some description" },
    });
    expect(createButton()).toBeDisabled();
  });

  it("enables Create and fires onCreate with the correct payload", () => {
    const { onCreate } = renderDialog();

    fireEvent.change(screen.getByLabelText(/name/i), {
      target: { value: "  QA env  " },
    });

    const typeSelect = screen.getByRole("combobox", { name: /type/i });
    fireEvent.mouseDown(typeSelect);
    fireEvent.click(screen.getByRole("option", { name: /^qa$/i }));

    fireEvent.change(screen.getByLabelText(/description/i), {
      target: { value: "QA environment" },
    });

    fireEvent.click(createButton());
    expect(onCreate).toHaveBeenCalledWith({
      projectId: PROJECT_ID,
      name: "QA env",
      type: "qa",
      description: "QA environment",
    });
  });

  it("renders a type selector with all 6 deployment types", () => {
    renderDialog();
    const typeSelect = screen.getByRole("combobox", { name: /type/i });
    fireEvent.mouseDown(typeSelect);
    expect(screen.getByRole("option", { name: /primary production/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /staging/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /^qa$/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /stress/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /uat/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /development/i })).toBeInTheDocument();
  });

  it("calls onClose when Cancel is clicked", () => {
    const { onClose } = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
