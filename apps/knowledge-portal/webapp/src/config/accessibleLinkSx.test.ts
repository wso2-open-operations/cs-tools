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

import { describe, expect, it } from "vitest";
import { AcrylicOrangeTheme } from "@wso2/oxygen-ui";
import type { Theme } from "@mui/material/styles";
import { accessibleLinkSx } from "@config/a11yThemeOverrides";

const channel = (v: number): number => {
  const c = v / 255;
  return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
};
const luminance = ([r, g, b]: number[]): number => 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
const contrast = (a: number[], b: number[]): number => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};
const parse = (css: string): number[] =>
  css.startsWith("#")
    ? [1, 3, 5].map((i) => parseInt(css.slice(i, i + 2), 16))
    : (css.match(/\d+/g) ?? []).slice(0, 3).map(Number);

describe("accessibleLinkSx", () => {
  const schemes = (AcrylicOrangeTheme as unknown as { colorSchemes: Record<"light" | "dark", { palette: { primary: { main: string; dark: string } } }> }).colorSchemes;
  const fakeTheme = {
    palette: { primary: { main: "MAIN", dark: "DARK" } },
    applyStyles: (scheme: string, styles: object) => ({ [`[data-${scheme}]`]: styles }),
  } as unknown as Theme;

  it("uses the accent by default and the darker shade in the light scheme", () => {
    expect(accessibleLinkSx(fakeTheme)).toEqual({
      color: "MAIN",
      "[data-light]": { color: "DARK" },
    });
  });

  it("meets WCAG AA (4.5:1) on both page backgrounds in each scheme", () => {
    const light = parse(schemes.light.palette.primary.dark);
    expect(contrast(light, [255, 255, 255])).toBeGreaterThanOrEqual(4.5);
    expect(contrast(light, [245, 245, 245])).toBeGreaterThanOrEqual(4.5);
    const dark = parse(schemes.dark.palette.primary.main);
    expect(contrast(dark, [0, 0, 0])).toBeGreaterThanOrEqual(4.5);
    expect(contrast(dark, [18, 18, 18])).toBeGreaterThanOrEqual(4.5);
    // The accent the links used before fails on light, which is why light differs.
    expect(contrast(parse(schemes.light.palette.primary.main), [255, 255, 255])).toBeLessThan(4.5);
  });
});
