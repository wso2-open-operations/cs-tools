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

import type { Locator, Page } from "@playwright/test";

/** WCAG relative luminance of an sRGB colour (0-255 channels). */
function luminance([r, g, b]: number[]): number {
  const [lr, lg, lb] = [r, g, b].map((v) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * lr + 0.7152 * lg + 0.0722 * lb;
}

/** WCAG contrast ratio of two sRGB colours. */
export function contrastRatio(a: number[], b: number[]): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

/** The first three numbers of a CSS `rgb(...)` / `rgba(...)` colour. */
export function parseCssColor(css: string): number[] {
  const numbers = css.match(/[\d.]+/g);
  if (!numbers || numbers.length < 3) throw new Error(`not an rgb colour: ${css}`);
  return numbers.slice(0, 3).map(Number);
}

/**
 * The colours actually painted behind an element's text, sampled from a screenshot of
 * its box (left and right padding, top and bottom edge). A computed `background-color`
 * says nothing for an outlined button on the page's gradient, and "worst of the samples"
 * is the honest figure for a gradient.
 *
 * @param page - The page the element is on.
 * @param element - The element (it must be visible).
 * @returns The sampled `[r, g, b]` colours.
 */
export async function paintedBackground(page: Page, element: Locator): Promise<number[][]> {
  const box = await element.boundingBox();
  if (!box) throw new Error("the element has no box to sample");
  const png = await page.screenshot({
    clip: { x: box.x, y: box.y, width: box.width, height: box.height },
  });
  return page.evaluate(
    async ({ b64, width, height }) => {
      const image = new Image();
      image.src = `data:image/png;base64,${b64}`;
      await image.decode();
      const canvas = document.createElement("canvas");
      canvas.width = image.width;
      canvas.height = image.height;
      const context = canvas.getContext("2d");
      if (!context) throw new Error("no 2d canvas");
      context.drawImage(image, 0, 0);
      const scaleX = image.width / width;
      const scaleY = image.height / height;
      const points: [number, number][] = [
        [4, height / 2],
        [width - 5, height / 2],
        [width / 2, 3],
        [width / 2, height - 4],
      ];
      return points.map(([x, y]) =>
        Array.from(context.getImageData(Math.floor(x * scaleX), Math.floor(y * scaleY), 1, 1).data.slice(0, 3)),
      );
    },
    { b64: png.toString("base64"), width: box.width, height: box.height },
  );
}

/**
 * The lowest contrast between an element's text and what is painted behind it.
 *
 * @param page - The page.
 * @param element - A visible element with text.
 * @returns The worst ratio over the sampled points.
 */
export async function worstTextContrast(page: Page, element: Locator): Promise<number> {
  const text = parseCssColor(await element.evaluate((el) => getComputedStyle(el).color));
  const samples = await paintedBackground(page, element);
  return Math.min(...samples.map((background) => contrastRatio(text, background)));
}
