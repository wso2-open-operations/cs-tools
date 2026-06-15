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
import {
  deleteFilterView,
  saveFilterView,
  useSavedFilterViews,
} from "./savedFilterViews";

beforeEach(() => {
  localStorage.clear();
});

describe("savedFilterViews", () => {
  it("starts empty", () => {
    const { result } = renderHook(() => useSavedFilterViews());
    expect(result.current).toEqual([]);
  });

  it("saves a view and exposes it to readers in the same tab", () => {
    const reader = renderHook(() => useSavedFilterViews());
    act(() => {
      saveFilterView("My S1/S2", "severities=S1,S2");
    });
    expect(reader.result.current).toHaveLength(1);
    expect(reader.result.current[0]).toEqual({
      name: "My S1/S2",
      qs: "severities=S1,S2",
    });
  });

  it("trims the name and ignores an empty one", () => {
    const reader = renderHook(() => useSavedFilterViews());
    act(() => {
      expect(saveFilterView("  Spaced  ", "states=open")).toBe("Spaced");
      expect(saveFilterView("   ", "states=open")).toBe("");
    });
    expect(reader.result.current.map((v) => v.name)).toEqual(["Spaced"]);
  });

  it("overwrites a view with the same name (case-insensitive), newest first", () => {
    const reader = renderHook(() => useSavedFilterViews());
    act(() => saveFilterView("Triage", "states=open"));
    act(() => saveFilterView("Other", "sla=breached"));
    act(() => saveFilterView("triage", "states=work_in_progress"));

    // "triage" replaces "Triage" and moves to the front; no duplicate.
    expect(reader.result.current).toEqual([
      { name: "triage", qs: "states=work_in_progress" },
      { name: "Other", qs: "sla=breached" },
    ]);
  });

  it("persists across a fresh hook mount (reload)", () => {
    act(() => saveFilterView("Persisted", "states=reopened"));
    const remount = renderHook(() => useSavedFilterViews());
    expect(remount.result.current).toEqual([
      { name: "Persisted", qs: "states=reopened" },
    ]);
  });

  it("deletes a view by name (case-insensitive)", () => {
    const reader = renderHook(() => useSavedFilterViews());
    act(() => saveFilterView("Keep", "states=open"));
    act(() => saveFilterView("Drop", "states=closed"));
    act(() => deleteFilterView("DROP"));
    expect(reader.result.current.map((v) => v.name)).toEqual(["Keep"]);
  });
});
