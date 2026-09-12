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
import type { BeCatalogItemVariable } from "@api/backend/types";

// The rich-text editor pulls in the whole Lexical stack for the Description
// field, which none of these assertions touch — stub it to a marker so the
// suite stays a fast, focused test of control selection.
vi.mock("@components/rich-text-editor/Editor", () => ({
  default: () => <div data-testid="rich-text-editor" />,
}));

import CatalogVariableFields from "@features/csm-operations/components/CatalogVariableFields";

/** Opens a MUI select by its accessible name and returns its option list. */
function openSelect(name: RegExp | string): HTMLElement {
  fireEvent.mouseDown(screen.getByRole("combobox", { name }));
  return screen.getByRole("listbox");
}

function renderFields(
  variables: BeCatalogItemVariable[],
  values: Record<string, string> = {},
): { onChange: ReturnType<typeof vi.fn> } {
  const onChange = vi.fn();
  render(
    <CatalogVariableFields
      variables={variables}
      values={values}
      onChange={onChange}
    />,
  );
  return { onChange };
}

const TARGET_CLOUD: BeCatalogItemVariable = {
  id: "11111111-1111-1111-1111-111111111111",
  questionText: "Target cloud",
  order: 100,
  type: "select",
  choices: [
    { value: "aws", text: "AWS", order: 100 },
    { value: "azure", text: "Azure", order: 200 },
  ],
};

const NOTES: BeCatalogItemVariable = {
  id: "22222222-2222-2222-2222-222222222222",
  questionText: "Notes",
  order: 200,
  type: "multi_line_text",
};

describe("CatalogVariableFields — choice lists", () => {
  it("renders a variable with choices as a select labelled by its question text", () => {
    renderFields([TARGET_CLOUD]);

    const select = screen.getByRole("combobox", { name: /target cloud/i });
    expect(select).toBeInTheDocument();
  });

  it("offers the supplied choices, using `text` as the visible label", () => {
    renderFields([TARGET_CLOUD]);

    const listbox = openSelect(/target cloud/i);
    expect(within(listbox).getByRole("option", { name: "AWS" })).toBeInTheDocument();
    expect(within(listbox).getByRole("option", { name: "Azure" })).toBeInTheDocument();
    expect(within(listbox).getAllByRole("option")).toHaveLength(2);
  });

  it("submits the choice's `value`, not its display `text`", () => {
    const { onChange } = renderFields([TARGET_CLOUD]);

    openSelect(/target cloud/i);
    fireEvent.click(screen.getByRole("option", { name: "AWS" }));

    expect(onChange).toHaveBeenCalledWith(TARGET_CLOUD.id, "aws");
  });

  it("falls back to `value` as the label when `text` is null", () => {
    renderFields([
      {
        ...TARGET_CLOUD,
        choices: [{ value: "aws", text: null, order: 100 }],
      },
    ]);

    const listbox = openSelect(/target cloud/i);
    expect(within(listbox).getByRole("option", { name: "aws" })).toBeInTheDocument();
  });

  it("skips a malformed choice whose `value` is null", () => {
    renderFields([
      {
        ...TARGET_CLOUD,
        choices: [
          { value: "aws", text: "AWS", order: 100 },
          { value: null, text: null, order: null },
        ],
      },
    ]);

    const listbox = openSelect(/target cloud/i);
    expect(within(listbox).getAllByRole("option")).toHaveLength(1);
    expect(within(listbox).getByRole("option", { name: "AWS" })).toBeInTheDocument();
  });

  it("preserves the order the backend returned the choices in", () => {
    renderFields([
      {
        ...TARGET_CLOUD,
        // Reversed relative to `order` on purpose: the array order wins.
        choices: [
          { value: "azure", text: "Azure", order: 200 },
          { value: "aws", text: "AWS", order: 100 },
        ],
      },
    ]);

    const listbox = openSelect(/target cloud/i);
    expect(
      within(listbox)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(["Azure", "AWS"]);
  });

  it("keeps the select keyboard-operable and reflects the current value", () => {
    renderFields([TARGET_CLOUD], { [TARGET_CLOUD.id]: "azure" });

    const select = screen.getByRole("combobox", { name: /target cloud/i });
    expect(select).toHaveTextContent("Azure");
    // MUI renders the trigger as a focusable, ARIA-labelled combobox.
    expect(select).toHaveAttribute("tabindex", "0");
  });

  it("falls back to a text input when every choice is unusable", () => {
    renderFields([
      {
        id: "33333333-3333-3333-3333-333333333333",
        questionText: "Ticket reference",
        order: 100,
        type: "single_line_text",
        choices: [{ value: null, text: "Broken", order: null }],
      },
    ]);

    expect(
      screen.queryByRole("combobox", { name: /ticket reference/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: /ticket reference/i }),
    ).toBeInTheDocument();
  });

  it("still renders a text control for a variable with no `choices` key at all", () => {
    renderFields([NOTES]);

    expect(
      screen.queryByRole("combobox", { name: /notes/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: /notes/i })).toBeInTheDocument();
  });

  it("keeps the Yes/No fallback for a choice-type variable the data source sent no list for", () => {
    renderFields([
      {
        id: "44444444-4444-4444-4444-444444444444",
        questionText: "Requires downtime",
        order: 100,
        type: "Select Box",
      },
    ]);

    const listbox = openSelect(/requires downtime/i);
    expect(
      within(listbox)
        .getAllByRole("option")
        .map((o) => o.textContent),
    ).toEqual(["Yes", "No"]);
  });

  it("leaves the required flag on a choice field unchanged", () => {
    renderFields([TARGET_CLOUD]);

    expect(screen.getByRole("combobox", { name: /target cloud/i })).toHaveAttribute(
      "aria-required",
      "true",
    );
  });

  it("keeps a File Copy Path field optional even when it carries choices", () => {
    renderFields([
      {
        id: "55555555-5555-5555-5555-555555555555",
        questionText: "File Copy Path",
        order: 100,
        type: "single_line_text",
        choices: [{ value: "/srv/data", text: "/srv/data", order: 100 }],
      },
    ]);

    const select = screen.getByRole("combobox", { name: /file copy path/i });
    expect(select).not.toHaveAttribute("aria-required", "true");
  });
});
