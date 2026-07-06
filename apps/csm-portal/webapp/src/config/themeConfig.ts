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

import {
  AcrylicOrangeTheme,
  AcrylicPurpleTheme,
  ChoreoTheme,
  ClassicTheme,
  HighContrastTheme,
} from "@wso2/oxygen-ui";
import type { OxygenTheme } from "@wso2/oxygen-ui/styles/Themes/OxygenThemeBase";

/**
 * Every Oxygen UI theme the CSM portal exposes, keyed by the value used in
 * `window.config.CSM_PORTAL_THEME` and persisted as the user's runtime choice.
 * Single source of truth for both the build-time default and the runtime theme
 * dropdown — add a theme here and it appears in the picker.
 */
export const THEMES = {
  acrylicOrange: AcrylicOrangeTheme,
  acrylicPurple: AcrylicPurpleTheme,
  choreo: ChoreoTheme,
  classic: ClassicTheme,
  highContrast: HighContrastTheme,
} satisfies Record<string, OxygenTheme>;

export type ThemeKey = keyof typeof THEMES;

export const DEFAULT_THEME_KEY: ThemeKey = "acrylicOrange";

/** Human labels for the theme dropdown, in display order. */
export const THEME_OPTIONS: { key: ThemeKey; label: string }[] = [
  { key: "acrylicOrange", label: "Acrylic Orange" },
  { key: "acrylicPurple", label: "Acrylic Purple" },
  { key: "choreo", label: "Choreo" },
  { key: "classic", label: "Classic" },
  { key: "highContrast", label: "High Contrast" },
];

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
 * {@link DEFAULT_THEME_KEY}. The runtime picker layers a persisted user choice
 * on top of this (see `ThemePreferenceProvider`).
 */
export function configThemeKey(): ThemeKey {
  const fromConfig = window.config?.CSM_PORTAL_THEME;
  return isThemeKey(fromConfig) ? fromConfig : DEFAULT_THEME_KEY;
}
