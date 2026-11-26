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
// The Lexical editor has its own tests; stub it down to a textarea that takes
// and emits HTML.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: ({
    value,
    onChange,
  }: {
    value: string;
    onChange: (v: string) => void;
  }) => (
    <textarea
      aria-label="Custom message editor"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
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

function typeCustomMessage(html: string): void {
  fireEvent.change(
    screen.getByRole("textbox", { name: /custom message editor/i }),
    { target: { value: html } },
  );
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

  it("shows a labelled rich-text editor instead of a preview for a custom message", () => {
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
    expect(
      within(screen.getByRole("group", { name: /^custom message$/i })).getByRole(
        "textbox",
        { name: /custom message editor/i },
      ),
    ).toBeInTheDocument();
  });

  it("blocks Send until the custom message has text", () => {
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
    const send = screen.getByRole("button", { name: /^send$/i });
    expect(send).toBeDisabled();
    for (const blank of [
      "<p><br></p>",
      "<p>&nbsp;</p>",
      "<p>   </p><ul><li></li></ul>",
      "<script>alert(1)</script>",
    ]) {
      typeCustomMessage(blank);
      expect(send).toBeDisabled();
    }

    typeCustomMessage("<p>Any update on your side?</p>");
    expect(send).toBeEnabled();
  });

  it("sends the custom message's formatting", () => {
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
    const html =
      "<p>Hi Team,</p>" +
      "<p><strong>Still needed</strong> to <em>reproduce</em> the error:</p>" +
      "<ol><li>The server log</li><li>The <code>deployment.toml</code> file</li></ol>" +
      "<ul><li>Product version</li><li>Update level</li></ul>" +
      '<pre spellcheck="false">sh bin/server.sh --debug</pre>' +
      '<p>See <a href="https://example.com/docs/logs">the guide</a>.</p>';
    typeCustomMessage(html);
    fireEvent.click(screen.getByRole("button", { name: /^send$/i }));
    expect(onSave).toHaveBeenCalledWith({ stage: "custom", customContent: html });
  });

  it("strips scripts, event handlers and javascript: links from the custom message", () => {
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
    typeCustomMessage(
      '<p onclick="alert(1)">Any <strong onmouseover="alert(1)">update</strong>?</p>' +
        "<script>alert(1)</script>" +
        '<ol><li><a href="javascript:alert(1)">Open</a></li></ol>',
    );
    fireEvent.click(screen.getByRole("button", { name: /^send$/i }));
    expect(onSave).toHaveBeenCalledWith({
      stage: "custom",
      customContent:
        "<p>Any <strong>update</strong>?</p><ol><li><a>Open</a></li></ol>",
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
    typeCustomMessage("<p>Checking in.</p>");
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
