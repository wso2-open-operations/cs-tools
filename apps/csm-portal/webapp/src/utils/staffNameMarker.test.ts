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
import { hasStaffNameMarker, stripStaffNameMarker, withoutStaffNameMarker } from "./staffNameMarker";

describe("hasStaffNameMarker", () => {
  it("finds either circled W, wherever it sits", () => {
    expect(hasStaffNameMarker("Jane Doe Ⓦ")).toBe(true);
    expect(hasStaffNameMarker("Jane ⓦ Doe")).toBe(true);
  });

  it("is false for a plain name or no name", () => {
    expect(hasStaffNameMarker("Jane Doe")).toBe(false);
    expect(hasStaffNameMarker("")).toBe(false);
    expect(hasStaffNameMarker(null)).toBe(false);
  });

  it("gives the same answer when called again (no regex state carried over)", () => {
    expect([hasStaffNameMarker("A Ⓦ"), hasStaffNameMarker("B Ⓦ")]).toEqual([true, true]);
  });
});

describe("stripStaffNameMarker", () => {
  it("removes the marker and tidies the spaces", () => {
    expect(stripStaffNameMarker("Jane Doe Ⓦ")).toBe("Jane Doe");
    expect(stripStaffNameMarker("Jane ⓦ Doe")).toBe("Jane Doe");
  });
});

describe("withoutStaffNameMarker", () => {
  it("cleans a marked name", () => {
    expect(withoutStaffNameMarker("Jane  Doe Ⓦ")).toBe("Jane Doe");
  });

  it("returns a name without the marker exactly as given", () => {
    expect(withoutStaffNameMarker("Jane  Doe")).toBe("Jane  Doe");
    expect(withoutStaffNameMarker("山田\u3000太郎")).toBe("山田\u3000太郎");
  });

  it("is empty for no name", () => {
    expect(withoutStaffNameMarker(undefined)).toBe("");
    expect(withoutStaffNameMarker(null)).toBe("");
  });
});
