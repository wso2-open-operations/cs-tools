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

import type { OxygenTheme } from "@wso2/oxygen-ui/styles/Themes/OxygenThemeBase";
import { pickAccessibleText } from "@utils/contrastText";

/**
 * Accessibility overlay applied on top of whichever Oxygen theme is active.
 *
 * The brand accent (`primary.main`, a light salmon orange ~`#F87643`) is fine
 * as white-on-orange (contained buttons) and as orange-on-dark, but as *text or
 * border on a light surface* it sits around 2.5:1 and fails WCAG AA. That makes
 * every text/outlined `color="primary"` button unreadable in light mode. Rather
 * than darken the brand fill (which would dull the contained CTA the brand
 * wants kept vivid) or patch each call site, we shift only the text/border
 * colour of text & outlined primary buttons to the darker `primary.dark` shade,
 * and only in the light colour scheme (`applyStyles("light", …)`); dark mode,
 * where orange-on-dark already passes, is untouched. Kept theme-agnostic — it
 * reads `primary.dark` from whatever theme is active, so it survives a theme
 * swap and is not a different named theme.
 *
 * Returns a shallow clone of `base` with the extra `MuiButton` style slots
 * merged in, preserving the theme's CSS-variable colour schemes.
 */
export function withA11yOverrides(base: OxygenTheme): OxygenTheme {
  // CSS-vars themes expose `applyStyles(scheme, styles)`, which scopes the
  // given styles to one colour scheme. Typed loosely because the public
  // OxygenTheme type does not surface the CssVars helpers.
  const lightColor = ({
    theme,
  }: {
    theme: {
      applyStyles: (scheme: string, styles: Record<string, unknown>) => unknown;
      palette: { primary: { dark: string } };
    };
  }): Record<string, unknown> => ({
    ...(theme.applyStyles("light", {
      color: theme.palette.primary.dark,
    }) as Record<string, unknown>),
  });

  const lightColorAndBorder = ({
    theme,
  }: {
    theme: {
      applyStyles: (scheme: string, styles: Record<string, unknown>) => unknown;
      palette: { primary: { dark: string } };
    };
  }): Record<string, unknown> => ({
    ...(theme.applyStyles("light", {
      color: theme.palette.primary.dark,
      borderColor: theme.palette.primary.dark,
    }) as Record<string, unknown>),
  });

  // Oxygen's `UserMenu` paints the user-initial avatar (trigger + dropdown
  // header) as white-on-`primary.main`, which is ~2.7:1 and fails AA in BOTH
  // modes (the fill is orange in light and dark alike). The avatar is rendered
  // inside the library component, so there is no call site to patch: override
  // the `MuiUserMenu` `avatar`/`headerAvatar` slots and let the initial pick a
  // contrast-safe colour from the resolved fill. Unlike the button fix this is
  // not mode-scoped: the fill is the same orange in both schemes, so the same
  // dark-on-orange label is correct everywhere. Mirrors the inline avatar fix
  // in `CsmCaseCommentBubble`.
  const avatarText = ({
    theme,
  }: {
    theme: { palette: { primary: { main: string } } };
  }): Record<string, unknown> => ({
    color: pickAccessibleText(theme.palette.primary.main),
  });

  const baseComponents = (base as { components?: Record<string, unknown> })
    .components;
  const baseButton =
    (baseComponents?.MuiButton as
      | { styleOverrides?: Record<string, unknown> }
      | undefined) ?? {};
  const baseUserMenu =
    (baseComponents?.MuiUserMenu as
      | { styleOverrides?: Record<string, unknown> }
      | undefined) ?? {};
  const baseChip =
    (baseComponents?.MuiChip as
      | { styleOverrides?: Record<string, unknown> }
      | undefined) ?? {};

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
      MuiUserMenu: {
        ...baseUserMenu,
        styleOverrides: {
          ...baseUserMenu.styleOverrides,
          avatar: avatarText,
          headerAvatar: avatarText,
        },
      },
      // Outlined `color="primary"` chips (e.g. the "WSO2" author-org badge on
      // comments) paint their label and border with `primary.main`, which is
      // ~2.5:1 on a light surface — the same orange-on-light failure as the
      // text/outlined buttons above, and fixed the same way: shift to
      // `primary.dark` in light mode only. Dark mode (orange-on-dark) passes
      // and is left untouched.
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
