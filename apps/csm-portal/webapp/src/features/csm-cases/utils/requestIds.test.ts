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
import { newRequestId, requestWasAnswered } from "@features/csm-cases/utils/productWorkItems";

describe("newRequestId", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("falls back to getRandomValues when randomUUID is unavailable", () => {
    vi.stubGlobal("crypto", {
      getRandomValues: (a: Uint8Array) => a.fill(7),
    });
    expect(newRequestId()).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  });

  it("falls back when randomUUID throws", () => {
    vi.stubGlobal("crypto", {
      randomUUID: () => {
        throw new Error("insecure");
      },
      getRandomValues: (a: Uint8Array) => a.fill(1),
    });
    expect(newRequestId()).toMatch(/^[0-9a-f-]{36}$/);
  });
});

describe("requestWasAnswered", () => {
  it("is true only for an error carrying an HTTP status", () => {
    expect(requestWasAnswered(Object.assign(new Error("x"), { status: 409 }))).toBe(true);
    expect(requestWasAnswered(new Error("Failed to fetch"))).toBe(false);
  });
});
