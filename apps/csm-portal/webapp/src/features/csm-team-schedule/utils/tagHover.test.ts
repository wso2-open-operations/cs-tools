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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { describe, expect, it } from "vitest";
import { tagHoverText, tagSideText } from "./tagHover";

describe("tagHoverText", () => {
  it("is just the name for a plain tag", () => {
    expect(tagHoverText({ label: "Work from home" })).toBe("Work from home");
  });

  it("adds the description and says when the tag takes people off the timesheet", () => {
    expect(
      tagHoverText({ label: "SRE rotation", description: " On the SRE rota ", excludesFromTimesheet: true }),
    ).toBe("SRE rotation — On the SRE rota — Not on the timesheet");
  });

  it("leaves out a blank description", () => {
    expect(tagHoverText({ label: "WFH", description: "  " })).toBe("WFH");
  });
});

describe("tagSideText", () => {
  it("is the name of a built-in tag, whose name is not its chip", () => {
    expect(tagSideText({ shortCode: "AL/CL", label: "Annual/Casual leave" })).toBe("Annual/Casual leave");
  });

  it("is the description of a tag whose name is its chip, and nothing without one", () => {
    expect(tagSideText({ shortCode: "SRE", label: "SRE", description: "On the SRE rota" })).toBe("On the SRE rota");
    expect(tagSideText({ shortCode: "WFH", label: "wfh" })).toBe("");
  });
});
