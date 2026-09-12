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

import * as OxygenUI from "@wso2/oxygen-ui";
import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";

const THEME_EXPORT_SUFFIX = "Theme";

/**
 * Themes the CSM portal shipped with before this file switched from a
 * hand-picked list to deriving `THEMES`/`THEME_OPTIONS` from whatever
 * `@wso2/oxygen-ui` actually exports (see module doc below). Used only to
 * order the picker — these four sort first, in this relative order, so the
 * dropdown an agent already knows doesn't reshuffle every time oxygen-ui
 * adds or removes a theme. Everything else is appended after, in whatever
 * order the package exports it.
 */
const LEGACY_KEY_ORDER = [
  "acrylicOrange",
  "acrylicPurple",
  "classic",
  "highContrast",
];

/**
 * Theme keys still resolvable (so an already-persisted user preference or
 * `window.config.CSM_PORTAL_THEME` value doesn't silently break) but no
 * longer offered in the picker.
 */
const EXCLUDED_FROM_OPTIONS = new Set(["classic", "highContrast"]);

/** Duck-types an oxygen-ui export as an actual MUI/Oxygen theme object, as
 * opposed to a component, hook, or provider whose name happens to end in
 * "Theme" (e.g. a future `OxygenUIThemeProvider`-shaped export). */
function isOxygenThemeValue(value: unknown): value is OxygenTheme {
  return (
    typeof value === "object" &&
    value !== null &&
    "palette" in value &&
    "typography" in value
  );
}

/**
 * Splits a PascalCase name into its constituent words, treating a run of
 * capitals followed by lowercase/digits as one word — so "WSO2" stays a
 * single word (`["WSO2"]`) while "AcrylicOrange" splits into
 * `["Acrylic", "Orange"]`. Falls back to the whole name if it doesn't look
 * like PascalCase at all.
 */
function splitPascalWords(name: string): string[] {
  return name.match(/[A-Z]+[a-z0-9]*/g) ?? [name];
}

/**
 * Derives this app's `{ key, label }` for an oxygen-ui theme export name
 * (e.g. `"AcrylicOrangeTheme"` -> `{ key: "acrylicOrange", label: "Acrylic
 * Orange" }`, `"WSO2Theme"` -> `{ key: "wso2", label: "WSO2" }`). A small
 * reusable rule (strip the trailing "Theme", camelCase the key, space out
 * the label) instead of another hardcoded per-theme map — the whole point
 * of deriving the registry is that a new oxygen-ui theme needs zero changes
 * here.
 */
function deriveThemeMeta(exportName: string): { key: string; label: string } {
  const stripped = exportName.endsWith(THEME_EXPORT_SUFFIX)
    ? exportName.slice(0, -THEME_EXPORT_SUFFIX.length)
    : exportName;
  const words = splitPascalWords(stripped);
  const key = words
    .map((word, index) => (index === 0 ? word.toLowerCase() : word))
    .join("");
  const label = words.join(" ");
  return { key, label };
}

/**
 * Every oxygen-ui export ending in "Theme" whose value actually looks like a
 * theme object, in the order the package exports them, with this app's
 * derived key/label attached. Single pass that both `THEMES` and
 * `THEME_OPTIONS` below are built from — not a hand-picked subset — so a
 * future oxygen-ui release adding or removing a theme doesn't need a manual
 * edit here.
 */
const DERIVED_THEME_ENTRIES = Object.entries(OxygenUI)
  .filter(
    ([name, value]) =>
      name.endsWith(THEME_EXPORT_SUFFIX) && isOxygenThemeValue(value),
  )
  .map(([name, value]) => ({
    ...deriveThemeMeta(name),
    theme: value as OxygenTheme,
  }));

/**
 * Every Oxygen UI theme the CSM portal exposes, keyed by the value used in
 * `window.config.CSM_PORTAL_THEME` and persisted as the user's runtime
 * choice.
 */
const THEMES: Record<string, OxygenTheme> = Object.fromEntries(
  DERIVED_THEME_ENTRIES.map(({ key, theme }) => [key, theme]),
);

export type ThemeKey = keyof typeof THEMES;

export const DEFAULT_THEME_KEY: ThemeKey = "acrylicOrange";

/**
 * Human labels for the theme dropdown. Ordered so the four themes already
 * in use (see `LEGACY_KEY_ORDER`) keep their current relative order first;
 * anything newly derived is appended after, in package export order.
 */
export const THEME_OPTIONS: { key: ThemeKey; label: string }[] =
  DERIVED_THEME_ENTRIES.filter(({ key }) => !EXCLUDED_FROM_OPTIONS.has(key))
    .map(({ key, label }) => ({
      key: key as ThemeKey,
      label,
      legacyIndex: LEGACY_KEY_ORDER.indexOf(key),
    }))
    .sort((a, b) => {
      const aOrder = a.legacyIndex === -1 ? LEGACY_KEY_ORDER.length : a.legacyIndex;
      const bOrder = b.legacyIndex === -1 ? LEGACY_KEY_ORDER.length : b.legacyIndex;
      return aOrder - bOrder;
    })
    .map(({ key, label }) => ({ key, label }));

/** True when `value` is a known theme key. */
export function isThemeKey(value: unknown): value is ThemeKey {
  return typeof value === "string" && value in THEMES;
}

/** Resolve a (possibly invalid) key to a concrete Oxygen theme. */
export function resolveTheme(key: string | undefined): OxygenTheme {
  return isThemeKey(key) ? THEMES[key] : THEMES[DEFAULT_THEME_KEY];
}

/**
 * Build-time default theme key from `window.config`, falling back to
 * {@link DEFAULT_THEME_KEY}. The runtime picker layers a persisted user
 * choice on top of this (see `ThemePreferenceProvider`).
 */
export function configThemeKey(): ThemeKey {
  const fromConfig = window.config?.CSM_PORTAL_THEME;
  return isThemeKey(fromConfig) ? fromConfig : DEFAULT_THEME_KEY;
}
