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

import { afterEach, describe, expect, it } from "vitest";
import {
  configThemeKey,
  DEFAULT_THEME_KEY,
  isThemeKey,
  resolveTheme,
  THEME_OPTIONS,
} from "@config/themeConfig";

describe("themeConfig", () => {
  const originalConfig = window.config;

  afterEach(() => {
    window.config = originalConfig;
  });

  it("keeps the two remaining legacy keys the picker still offers", () => {
    const keys = THEME_OPTIONS.map((o) => o.key);
    expect(keys).toEqual(
      expect.arrayContaining(["acrylicOrange", "acrylicPurple"]),
    );
  });

  it("excludes classic and highContrast from the picker, but keeps them resolvable", () => {
    const keys = THEME_OPTIONS.map((o) => o.key);
    expect(keys).not.toContain("classic");
    expect(keys).not.toContain("highContrast");
    // still resolvable for back-compat with an already-persisted preference
    expect(isThemeKey("classic")).toBe(true);
    expect(isThemeKey("highContrast")).toBe(true);
    expect(resolveTheme("classic")).toBeTruthy();
    expect(resolveTheme("highContrast")).toBeTruthy();
  });

  it("orders the surviving legacy keys first, in their historical relative order", () => {
    const keys = THEME_OPTIONS.map((o) => o.key);
    const legacyIndexes = ["acrylicOrange", "acrylicPurple"].map((k) =>
      keys.indexOf(k),
    );
    expect(legacyIndexes).toEqual([...legacyIndexes].sort((a, b) => a - b));
    // and every legacy key sorts before every non-legacy one
    const firstNonLegacyIndex = keys.findIndex(
      (k) => !["acrylicOrange", "acrylicPurple"].includes(k),
    );
    if (firstNonLegacyIndex !== -1) {
      expect(Math.max(...legacyIndexes)).toBeLessThan(firstNonLegacyIndex);
    }
  });

  it("derives readable labels for the surviving legacy keys", () => {
    const byKey = Object.fromEntries(THEME_OPTIONS.map((o) => [o.key, o.label]));
    expect(byKey.acrylicOrange).toBe("Acrylic Orange");
    expect(byKey.acrylicPurple).toBe("Acrylic Purple");
  });

  it("picks up new oxygen-ui exports (e.g. WSO2Theme) without a hand-written map", () => {
    const keys = THEME_OPTIONS.map((o) => o.key);
    // These come from oxygen-ui 0.13.1's expanded theme set; if oxygen-ui
    // ever removes one this assertion should be updated, not the app code.
    expect(keys).toEqual(
      expect.arrayContaining(["wso2", "paleGray", "paleIndigo"]),
    );
    const byKey = Object.fromEntries(THEME_OPTIONS.map((o) => [o.key, o.label]));
    expect(byKey.wso2).toBe("WSO2");
    expect(byKey.paleGray).toBe("Pale Gray");
    expect(byKey.paleIndigo).toBe("Pale Indigo");
  });

  it("has no duplicate keys", () => {
    const keys = THEME_OPTIONS.map((o) => o.key);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("isThemeKey / resolveTheme accept every derived key and reject junk", () => {
    for (const { key } of THEME_OPTIONS) {
      expect(isThemeKey(key)).toBe(true);
      expect(resolveTheme(key)).toBeTruthy();
    }
    expect(isThemeKey("not-a-real-theme")).toBe(false);
    expect(isThemeKey(undefined)).toBe(false);
    expect(resolveTheme("not-a-real-theme")).toBe(resolveTheme(DEFAULT_THEME_KEY));
  });

  it("defaults to acrylicOrange", () => {
    expect(DEFAULT_THEME_KEY).toBe("acrylicOrange");
  });

  it("configThemeKey falls back to the default when window.config is unset or invalid", () => {
    window.config = undefined as unknown as Window["config"];
    expect(configThemeKey()).toBe(DEFAULT_THEME_KEY);

    window.config = {
      ...originalConfig,
      CSM_PORTAL_THEME: "not-a-real-theme",
    } as Window["config"];
    expect(configThemeKey()).toBe(DEFAULT_THEME_KEY);
  });

  it("configThemeKey honors a valid window.config value, including a newly derived key", () => {
    window.config = {
      ...originalConfig,
      CSM_PORTAL_THEME: "wso2",
    } as Window["config"];
    expect(configThemeKey()).toBe("wso2");
  });
});
