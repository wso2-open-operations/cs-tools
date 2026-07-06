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
import { getStatusColor, stripHtmlTags } from "@features/project-hub/utils/projectCard";

describe("getStatusColor", () => {
  it("maps healthy statuses to success", () => {
    expect(getStatusColor("All Good")).toBe("success");
    expect(getStatusColor("healthy")).toBe("success");
  });

  it("maps need attention to warning", () => {
    expect(getStatusColor("Need Attention")).toBe("warning");
  });

  it("maps critical statuses to error", () => {
    expect(getStatusColor("Critical Issues")).toBe("error");
  });

  it("returns default for unknown status", () => {
    expect(getStatusColor("unknown")).toBe("default");
  });
});

describe("stripHtmlTags", () => {
  it("removes HTML tags", () => {
    expect(stripHtmlTags("<p>Hello <b>world</b></p>")).toBe("Hello world");
  });

  it("returns empty string for falsy input", () => {
    expect(stripHtmlTags("")).toBe("");
  });
});
