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
import RequestUpdateDialog from "@features/csm-cases/components/RequestUpdateDialog";
import { useGetCaseUpdateRequestTemplates } from "@features/csm-cases/api/useRequestCaseUpdate";
import type { BeCaseUpdateRequestTemplates } from "@api/backend/types";

vi.mock("@features/csm-cases/api/useRequestCaseUpdate", () => ({
  useGetCaseUpdateRequestTemplates: vi.fn(),
}));

const mockedUseGetTemplates = vi.mocked(useGetCaseUpdateRequestTemplates);

const TEMPLATES: BeCaseUpdateRequestTemplates = {
  generic: {
    first: "<p>Hi Team,</p><p>Generic first reminder.</p>",
    second: "<p>Hi Team,</p><p>Generic second reminder.</p>",
    final: "<p>Hi Team,</p><p>Generic final notice.</p>",
  },
  migration: {
    first: "<p>Hi Team,</p><p>Migration first reminder.</p>",
    second: "<p>Hi Team,</p><p>Migration second reminder.</p>",
    final: "<p>Hi Team,</p><p>Migration final notice.</p>",
  },
};

function mockTemplatesLoaded(
  templates: BeCaseUpdateRequestTemplates = TEMPLATES,
): void {
  mockedUseGetTemplates.mockReturnValue({
    data: templates,
    isLoading: false,
    isError: false,
  } as ReturnType<typeof useGetCaseUpdateRequestTemplates>);
}

function selectStage(label: RegExp): void {
  fireEvent.mouseDown(screen.getByRole("combobox", { name: /message/i }));
  fireEvent.click(within(screen.getByRole("listbox")).getByText(label));
}

describe("RequestUpdateDialog", () => {
  it("defaults to the first reminder and previews the generic template", () => {
    mockTemplatesLoaded();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByTestId("request-update-preview").innerHTML).toContain(
      "Generic first reminder.",
    );
  });

  it("previews the migration template set when category is migration", () => {
    mockTemplatesLoaded();
    render(
      <RequestUpdateDialog
        category="migration"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByTestId("request-update-preview").innerHTML).toContain(
      "Migration first reminder.",
    );
  });

  it("switches the preview when a different stage is selected", () => {
    mockTemplatesLoaded();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    selectStage(/second reminder/i);
    expect(screen.getByTestId("request-update-preview").innerHTML).toContain(
      "Generic second reminder.",
    );
  });

  it("calls onSave with just the stage for a fixed reminder", () => {
    mockTemplatesLoaded();
    const onSave = vi.fn();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^send$/i }));
    expect(onSave).toHaveBeenCalledWith({ stage: "first" });
  });

  it("shows a text field instead of a preview for a custom message, and blocks Send until non-empty", () => {
    mockTemplatesLoaded();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    selectStage(/custom message/i);
    expect(screen.queryByTestId("request-update-preview")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^send$/i })).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/custom message/i), {
      target: { value: "Any update on your side?" },
    });
    expect(screen.getByRole("button", { name: /^send$/i })).toBeEnabled();
  });

  it("wraps the custom message in an escaped paragraph before saving", () => {
    mockTemplatesLoaded();
    const onSave = vi.fn();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    selectStage(/custom message/i);
    fireEvent.change(screen.getByLabelText(/custom message/i), {
      target: { value: "Any update <on> your side?" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^send$/i }));
    expect(onSave).toHaveBeenCalledWith({
      stage: "custom",
      customContent: "<p>Any update &lt;on&gt; your side?</p>",
    });
  });

  it("disables Send while templates are loading", () => {
    mockedUseGetTemplates.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    } as ReturnType<typeof useGetCaseUpdateRequestTemplates>);
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /^send$/i })).toBeDisabled();
  });

  it("shows an error state when templates fail to load, but still allows a custom message", () => {
    mockedUseGetTemplates.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    } as ReturnType<typeof useGetCaseUpdateRequestTemplates>);
    const onSave = vi.fn();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={() => {}}
        onSave={onSave}
      />,
    );
    expect(screen.getByText(/couldn't load the reminder message templates/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^send$/i })).toBeDisabled();

    selectStage(/custom message/i);
    fireEvent.change(screen.getByLabelText(/custom message/i), {
      target: { value: "Checking in." },
    });
    fireEvent.click(screen.getByRole("button", { name: /^send$/i }));
    expect(onSave).toHaveBeenCalledWith({
      stage: "custom",
      customContent: "<p>Checking in.</p>",
    });
  });

  it("shows a loading state and disables actions while isSaving", () => {
    mockTemplatesLoaded();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving
        onClose={() => {}}
        onSave={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: /^send$/i })).toBeDisabled();
    expect(screen.getByRole("button", { name: /^close$/i })).toBeDisabled();
  });

  it("calls onClose on Close without calling onSave", () => {
    mockTemplatesLoaded();
    const onSave = vi.fn();
    const onClose = vi.fn();
    render(
      <RequestUpdateDialog
        category="generic"
        isSaving={false}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /^close$/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });
});
