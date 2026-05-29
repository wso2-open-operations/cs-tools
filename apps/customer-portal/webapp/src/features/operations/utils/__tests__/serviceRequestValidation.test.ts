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
});
