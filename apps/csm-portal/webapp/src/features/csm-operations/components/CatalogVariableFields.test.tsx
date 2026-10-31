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
  default: ({ ariaRequired }: { ariaRequired?: boolean }) => (
    <div data-testid="rich-text-editor" aria-required={ariaRequired ? "true" : undefined} />
  ),
}));

import CatalogVariableFields from "@features/csm-operations/components/CatalogVariableFields";
import { getFirstEmptyRequiredField } from "@features/csm-operations/utils/catalogVariables";

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

describe("CatalogVariableFields — SR variable metadata (mandatory/readOnly/maxLength/validation)", () => {
  it("marks a plain text field required only when the real mandatory flag says so", () => {
    renderFields([
      { ...NOTES, id: "req", questionText: "Required text", mandatory: true },
      { ...NOTES, id: "opt", questionText: "Optional text", mandatory: false },
    ]);

    expect(screen.getByLabelText(/required text/i)).toHaveAttribute("required");
    expect(screen.getByLabelText(/optional text/i)).not.toHaveAttribute("required");
  });

  it("disables a readOnly field's input, still showing its value", () => {
    renderFields(
      [{ ...NOTES, id: "ro", questionText: "Read-only field", readOnly: true }],
      { ro: "fixed value" },
    );

    const field = screen.getByLabelText(/read-only field/i);
    expect(field).toBeDisabled();
    expect(field).toHaveValue("fixed value");
  });

  it("constrains a text input's maxLength attribute from the declared metadata", () => {
    renderFields([
      {
        id: "33333333-3333-3333-3333-333333333333",
        questionText: "Short field",
        order: 100,
        type: "single_line_text",
        maxLength: 10,
      },
    ]);

    expect(screen.getByLabelText(/short field/i)).toHaveAttribute("maxlength", "10");
  });

  it("shows the declared validation message as an error when the current value fails the regex", () => {
    renderFields(
      [
        {
          id: "44444444-4444-4444-4444-444444444444",
          questionText: "Email Address",
          order: 100,
          type: "single_line_text",
          validation: {
            name: "Email",
            regex: "^[^@]+@[^@]+\\.[^@]+$",
            message: "Not a valid email",
          },
        },
      ],
      { "44444444-4444-4444-4444-444444444444": "not-an-email" },
    );

    expect(screen.getByText("Not a valid email")).toBeInTheDocument();
  });

  it("shows no validation error once the value matches the pattern", () => {
    renderFields(
      [
        {
          id: "44444444-4444-4444-4444-444444444444",
          questionText: "Email Address",
          order: 100,
          type: "single_line_text",
          validation: {
            name: "Email",
            regex: "^[^@]+@[^@]+\\.[^@]+$",
            message: "Not a valid email",
          },
        },
      ],
      { "44444444-4444-4444-4444-444444444444": "jane.doe@example.com" },
    );

    expect(screen.queryByText("Not a valid email")).not.toBeInTheDocument();
  });
});

describe("CatalogVariableFields — rich-text Description", () => {
  const DESCRIPTION: BeCatalogItemVariable = {
    id: "66666666-6666-6666-6666-666666666666",
    questionText: "Description",
    order: 100,
    type: "multi_line_text",
  };

  it("puts the required asterisk on its caption and aria-required on the editor when mandatory", () => {
    renderFields([{ ...DESCRIPTION, mandatory: true }]);

    const caption = screen.getByText("Description");
    expect(caption.querySelector(".MuiFormLabel-asterisk")).not.toBeNull();
    expect(screen.getByTestId("rich-text-editor")).toHaveAttribute("aria-required", "true");
  });

  it("shows neither when the variable is optional", () => {
    renderFields([{ ...DESCRIPTION, mandatory: false }]);

    const caption = screen.getByText("Description");
    expect(caption.querySelector(".MuiFormLabel-asterisk")).toBeNull();
    expect(caption.textContent).not.toContain("*");
    expect(screen.getByTestId("rich-text-editor")).not.toHaveAttribute("aria-required");
  });

  it("shows neither on a read-only variable, even a mandatory one, since its editor is disabled", () => {
    renderFields([{ ...DESCRIPTION, mandatory: true, readOnly: true }]);

    expect(screen.getByText("Description").querySelector(".MuiFormLabel-asterisk")).toBeNull();
    expect(screen.getByTestId("rich-text-editor")).not.toHaveAttribute("aria-required");
  });
});

describe("CatalogVariableFields — read-only variables", () => {
  it("does not mark a mandatory read-only field required, whatever its kind, since it is disabled", () => {
    renderFields([
      { ...NOTES, id: "ro-text", questionText: "Read-only notes", mandatory: true, readOnly: true },
      { ...TARGET_CLOUD, id: "ro-choice", questionText: "Read-only cloud", mandatory: true, readOnly: true },
      { ...NOTES, id: "rw-text", questionText: "Editable notes", mandatory: true },
    ]);

    const notes = screen.getByLabelText(/read-only notes/i);
    expect(notes).toBeDisabled();
    expect(notes).not.toBeRequired();
    const cloud = screen.getByRole("combobox", { name: /read-only cloud/i });
    expect(cloud).toHaveAttribute("aria-disabled", "true");
    expect(cloud).not.toBeRequired();
    // The same kind of variable, editable, is marked.
    expect(screen.getByLabelText(/editable notes/i)).toBeRequired();
  });

  it("agrees with the submit gate on an empty mandatory read-only field: no mark, no block", () => {
    const variables = [
      { ...NOTES, id: "ro-text", questionText: "Read-only notes", mandatory: true, readOnly: true },
      { ...NOTES, id: "rw-text", questionText: "Editable notes", mandatory: true },
    ];
    renderFields(variables);

    expect(screen.getByLabelText(/read-only notes/i)).not.toBeRequired();
    // The only field that blocks is the one that carries the mark.
    expect(getFirstEmptyRequiredField(variables, {})).toBe("Editable notes");
    expect(getFirstEmptyRequiredField(variables, { "rw-text": "filled" })).toBeNull();
  });
});
