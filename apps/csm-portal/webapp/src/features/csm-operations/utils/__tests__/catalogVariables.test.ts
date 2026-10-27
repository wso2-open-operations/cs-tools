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
  getDateTimeWindowError,
  getFirstEmptyRequiredField,
  getFirstFieldExceedingMaxLength,
  getFirstFieldFailingValidation,
  getUserEditableVariables,
  isEndDateTimeField,
  isStartDateTimeField,
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

describe("isStartDateTimeField / isEndDateTimeField", () => {
  it("matches a field named exactly Start Date or Start Time", () => {
    expect(isStartDateTimeField(textVariable({ questionText: "Start Date" }))).toBe(true);
    expect(isStartDateTimeField(textVariable({ questionText: "Start Time" }))).toBe(true);
    expect(isStartDateTimeField(textVariable({ questionText: "* Start Date" }))).toBe(true);
  });

  it("matches a field named exactly End Date or End Time", () => {
    expect(isEndDateTimeField(textVariable({ questionText: "End Date" }))).toBe(true);
    expect(isEndDateTimeField(textVariable({ questionText: "End Time" }))).toBe(true);
  });

  it("does not match a broader date/time field that isn't exactly Start/End", () => {
    expect(isStartDateTimeField(textVariable({ questionText: "Scheduled Date" }))).toBe(false);
    expect(isEndDateTimeField(textVariable({ questionText: "Planned End" }))).toBe(false);
  });

  it("recognizes a Required: label with no leading bullet/asterisk marker", () => {
    expect(isStartDateTimeField(textVariable({ questionText: "Required: Start Date" }))).toBe(
      true,
    );
    expect(isEndDateTimeField(textVariable({ questionText: "Required: End Date" }))).toBe(true);
  });
});

describe("getDateTimeWindowError", () => {
  function startField(): BeCatalogItemVariable {
    return textVariable({ id: "start", questionText: "Start Date", order: 1 });
  }
  function endField(): BeCatalogItemVariable {
    return textVariable({ id: "end", questionText: "End Date", order: 2 });
  }

  it("reports an error when the end time is before the start time", () => {
    const vars = [startField(), endField()];
    const result = getDateTimeWindowError(vars, {
      start: "2026-01-02T10:00",
      end: "2026-01-01T10:00",
    });
    expect(result).toBe("Start time must be before end time.");
  });

  it("reports an error when the end time equals the start time", () => {
    const vars = [startField(), endField()];
    const result = getDateTimeWindowError(vars, {
      start: "2026-01-01T10:00",
      end: "2026-01-01T10:00",
    });
    expect(result).toBe("Start time must be before end time.");
  });

  it("passes when the end time is after the start time", () => {
    const vars = [startField(), endField()];
    expect(
      getDateTimeWindowError(vars, { start: "2026-01-01T10:00", end: "2026-01-01T11:00" }),
    ).toBeNull();
  });

  it("does not fire while either field is still empty", () => {
    const vars = [startField(), endField()];
    expect(getDateTimeWindowError(vars, { start: "", end: "2026-01-01T11:00" })).toBeNull();
    expect(getDateTimeWindowError(vars, { start: "2026-01-01T10:00", end: "" })).toBeNull();
  });

  it("is a no-op when the catalog item has no Start/End Date pair at all", () => {
    const vars = [textVariable({ id: "v1", questionText: "Subject" })];
    expect(getDateTimeWindowError(vars, { v1: "anything" })).toBeNull();
  });

  it("validates a second Start/End pair even when the first pair is valid", () => {
    const vars = [
      textVariable({ id: "startDate", questionText: "Start Date", order: 1 }),
      textVariable({ id: "endDate", questionText: "End Date", order: 2 }),
      textVariable({ id: "startTime", questionText: "Start Time", order: 3 }),
      textVariable({ id: "endTime", questionText: "End Time", order: 4 }),
    ];
    // The Date pair is a valid window; the Time pair is not — neither should
    // be able to hide the other.
    const result = getDateTimeWindowError(vars, {
      startDate: "2026-01-01T10:00",
      endDate: "2026-01-02T10:00",
      startTime: "2026-01-01T15:00",
      endTime: "2026-01-01T10:00",
    });
    expect(result).toBe("Start time must be before end time.");
  });

  it("pairs Start Date with End Date and Start Time with End Time independently, not crossed", () => {
    const vars = [
      textVariable({ id: "startDate", questionText: "Start Date", order: 1 }),
      textVariable({ id: "endDate", questionText: "End Date", order: 2 }),
      textVariable({ id: "startTime", questionText: "Start Time", order: 3 }),
      textVariable({ id: "endTime", questionText: "End Time", order: 4 }),
    ];
    // Both pairs are individually valid windows.
    const result = getDateTimeWindowError(vars, {
      startDate: "2026-01-01T00:00",
      endDate: "2026-01-02T00:00",
      startTime: "2026-01-01T09:00",
      endTime: "2026-01-01T17:00",
    });
    expect(result).toBeNull();
  });
});
