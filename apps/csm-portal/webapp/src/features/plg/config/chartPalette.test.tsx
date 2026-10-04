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
import type { Theme } from "@mui/material/styles";
import { describe, expect, it } from "vitest";
import { chartSchemeVars, useChartColors } from "@features/plg/config/chartPalette";

describe("chartPalette", () => {
  it("sets the lightness per scheme through applyStyles, not a mode check", () => {
    const theme = {
      applyStyles: (scheme: string, styles: object) => ({ [`[data-scheme=${scheme}] &`]: styles }),
    } as unknown as Theme;
    expect(chartSchemeVars(theme)).toEqual({
      "--plg-chart-l": "48%",
      "[data-scheme=dark] &": { "--plg-chart-l": "60%" },
    });
  });

  it("builds twelve series that read their lightness from the scheme variable", () => {
    const { result } = renderHook(() => useChartColors());
    expect(result.current).toHaveLength(12);
    for (const c of result.current) {
      expect(c).toMatch(/^hsl\(\d+ \d+% var\(--plg-chart-l, 48%\)\)$/);
    }
  });
});
