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
import { formatClosureState } from "@features/project-hub/utils/closureState";

describe("formatClosureState", () => {
  it("capitalises each word of the raw status", () => {
    expect(formatClosureState("open")).toBe("Open");
    expect(formatClosureState("read_only")).toBe("Read Only");
    expect(formatClosureState("pending_notified")).toBe("Pending Notified");
    expect(formatClosureState("SUSPENDED")).toBe("Suspended");
  });

  it("calls a project with no status Active", () => {
    expect(formatClosureState(null)).toBe("Active");
    expect(formatClosureState(undefined)).toBe("Active");
    expect(formatClosureState("")).toBe("Active");
  });
});
