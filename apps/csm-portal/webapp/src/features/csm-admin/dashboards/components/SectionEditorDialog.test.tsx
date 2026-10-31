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
import type { BeDashboardWidget } from "@api/backend/types";

// SectionEditorDialog mounts the widget editor (and through it the live
// widget tile) only when a widget is being edited, but its imports still
// load: stub the backend client and the signed-in user they reach for.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: vi.fn(), get: vi.fn() }),
}));
vi.mock("@config/apiConfig", () => ({
  apiConfig: { backendUrl: "https://example.test" },
}));
vi.mock("@context/current-user/CurrentUserContext", () => ({
  useCurrentUser: () => ({ user: { id: "user-1" }, isLoading: false, isError: false }),
}));

import SectionEditorDialog from "@features/csm-admin/dashboards/components/SectionEditorDialog";

const WIDGET: BeDashboardWidget = {
  widgetId: "w1",
  displayName: "My patches",
  resourceType: "case",
  shape: "count",
  gridWidth: 3,
  query: {},
};

describe("SectionEditorDialog — required fields", () => {
  it("marks the name, the heading and the widget list as required", () => {
    render(<SectionEditorDialog takenNames={[]} onClose={vi.fn()} onSave={vi.fn()} />);

    expect(screen.getByLabelText("Section name")).toBeRequired();
    expect(screen.getByLabelText("Section heading")).toBeRequired();
    const widgetsHeading = screen.getByText("Widgets", { selector: "h6" });
    expect(widgetsHeading.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
  });

  it("keeps Save disabled until all three are given", () => {
    const onSave = vi.fn();
    render(
      <SectionEditorDialog
        section={{ name: "", displayName: "", widgets: [WIDGET] }}
        takenNames={[]}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );
    const save = screen.getByRole("button", { name: /save/i });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Section name"), { target: { value: "my-work" } });
    expect(save).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Section heading"), { target: { value: "My Work" } });
    expect(save).toBeEnabled();

    fireEvent.click(save);
    expect(onSave).toHaveBeenCalledWith({ name: "my-work", displayName: "My Work", widgets: [WIDGET] });
  });
});
