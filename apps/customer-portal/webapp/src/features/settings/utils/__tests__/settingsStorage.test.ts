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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  getNoveraChatEnabled,
  setNoveraChatEnabled,
} from "@features/settings/utils/settingsStorage";

describe("settingsStorage", () => {

  beforeEach(() => {
    vi.stubGlobal(
      "localStorage",
      (() => {
        let store: Record<string, string> = {};
        return {
          getItem: (key: string) => store[key] ?? null,
          setItem: (key: string, value: string) => {
            store[key] = value;
          },
          removeItem: (key: string) => {
            delete store[key];
          },
          clear: () => {
            store = {};
          },
          get length() {
            return Object.keys(store).length;
          },
          key: (i: number) => Object.keys(store)[i] ?? null,
        };
      })(),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns true when nothing is set (default)", () => {
    expect(getNoveraChatEnabled()).toBe(true);
  });

  it("returns true when stored as 'true'", () => {
    setNoveraChatEnabled(true);
    expect(getNoveraChatEnabled()).toBe(true);
  });

  it("returns false when stored as 'false'", () => {
    setNoveraChatEnabled(false);
    expect(getNoveraChatEnabled()).toBe(false);
  });

  it("persists across get/set calls", () => {
    setNoveraChatEnabled(false);
    expect(getNoveraChatEnabled()).toBe(false);
    setNoveraChatEnabled(true);
    expect(getNoveraChatEnabled()).toBe(true);
  });
});
