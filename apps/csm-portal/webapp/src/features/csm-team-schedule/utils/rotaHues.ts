/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * The palette teams are coloured from.
 *
 * A list of colours, not a list of teams. The previous version mapped eleven
 * organisation team keys to hexes in committed source, which is the coupling
 * CSM_TEAM_REGISTRY exists to avoid -- raised in review, and correct.
 *
 * A team's colour is now its position in the list the catalogue serves, so
 * every team keeps a distinct colour and no team is named here. Deriving a
 * hue from the key instead was tried and rejected: it collapsed eleven teams
 * onto eight colours, because no hash can promise separation for a key set it
 * has never seen.
 *
 * Ordered so that neighbours in the list are far apart in hue, since teams
 * next to each other in the roster are the ones most often compared.
 */
export const TEAM_PALETTE: readonly string[] = [
  // Fourteen hues, each its own colour family -- the list once held three
  // blues, and two teams side by side in the roster read as one. Neighbours
  // are far apart on the wheel, since teams next to each other in the roster
  // are the ones most often compared.
  "#3b6fd8", // blue
  "#e07b1a", // orange
  "#1f9e8c", // teal
  "#c2479a", // magenta
  "#7c55cf", // violet
  "#d64545", // red
  "#3f9b3a", // green
  "#c49a12", // gold
  "#1593b5", // cyan
  "#9a5a2c", // brown
  "#e05f8a", // pink
  "#6b8a12", // olive
  "#4b4fbf", // indigo
  "#b5651d", // copper
];

/**
 * The SRE time zones, in the prototype's own hues.
 *
 * These were JS constants there, not CSS tokens -- which is why an earlier
 * version of this page referenced --tz1-fg and friends, found nothing, and
 * drew all three lanes in the same fallback grey. A zone's colour is how a
 * reader tells the three columns apart at a glance, so it is worth being
 * explicit about.
 */
export const ZONE_COLOURS: Record<string, string> = {
  TZ1: "#e8962a",
  TZ2: "#4a7fe0",
  TZ3: "#8a63d2",
};

/** A rotation's Day and Night zones (MOE_D, MOE_N): the same two colours for
 *  every rotation, since only one rotation is ever on screen at a time. */
const DAY_NIGHT_COLOURS = { day: "#e8962a", night: "#5a47b6" } as const;

/** The colour for a time zone, falling back to a neutral for one not listed. */
export function zoneColour(code: string): string {
  const c = code.toUpperCase();
  if (ZONE_COLOURS[c]) return ZONE_COLOURS[c];
  if (/_D$/.test(c)) return DAY_NIGHT_COLOURS.day;
  if (/_N$/.test(c)) return DAY_NIGHT_COLOURS.night;
  return "#6b7280";
}

/**
 * The colour for a team, by its position in the catalogue.
 *
 * `order` comes from the served team list. A team the caller has no position
 * for -- one that has left the registry but still has rota history -- falls
 * back to a neutral rather than borrowing somebody else's colour.
 */
export function teamColour(order: number | undefined): string {
  if (order === undefined || order < 0) return "#6b7280";
  if (order < TEAM_PALETTE.length) return TEAM_PALETTE[order];
  // Past the palette, a generated hue rather than the first colour again: the
  // golden angle keeps each new one as far as it can be from those before.
  const hue = Math.round((order * 137.508 + 20) % 360);
  return `hsl(${hue} 55% 45%)`;
}

/**
 * Chips drawn as light text on a dark fill. For these the text colour is
 * white, so a card or row tinted "in the chip's colour" by its text colour was
 * tinted white -- the evening card's title all but vanished on a light page.
 * Their fill is the colour that identifies them.
 */
const DARK_CHIPS = new Set(["pm", "ext", "al", "ll", "onb", "exc"]);

/** The colour a card, cell or row takes from a chip's colour token: the
 *  chip's text colour, or its fill for a chip that is dark with light text. */
export function accentOf(token: string | undefined, fallback = "var(--faint)"): string {
  const t = (token ?? "").toLowerCase();
  if (!t) return fallback;
  return DARK_CHIPS.has(t) ? `var(--${t}-bg, ${fallback})` : `var(--${t}-fg, ${fallback})`;
}
