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

// Port of v3's src/lib/theme.ts.
import { AcrylicPurpleTheme } from "@wso2/oxygen-ui";

// Acrylic Purple is Oxygen UI's frosted-glass material theme: translucent
// surfaces + backdrop blur over a soft purple gradient background, primary
// accent #646cff. The dashboard's own design tokens (--sla-*, global.css)
// alias this theme's live CSS variables (--oxygen-palette-*, --oxygen-blur-*,
// --oxygen-gradient-primary) rather than hardcoding hex values, so switching
// themes here propagates everywhere without touching component code.
//
// Pass the theme object through as-is — do NOT re-wrap it in a second
// `createTheme(AcrylicPurpleTheme, {...})` call. MUI's createTheme rebuilds
// the CSS-vars tree from scratch from `colorSchemes`, which silently drops
// Oxygen UI's custom top-level fields (`gradient`, `blur`, `border`) that
// were attached via a direct `Object.assign` after the theme was built —
// confirmed by `--oxygen-gradient-primary` resolving to WSO2Theme's orange
// gradient instead of Acrylic Purple's, the one time this was tried.
export const theme = AcrylicPurpleTheme;
