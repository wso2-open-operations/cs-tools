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

/* eslint-disable react-refresh/only-export-components -- Provider component and its useXxx hook are colocated per the repo's context idiom (fast-refresh DX only) */

import { OxygenUIThemeProvider } from "@wso2/oxygen-ui";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";
import {
  configThemeKey,
  isThemeKey,
  resolveTheme,
  THEME_OPTIONS,
  type ThemeKey,
} from "@config/themeConfig";
import { withA11yOverrides } from "@config/a11yThemeOverrides";

const STORAGE_KEY = "csm.theme";

interface ThemePreferenceContextValue {
  /** The currently applied Oxygen theme key. */
  themeKey: ThemeKey;
  /** Set (and persist) the theme key. */
  setThemeKey: (next: ThemeKey) => void;
  /** Theme keys + labels for rendering the picker. */
  options: typeof THEME_OPTIONS;
}

const ThemePreferenceContext =
  createContext<ThemePreferenceContextValue | null>(null);

/**
 * Initial theme key: a saved user choice wins, otherwise the build-time
 * `window.config.CSM_PORTAL_THEME` default.
 */
function readInitial(): ThemeKey {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (isThemeKey(saved)) return saved;
  } catch {
    /* localStorage may be unavailable — fall back to the config default */
  }
  return configThemeKey();
}

/**
 * Owns the runtime theme selection. Holds the chosen key in React state so the
 * picker can switch themes live, persists the choice to localStorage, and wraps
 * children in {@link OxygenUIThemeProvider} with the resolved theme. Sits above
 * the rest of the provider tree so every component (including the header
 * dropdown) renders under the selected theme.
 */
export function ThemePreferenceProvider({
  children,
}: {
  children: ReactNode;
}): JSX.Element {
  const [themeKey, setThemeKeyState] = useState<ThemeKey>(() => readInitial());

  // Layer the WCAG-AA accent overlay on the resolved theme (see
  // withA11yOverrides). Memoised so the theme object is stable per key.
  const theme = useMemo(() => withA11yOverrides(resolveTheme(themeKey)), [
    themeKey,
  ]);

  const setThemeKey = useCallback((next: ThemeKey): void => {
    setThemeKeyState(next);
    try {
      window.localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* ignore — the in-memory choice still applies for this session */
    }
  }, []);

  const value = useMemo<ThemePreferenceContextValue>(
    () => ({ themeKey, setThemeKey, options: THEME_OPTIONS }),
    [themeKey, setThemeKey],
  );

  return (
    <ThemePreferenceContext.Provider value={value}>
      <OxygenUIThemeProvider theme={theme}>
        {children}
      </OxygenUIThemeProvider>
    </ThemePreferenceContext.Provider>
  );
}

export function useThemePreference(): ThemePreferenceContextValue {
  const ctx = useContext(ThemePreferenceContext);
  if (!ctx) {
    throw new Error(
      "useThemePreference must be used within a ThemePreferenceProvider",
    );
  }
  return ctx;
}
