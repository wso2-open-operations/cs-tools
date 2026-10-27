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
  getUserEditableVariables,
  isAttachmentField,
  isDescriptionField,
  isFileCopyPathField,
} from "@features/operations/utils/serviceRequestValidation";

describe("serviceRequestValidation", () => {
  it("detects description field", () => {
    expect(isDescriptionField("Description")).toBe(true);
    expect(isDescriptionField("* Summary")).toBe(false);
  });

  it("detects attachment and file copy path fields", () => {
    expect(
      isAttachmentField({
        id: "1",
        order: 1,
        questionText: "Upload vulnerability scan report",
        type: "Single Line Text",
      }),
    ).toBe(true);
    expect(
      isFileCopyPathField({
        id: "2",
        order: 2,
        questionText: "File Copy Path",
        type: "text",
      }),
    ).toBe(true);
  });

  it("filters user-editable variables", () => {
    const contextValues = {
      projectDisplay: "Proj",
      deploymentDisplay: "Dep",
      productDisplay: "Prod",
    };
    const editable = getUserEditableVariables(
      [
        { id: "1", questionText: "Project", type: "text" } as never,
        { id: "2", questionText: "Notes", type: "text" } as never,
      ],
      contextValues,
    );
    expect(editable).toHaveLength(1);
    expect(editable[0].questionText).toBe("Notes");
  });

  it("returns first empty required field label", () => {
    const contextValues = {
      projectDisplay: "Proj",
      deploymentDisplay: "Dep",
      productDisplay: "Prod",
    };
    const label = getFirstEmptyRequiredField(
      [{ id: "v1", questionText: "Required field", type: "text" } as never],
      contextValues,
      {},
    );
    expect(label).toBe("Required field");
  });

  describe("getDateTimeWindowError", () => {
    const contextValues = {
      projectDisplay: "Proj",
      deploymentDisplay: "Dep",
      productDisplay: "Prod",
    };
    const variables = [
      { id: "start", questionText: "Start Date", type: "Date/Time" } as never,
      { id: "end", questionText: "End Date", type: "Date/Time" } as never,
    ];

    it("reports an error when the end time is at or before the start time", () => {
      expect(
        getDateTimeWindowError(variables, contextValues, {
          start: "2026-01-02T10:00",
          end: "2026-01-01T10:00",
        }),
      ).toBe("Start time must be before end time.");
      expect(
        getDateTimeWindowError(variables, contextValues, {
          start: "2026-01-01T10:00",
          end: "2026-01-01T10:00",
        }),
      ).toBe("Start time must be before end time.");
    });

    it("passes when the end time is after the start time", () => {
      expect(
        getDateTimeWindowError(variables, contextValues, {
          start: "2026-01-01T10:00",
          end: "2026-01-01T11:00",
        }),
      ).toBeNull();
    });

    it("does not fire while either field is still empty", () => {
      expect(
        getDateTimeWindowError(variables, contextValues, {
          start: "",
          end: "2026-01-01T11:00",
        }),
      ).toBeNull();
    });

    it("is a no-op when the catalog item has no Start/End Date pair", () => {
      expect(
        getDateTimeWindowError(
          [{ id: "v1", questionText: "Notes", type: "text" } as never],
          contextValues,
          { v1: "anything" },
        ),
      ).toBeNull();
    });

    it("recognizes a Required: label with no leading bullet/asterisk marker", () => {
      const unmarkedVariables = [
        { id: "start", questionText: "Required: Start Date", type: "Date/Time" } as never,
        { id: "end", questionText: "Required: End Date", type: "Date/Time" } as never,
      ];
      expect(
        getDateTimeWindowError(unmarkedVariables, contextValues, {
          start: "2026-01-02T10:00",
          end: "2026-01-01T10:00",
        }),
      ).toBe("Start time must be before end time.");
    });

    it("validates every Start/End pair when a catalog item has more than one", () => {
      const twoPairs = [
        { id: "startDate", questionText: "Start Date", type: "Date" } as never,
        { id: "endDate", questionText: "End Date", type: "Date" } as never,
        { id: "startTime", questionText: "Start Time", type: "Time" } as never,
        { id: "endTime", questionText: "End Time", type: "Time" } as never,
      ];
      // The Date pair is valid; the Time pair is not — the second pair must
      // still be caught, and Start Time must pair with End Time, not with
      // End Date.
      expect(
        getDateTimeWindowError(twoPairs, contextValues, {
          startDate: "2026-01-01T10:00",
          endDate: "2026-01-02T10:00",
          startTime: "2026-01-01T15:00",
          endTime: "2026-01-01T10:00",
        }),
      ).toBe("Start time must be before end time.");
      expect(
        getDateTimeWindowError(twoPairs, contextValues, {
          startDate: "2026-01-01T00:00",
          endDate: "2026-01-02T00:00",
          startTime: "2026-01-01T09:00",
          endTime: "2026-01-01T17:00",
        }),
      ).toBeNull();
    });
  });
});
