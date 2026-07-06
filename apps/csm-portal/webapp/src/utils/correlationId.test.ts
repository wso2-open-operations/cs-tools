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
import {
  CORRELATION_ID_HEADER,
  getErrorReferenceId,
  newCorrelationId,
} from "./correlationId";

const UUID_V4 =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("newCorrelationId", () => {
  it("returns a UUID v4", () => {
    expect(newCorrelationId()).toMatch(UUID_V4);
  });

  it("returns a distinct value each call", () => {
    const ids = new Set(Array.from({ length: 100 }, () => newCorrelationId()));
    expect(ids.size).toBe(100);
  });
});

describe("newCorrelationId fallbacks", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("uses getRandomValues when randomUUID is unavailable", () => {
    let calls = 0;
    vi.stubGlobal("crypto", {
      getRandomValues: (arr: Uint8Array) => {
        // Deterministic, varied fill; the generator sets the version/variant
        // bits itself, so any byte values still yield a well-formed UUID v4.
        for (let i = 0; i < arr.length; i += 1) arr[i] = (i * 37 + calls) & 0xff;
        calls += 1;
        return arr;
      },
    });
    expect(newCorrelationId()).toMatch(UUID_V4);
    // Distinct seeds across calls produce distinct IDs.
    expect(newCorrelationId()).not.toBe(newCorrelationId());
  });

  it("falls back to a non-crypto UUID when Web Crypto is unavailable", () => {
    vi.stubGlobal("crypto", undefined);
    expect(newCorrelationId()).toMatch(UUID_V4);
  });
});

describe("CORRELATION_ID_HEADER", () => {
  it("matches the backend header name", () => {
    expect(CORRELATION_ID_HEADER).toBe("X-CSM-Correlation-ID");
  });
});

describe("getErrorReferenceId", () => {
  it("extracts a correlationId from an error-like object", () => {
    expect(getErrorReferenceId({ correlationId: "abc-123" })).toBe("abc-123");
  });

  it("returns undefined for an empty correlationId", () => {
    expect(getErrorReferenceId({ correlationId: "" })).toBeUndefined();
  });

  it("returns undefined when the field is missing or wrong type", () => {
    expect(getErrorReferenceId(new Error("boom"))).toBeUndefined();
    expect(getErrorReferenceId({ correlationId: 42 })).toBeUndefined();
    expect(getErrorReferenceId(null)).toBeUndefined();
    expect(getErrorReferenceId(undefined)).toBeUndefined();
    expect(getErrorReferenceId("string")).toBeUndefined();
  });
});
