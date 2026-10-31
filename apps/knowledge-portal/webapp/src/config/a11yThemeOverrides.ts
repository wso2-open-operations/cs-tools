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

import type { CSSObject, Theme } from "@mui/material/styles";
import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";

interface CssVarsTheme {
  applyStyles: (scheme: string, styles: Record<string, unknown>) => unknown;
  palette: { primary: { dark: string } };
}

/**
 * `sx` fragment for a text link coloured with the brand accent: the accent
 * itself in dark mode (about 7.7:1 on black), the darker shade in light mode
 * (5.2:1 on white, 4.8:1 on the page grey) where the accent is only 2.4-2.6:1.
 */
export const accessibleLinkSx = (theme: Theme): CSSObject => ({
  color: theme.palette.primary.main,
  ...theme.applyStyles("light", { color: theme.palette.primary.dark }),
});

/**
 * Accessibility overlay on the Oxygen theme. The brand accent used as text or
 * border on a light surface is about 2.5:1 and fails WCAG AA, so text and
 * outlined `color="primary"` buttons and chips use the darker `primary.dark`
 * shade, in the light colour scheme only (orange on dark already passes).
 * Same overlay the CSM portal applies.
 */
export function withA11yOverrides(base: OxygenTheme): OxygenTheme {
  const lightColor = ({ theme }: { theme: CssVarsTheme }) => ({
    ...(theme.applyStyles("light", {
      color: theme.palette.primary.dark,
    }) as Record<string, unknown>),
  });
  const lightColorAndBorder = ({ theme }: { theme: CssVarsTheme }) => ({
    ...(theme.applyStyles("light", {
      color: theme.palette.primary.dark,
      borderColor: theme.palette.primary.dark,
    }) as Record<string, unknown>),
  });

  type Slot = { styleOverrides?: Record<string, unknown> } | undefined;
  const baseComponents = (base as { components?: Record<string, unknown> })
    .components;
  const baseButton = (baseComponents?.MuiButton as Slot) ?? {};
  const baseChip = (baseComponents?.MuiChip as Slot) ?? {};

  return {
    ...base,
    components: {
      ...baseComponents,
      MuiButton: {
        ...baseButton,
        styleOverrides: {
          ...baseButton.styleOverrides,
          textPrimary: lightColor,
          outlinedPrimary: lightColorAndBorder,
        },
      },
      MuiChip: {
        ...baseChip,
        styleOverrides: {
          ...baseChip.styleOverrides,
          outlinedPrimary: lightColorAndBorder,
        },
      },
    },
  } as OxygenTheme;
}
