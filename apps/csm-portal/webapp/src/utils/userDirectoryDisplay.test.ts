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
import { displayUserType } from "@utils/userDirectoryDisplay";

describe("displayUserType", () => {
  it("labels internal plainly", () => {
    expect(displayUserType("internal")).toBe("Internal");
  });

  it("labels both external-user spellings the same way", () => {
    expect(displayUserType("customer")).toBe("External (customer)");
    expect(displayUserType("external")).toBe("External (customer)");
  });

  it("labels system", () => {
    expect(displayUserType("system")).toBe("System");
  });

  it("falls back to an em dash for anything else", () => {
    expect(displayUserType(undefined)).toBe("—");
    expect(displayUserType(null)).toBe("—");
    expect(displayUserType("")).toBe("—");
    expect(displayUserType("bogus")).toBe("—");
  });
});
