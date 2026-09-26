import { useTheme } from "@wso2/oxygen-ui";
import { useMemo } from "react";

/**
 * The colour series every chart in the portal draws from.
 *
 * Built by rotating hue away from the active theme's primary colour, rather
 * than by listing the theme's semantic colours. Semantic palettes are not
 * categorical palettes: in Acrylic Orange, `secondary.main` is #E8E8E8 and
 * `warning.main` is a near neighbour of `primary.main`, so a five-slice ring
 * drawn from them had one invisible slice and two that looked identical. Hue
 * rotation guarantees separation while staying anchored to whichever of the
 * five themes the user picked.
 *
 * Oxygen's charts do supply a default series, but it is built from the *syntax
 * highlighting* palette (comment green, keyword blue, string orange) and is
 * only six entries long. The dashboard needs nine on one ring: the lifecycle
 * stages.
 */

const SERIES_LENGTH = 12;

/**
 * Positions in the hue wheel, ordered so that consecutive colours are as far
 * apart as possible. Charts usually plot three or four series, and those should
 * be opposite each other on the wheel rather than adjacent — walking 0, 1, 2
 * would hand a three-slice ring three shades of the same hue.
 */
const SPREAD = [0, 6, 3, 9, 1, 7, 4, 10, 2, 8, 5, 11];

/** Hue, saturation and lightness of a `#rrggbb` colour, in degrees and percent. */
function hexToHsl(hex: string): { h: number; s: number; l: number } | null {
  const match = /^#?([\da-f]{6})$/i.exec(hex.trim());
  if (!match) return null;
  const int = parseInt(match[1], 16);
  const r = ((int >> 16) & 255) / 255;
  const g = ((int >> 8) & 255) / 255;
  const b = (int & 255) / 255;

  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const delta = max - min;
  const l = (max + min) / 2;

  if (delta === 0) return { h: 0, s: 0, l: l * 100 };

  const s = delta / (1 - Math.abs(2 * l - 1));
  let h: number;
  if (max === r) h = ((g - b) / delta) % 6;
  else if (max === g) h = (b - r) / delta + 2;
  else h = (r - g) / delta + 4;

  return { h: ((h * 60) % 360 + 360) % 360, s: s * 100, l: l * 100 };
}

export function useChartColors(): string[] {
  const theme = useTheme();

  return useMemo(() => {
    const anchor = hexToHsl(theme.palette.primary.main);
    const dark = theme.palette.mode === "dark";

    // A theme whose primary is not a plain hex (a gradient, a CSS variable) or
    // is a grey with no hue of its own: start from a warm hue so the series is
    // still readable rather than twelve greys.
    const baseHue = anchor && anchor.s > 8 ? anchor.h : 28;

    // One saturation and lightness for the whole series, so no slice reads as
    // more important than its neighbours. Clamped rather than inherited: a very
    // pale or very saturated primary makes a poor basis for eleven siblings.
    // Muted deliberately. Twelve fully saturated hues read as a rainbow and
    // fight the restrained Oxygen surfaces they sit on; pulling saturation back
    // keeps the slices separable while letting the theme stay the loudest thing
    // on the page.
    const saturation = Math.round(Math.min(64, Math.max(46, (anchor?.s ?? 62) * 0.8)));
    const lightness = dark ? 60 : 48;

    const step = 360 / SERIES_LENGTH;
    return SPREAD.map((position) => {
      const hue = Math.round((baseHue + position * step) % 360);
      return `hsl(${hue} ${saturation}% ${lightness}%)`;
    });
  }, [theme.palette.primary.main, theme.palette.mode]);
}

