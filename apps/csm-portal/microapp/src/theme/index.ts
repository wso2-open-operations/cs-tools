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

import type { OxygenTheme } from "@wso2/oxygen-ui/styles/OxygenThemeBase";
import { WSO2Theme, extendTheme } from "@wso2/oxygen-ui";
import { typography } from "@theme/typography";

const theme = extendTheme(WSO2Theme, {
  typography,
  components: {
    // WSO2Theme's dialog surface renders background.paper as-is, which is a
    // semi-transparent token (not a solid color) — that reads as hazy/unreadable against
    // varied page content behind it. All of this app's filter sheets (FiltersSheet.tsx and
    // friends) are built on MUI Dialog, so fix it once here rather than overriding sx per
    // sheet. Solid, not background.paper/acrylic, and reads `theme.vars.palette.*` (a CSS
    // var) rather than the static `theme.palette.mode`, since this theme switches light/dark
    // purely via CSS variables reacting to the OS color scheme.
    MuiDialog: {
      styleOverrides: {
        paper: ({ theme }) => ({
          WebkitBackdropFilter: "none",
          backdropFilter: "none",
          background: theme.vars.palette.background.default,
          opacity: 1,
        }),
      },
    },
    // WSO2Theme's own MuiButton override sets a black `root` text color for every button, and
    // only restores a readable color for the `outlined` variant (colored text) and `text`
    // variant (theme.palette.text.primary) — `contained` has no override at all, so every
    // contained button in the app (Create Case, Post, Save, filter-sheet Apply, this app's own
    // CaseActionBar single-target button, ...) fell through to black text on its own colored
    // background. One override here fixes all of them, keyed off whichever `color` prop the
    // button actually uses (not just primary) so error/success/etc. contained buttons
    // (TimeCardReviewDialog's Approve/Reject, CallRequestsTab's Cancel request) get their own
    // matching contrastText too.
    MuiButton: {
      styleOverrides: {
        contained: ({ theme, ownerState }) => {
          const color = ownerState.color;
          if (!color || color === "inherit") return {};
          return { color: theme.vars.palette[color].contrastText };
        },
      },
    },
  },
}) as OxygenTheme;

export default theme;
