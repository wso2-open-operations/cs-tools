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
import {
  compositeOver,
  contrastRatio,
  NEAR_BLACK,
  NEAR_BLACK_ALPHA,
  parseColor,
  pickAccessibleText,
  WHITE,
} from "./contrastText";

describe("parseColor", () => {
  it("parses 6-digit and 3-digit hex", () => {
    expect(parseColor("#ff0000")).toEqual({ r: 255, g: 0, b: 0 });
    expect(parseColor("#0f0")).toEqual({ r: 0, g: 255, b: 0 });
  });
  it("parses rgb()/rgba()", () => {
    expect(parseColor("rgb(2, 136, 209)")).toEqual({ r: 2, g: 136, b: 209 });
    expect(parseColor("rgba(0,0,0,0.5)")).toEqual({ r: 0, g: 0, b: 0 });
  });
  it("returns null for garbage", () => {
    expect(parseColor("not-a-colour")).toBeNull();
    expect(parseColor("")).toBeNull();
  });
});

describe("contrastRatio", () => {
  it("is 21 for black vs white", () => {
    expect(
      contrastRatio({ r: 0, g: 0, b: 0 }, { r: 255, g: 255, b: 255 }),
    ).toBeCloseTo(21, 0);
  });
});

describe("pickAccessibleText", () => {
  it("falls back to white for an unparseable background", () => {
    expect(pickAccessibleText("garbage")).toBe(WHITE);
  });

  // The contract that protects every SeverityChip: whatever text colour we pick
  // for a saturated semantic surface must itself clear WCAG AA (4.5:1) for small
  // text. These are the real `*.main` values shipped by the Oxygen themes (both
  // colour modes) plus the MUI defaults seen on staging — if any future theme
  // tweak drops one below 4.5:1 with both black and white, this test fails.
  const SEMANTIC_MAINS: Record<string, string> = {
    "acrylic-dark error": "#ef4444",
    "acrylic-dark warning": "#f59e0b",
    "acrylic-dark info": "#5567d5",
    "acrylic-dark success": "#22c55e",
    "acrylic-light error": "#f87171",
    "acrylic-light warning": "#fbbf24",
    "acrylic-light info": "#6b7de0",
    "acrylic-light success": "#4ade80",
    "mui-default error": "#f44336",
    "mui-default warning": "#ed6c02",
    "mui-default info": "#0288d1",
    "mui-default success": "#2e7d32",
  };

  for (const [name, main] of Object.entries(SEMANTIC_MAINS)) {
    it(`picks a text colour that passes AA on ${name} (${main})`, () => {
      const bg = parseColor(main)!;
      const text = pickAccessibleText(main);
      // Evaluate the colour as actually rendered: NEAR_BLACK is translucent, so
      // composite it over the fill rather than treating it as opaque black.
      const textRgb =
        text === NEAR_BLACK
          ? compositeOver({ r: 0, g: 0, b: 0 }, NEAR_BLACK_ALPHA, bg)
          : { r: 255, g: 255, b: 255 };
      const ratio = contrastRatio(bg, textRgb);
      expect(ratio).toBeGreaterThanOrEqual(4.5);
    });
  }
});
