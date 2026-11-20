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

import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useOpenIdempotencyKey } from "@features/csm-cases/hooks/useOpenIdempotencyKey";

describe("useOpenIdempotencyKey", () => {
  it("returns the same key for an identical payload within one open", () => {
    const { result } = renderHook(({ open }) => useOpenIdempotencyKey(open), {
      initialProps: { open: true },
    });
    const first = result.current({ title: "a" });
    expect(first.length).toBeGreaterThan(0);
    expect(result.current({ title: "a" })).toBe(first);
  });

  it("mints a new key when the payload was edited, and again when edited back", () => {
    const { result } = renderHook(({ open }) => useOpenIdempotencyKey(open), {
      initialProps: { open: true },
    });
    const a = result.current({ title: "a" });
    const b = result.current({ title: "b" });
    expect(b).not.toBe(a);
    // Only the last payload is remembered, so going back gets a third key.
    const a2 = result.current({ title: "a" });
    expect(a2).not.toBe(a);
    expect(a2).not.toBe(b);
  });

  it("returns a new key after the dialog is closed and reopened", () => {
    const { result, rerender } = renderHook(({ open }) => useOpenIdempotencyKey(open), {
      initialProps: { open: true },
    });
    const first = result.current({ title: "a" });
    rerender({ open: false });
    rerender({ open: true });
    expect(result.current({ title: "a" })).not.toBe(first);
  });
});
