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
  clearRecentViews,
  useRecentViews,
  useRecordRecentView,
  type RecentView,
} from "./useRecentViews";

const entry = (id: string): Omit<RecentView, "visitedAt"> => ({
  kind: "case",
  id,
  title: `Case ${id}`,
  href: `/cases/${id}`,
});

beforeEach(() => {
  localStorage.clear();
});

describe("useRecentViews + useRecordRecentView", () => {
  it("starts empty", () => {
    const { result } = renderHook(() => useRecentViews());
    expect(result.current).toEqual([]);
  });

  it("records a visit and exposes it to readers in the same tab", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());

    act(() => recorder.result.current(entry("1")));

    expect(reader.result.current).toHaveLength(1);
    expect(reader.result.current[0].id).toBe("1");
    expect(reader.result.current[0].visitedAt).toBeTruthy();
  });

  it("de-dupes by kind+id and bumps the entry to the top", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());

    act(() => recorder.result.current(entry("1")));
    act(() => recorder.result.current(entry("2")));
    act(() => recorder.result.current(entry("1")));

    expect(reader.result.current.map((v) => v.id)).toEqual(["1", "2"]);
  });

  it("caps the list at 12 entries, keeping the most recent", () => {
    const recorder = renderHook(() => useRecordRecentView());
    for (let i = 0; i < 15; i++) {
      act(() => recorder.result.current(entry(String(i))));
    }
    const reader = renderHook(() => useRecentViews());
    expect(reader.result.current).toHaveLength(12);
    // Most recent first: 14 down to 3.
    expect(reader.result.current[0].id).toBe("14");
    expect(reader.result.current.at(-1)?.id).toBe("3");
  });

  it("clearRecentViews empties the list", () => {
    const reader = renderHook(() => useRecentViews());
    const recorder = renderHook(() => useRecordRecentView());
    act(() => recorder.result.current(entry("1")));
    expect(reader.result.current).toHaveLength(1);

    act(() => clearRecentViews());
    expect(reader.result.current).toEqual([]);
  });
});
