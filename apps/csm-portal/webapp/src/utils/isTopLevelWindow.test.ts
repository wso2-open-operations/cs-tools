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

import { afterEach, describe, expect, it, vi } from "vitest";
import { isTopLevelWindow } from "./isTopLevelWindow";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("isTopLevelWindow", () => {
  it("is true for the top-level page", () => {
    expect(isTopLevelWindow()).toBe(true);
  });

  it("is false inside a frame", () => {
    vi.spyOn(window, "top", "get").mockReturnValue({} as Window);
    expect(isTopLevelWindow()).toBe(false);
  });

  it("treats a cross-origin parent that throws on access as framed", () => {
    vi.spyOn(window, "top", "get").mockImplementation(() => {
      throw new Error("blocked a frame with origin ... from accessing a cross-origin frame");
    });
    expect(isTopLevelWindow()).toBe(false);
  });
});
