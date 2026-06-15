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

/**
 * WCAG-aware text-colour selection for coloured surfaces (chips, badges).
 *
 * MUI's built-in `getContrastText` targets a 3:1 ratio (the floor for *large*
 * text). Chip and badge labels are small (~13px), which need 4.5:1, so a filled
 * `color="error"`/`"warning"`/`"info"` chip can render white-on-saturated text
 * at ~3:1 and silently fail WCAG AA. Rather than hardcode per-theme hexes (which
 * break when the Oxygen theme or mode changes), pick the label colour from the
 * resolved background's measured luminance: whichever of near-black / white
 * gives the higher contrast. For the saturated semantic `*.main` colours in
 * every shipped theme this clears 4.5:1.
 */

/** Opacity of MUI's conventional near-black body-text colour. */
export const NEAR_BLACK_ALPHA = 0.87;
/** MUI's conventional near-black body-text colour (translucent). */
export const NEAR_BLACK = `rgba(0, 0, 0, ${NEAR_BLACK_ALPHA})`;
export const WHITE = "#ffffff";

interface Rgb {
  r: number;
  g: number;
  b: number;
}

/** Parse `#rgb`, `#rrggbb`, or `rgb()/rgba()` into 0-255 channels; null if not. */
export function parseColor(input: string): Rgb | null {
  if (!input) return null;
  const s = input.trim();
  const hex = s.match(/^#([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (hex) {
    let h = hex[1];
    if (h.length === 3) {
      h = h
        .split("")
        .map((c) => c + c)
        .join("");
    }
    return {
      r: parseInt(h.slice(0, 2), 16),
      g: parseInt(h.slice(2, 4), 16),
      b: parseInt(h.slice(4, 6), 16),
    };
  }
  const rgb = s.match(/rgba?\(([^)]+)\)/i);
  if (rgb) {
    const parts = rgb[1].split(",").map((p) => parseFloat(p));
    if (parts.length >= 3 && parts.slice(0, 3).every((n) => !Number.isNaN(n))) {
      return { r: parts[0], g: parts[1], b: parts[2] };
    }
  }
  return null;
}

function channelLuminance(value: number): number {
  const v = value / 255;
  return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
}

/** WCAG relative luminance of an RGB colour. */
export function relativeLuminance({ r, g, b }: Rgb): number {
  return (
    0.2126 * channelLuminance(r) +
    0.7152 * channelLuminance(g) +
    0.0722 * channelLuminance(b)
  );
}

/** WCAG contrast ratio between two parsed colours (1..21). */
export function contrastRatio(a: Rgb, b: Rgb): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const hi = Math.max(la, lb);
  const lo = Math.min(la, lb);
  return (hi + 0.05) / (lo + 0.05);
}

/** Composite a translucent foreground over an opaque background. */
export function compositeOver(fg: Rgb, alpha: number, bg: Rgb): Rgb {
  return {
    r: fg.r * alpha + bg.r * (1 - alpha),
    g: fg.g * alpha + bg.g * (1 - alpha),
    b: fg.b * alpha + bg.b * (1 - alpha),
  };
}

/**
 * Pick the label colour (near-black or white) with the higher contrast against
 * `background`. The near-black candidate is the translucent {@link NEAR_BLACK},
 * so we evaluate the colour it actually composites to over the (opaque) surface
 * — not opaque black — otherwise the picked colour could sit below 4.5:1 even
 * though the opaque-black comparison said it passed. Falls back to white for an
 * unparseable background so a coloured chip never renders invisible text.
 */
export function pickAccessibleText(background: string): string {
  const bg = parseColor(background);
  if (!bg) return WHITE;
  const white = { r: 255, g: 255, b: 255 };
  const nearBlack = compositeOver({ r: 0, g: 0, b: 0 }, NEAR_BLACK_ALPHA, bg);
  return contrastRatio(bg, nearBlack) >= contrastRatio(bg, white)
    ? NEAR_BLACK
    : WHITE;
}
