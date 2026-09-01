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

import { describe, expect, it } from "vitest";
import {
  getFirstEmptyRequiredField,
  getFirstFieldExceedingMaxLength,
  getFirstFieldFailingValidation,
  getUserEditableVariables,
  isVariableRequired,
} from "@features/csm-operations/utils/catalogVariables";
import type { BeCatalogItemVariable } from "@api/backend/types";

function textVariable(overrides: Partial<BeCatalogItemVariable> = {}): BeCatalogItemVariable {
  return {
    id: "v1",
    questionText: "Subject",
    order: 100,
    type: "Single Line Text",
    ...overrides,
  };
}

describe("isVariableRequired", () => {
  it("uses the real mandatory flag when the backend supplies one", () => {
    expect(isVariableRequired(textVariable({ mandatory: true }))).toBe(true);
    expect(isVariableRequired(textVariable({ mandatory: false }))).toBe(false);
  });

  it("falls back to the old hot fix (every typable field required) when mandatory is absent", () => {
    expect(isVariableRequired(textVariable())).toBe(true);
  });

  it("File Copy Path stays optional under the fallback, even though it's typable", () => {
    expect(
      isVariableRequired(textVariable({ questionText: "File Copy Path" })),
    ).toBe(false);
  });

  it("an attachment-type variable stays optional under the fallback", () => {
    expect(isVariableRequired(textVariable({ type: "Attachment" }))).toBe(false);
  });

  it("mandatory: false overrides even the File Copy Path exception's own default", () => {
    // Real backend data wins outright once supplied.
    expect(
      isVariableRequired(textVariable({ questionText: "File Copy Path", mandatory: true })),
    ).toBe(true);
  });
});

describe("getUserEditableVariables — active/hidden metadata", () => {
  it("excludes a variable the backend marks hidden: true", () => {
    const vars = [textVariable({ id: "v1" }), textVariable({ id: "v2", hidden: true })];
    expect(getUserEditableVariables(vars).map((v) => v.id)).toEqual(["v1"]);
  });

  it("excludes a variable the backend marks active: false", () => {
    const vars = [textVariable({ id: "v1" }), textVariable({ id: "v2", active: false })];
    expect(getUserEditableVariables(vars).map((v) => v.id)).toEqual(["v1"]);
  });

  it("keeps a variable with no active/hidden metadata at all (untagged data)", () => {
    const vars = [textVariable({ id: "v1" })];
    expect(getUserEditableVariables(vars).map((v) => v.id)).toEqual(["v1"]);
  });
});

describe("getFirstEmptyRequiredField", () => {
  it("reports the first empty field whose real mandatory flag is true", () => {
    const vars = [
      textVariable({ id: "v1", questionText: "Optional field", mandatory: false }),
      textVariable({ id: "v2", questionText: "Required field", mandatory: true }),
    ];
    expect(getFirstEmptyRequiredField(vars, {})).toBe("Required field");
  });

  it("does not flag a field the backend marks mandatory: false, even if empty", () => {
    const vars = [textVariable({ id: "v1", questionText: "Optional field", mandatory: false })];
    expect(getFirstEmptyRequiredField(vars, {})).toBeNull();
  });

  it("returns null once the mandatory field has a value", () => {
    const vars = [textVariable({ id: "v1", questionText: "Required field", mandatory: true })];
    expect(getFirstEmptyRequiredField(vars, { v1: "answered" })).toBeNull();
  });
});

describe("getFirstFieldExceedingMaxLength", () => {
  it("reports a field whose value is longer than its declared maxLength", () => {
    const vars = [textVariable({ id: "v1", questionText: "Subject", maxLength: 5 })];
    const result = getFirstFieldExceedingMaxLength(vars, { v1: "way too long" });
    expect(result).toEqual({ label: "Subject", maxLength: 5 });
  });

  it("passes when the value is within maxLength", () => {
    const vars = [textVariable({ id: "v1", maxLength: 80 })];
    expect(getFirstFieldExceedingMaxLength(vars, { v1: "short" })).toBeNull();
  });

  it("skips a variable with no declared maxLength", () => {
    const vars = [textVariable({ id: "v1", maxLength: null })];
    expect(getFirstFieldExceedingMaxLength(vars, { v1: "x".repeat(500) })).toBeNull();
  });
});

describe("getFirstFieldFailingValidation", () => {
  const emailValidation = {
    name: "Email",
    regex: "^[^@]+@[^@]+\\.[^@]+$",
    message: "Not a valid email",
  };

  it("reports a field whose value fails its declared validation regex", () => {
    const vars = [
      textVariable({ id: "v1", questionText: "Email Address", validation: emailValidation }),
    ];
    const result = getFirstFieldFailingValidation(vars, { v1: "not-an-email" });
    expect(result).toEqual({ label: "Email Address", message: "Not a valid email" });
  });

  it("passes a value matching the pattern", () => {
    const vars = [textVariable({ id: "v1", validation: emailValidation })];
    expect(getFirstFieldFailingValidation(vars, { v1: "jane.doe@example.com" })).toBeNull();
  });

  it("skips an empty value — that's getFirstEmptyRequiredField's job, not this one's", () => {
    const vars = [textVariable({ id: "v1", validation: emailValidation, mandatory: true })];
    expect(getFirstFieldFailingValidation(vars, { v1: "" })).toBeNull();
  });

  it("does not throw on a malformed backend regex, and does not block the field", () => {
    const vars = [
      textVariable({
        id: "v1",
        validation: { name: "Broken", regex: "(unterminated", message: "n/a" },
      }),
    ];
    expect(() => getFirstFieldFailingValidation(vars, { v1: "anything" })).not.toThrow();
    expect(getFirstFieldFailingValidation(vars, { v1: "anything" })).toBeNull();
  });
});
