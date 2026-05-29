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
import { describe, expect, it, vi, beforeEach } from "vitest";
import {
  useIsMidSizeTouchViewport,
  useIsStackedHeaderLayout,
} from "@hooks/useResponsiveLayout";

const mockUseMediaQuery = vi.fn();

vi.mock("@wso2/oxygen-ui", async (importOriginal) => {
  const actual = (await importOriginal()) as object;
  return {
    ...actual,
    useTheme: () => ({
      breakpoints: {
        between: (start: string, end: string) => `(between:${start}-${end})`,
        down: (key: string) => `(down:${key})`,
      },
    }),
    useMediaQuery: (query: string) => mockUseMediaQuery(query),
  };
});

describe("useIsMidSizeTouchViewport", () => {
  beforeEach(() => {
    mockUseMediaQuery.mockReset();
  });

  it("should return true for sm–md width with coarse pointer", () => {
    mockUseMediaQuery.mockImplementation((query: string) => {
      if (query.includes("between:sm-md")) return true;
      if (query === "(pointer: coarse)") return true;
      return false;
    });

    const { result } = renderHook(() => useIsMidSizeTouchViewport());
    expect(result.current).toBe(true);
  });

  it("should return false for mid-size width with fine pointer", () => {
    mockUseMediaQuery.mockImplementation((query: string) => {
      if (query.includes("between:sm-md")) return true;
      if (query === "(pointer: coarse)") return false;
      return false;
    });

    const { result } = renderHook(() => useIsMidSizeTouchViewport());
    expect(result.current).toBe(false);
  });

  it("should return false for coarse pointer outside sm–md band", () => {
    mockUseMediaQuery.mockImplementation((query: string) => {
      if (query.includes("between:sm-md")) return false;
      if (query === "(pointer: coarse)") return true;
      return false;
    });

    const { result } = renderHook(() => useIsMidSizeTouchViewport());
    expect(result.current).toBe(false);
  });
});

describe("useIsStackedHeaderLayout", () => {
  beforeEach(() => {
    mockUseMediaQuery.mockReset();
  });

  it("should return true for viewports below lg", () => {
    mockUseMediaQuery.mockImplementation((query: string) =>
      query.includes("down:lg"),
    );

    const { result } = renderHook(() => useIsStackedHeaderLayout());
    expect(result.current).toBe(true);
  });

  it("should return true for phone-sized viewports below sm", () => {
    mockUseMediaQuery.mockImplementation((query: string) => {
      if (query.includes("down:lg")) return true;
      if (query.includes("between:sm-md")) return false;
      return false;
    });

    const { result } = renderHook(() => useIsStackedHeaderLayout());
    expect(result.current).toBe(true);
  });

  it("should return false at lg laptop width and above", () => {
    mockUseMediaQuery.mockImplementation(() => false);

    const { result } = renderHook(() => useIsStackedHeaderLayout());
    expect(result.current).toBe(false);
  });
});
