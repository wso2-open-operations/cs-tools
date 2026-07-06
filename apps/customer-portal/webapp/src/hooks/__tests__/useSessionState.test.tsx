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

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { MemoryRouter } from "react-router";
import type { ReactNode } from "react";
import { useSessionState } from "@hooks/useSessionState";

const wrapper =
  (initialEntries: string[] = ["/"]) =>
  ({ children }: { children: ReactNode }) =>
    <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>;

describe("useSessionState", () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it("restores stored value on mount", () => {
    sessionStorage.setItem("filters:test", JSON.stringify("saved"));
    const { result } = renderHook(() => useSessionState("filters:test", "default"), {
      wrapper: wrapper(),
    });
    expect(result.current[0]).toBe("saved");
  });

  it("persists updates to sessionStorage", () => {
    const { result } = renderHook(() => useSessionState("filters:write", "default"), {
      wrapper: wrapper(),
    });
    act(() => {
      result.current[1]("next");
    });
    expect(sessionStorage.getItem("filters:write")).toBe(JSON.stringify("next"));
  });

  it("uses default when stored value fails validation", () => {
    sessionStorage.setItem("filters:bad", JSON.stringify(99));
    const isString = (v: unknown): v is string => typeof v === "string";
    const { result } = renderHook(
      () => useSessionState("filters:bad", "default", isString),
      { wrapper: wrapper() },
    );
    expect(result.current[0]).toBe("default");
  });
});
