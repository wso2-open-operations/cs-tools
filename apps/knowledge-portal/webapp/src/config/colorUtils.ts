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

import { alpha, type Theme } from "@mui/material/styles";

interface ThemeWithVars {
  vars?: { palette: { primary: { mainChannel: string }; background: { paperChannel: string } } };
}

/**
 * A translucent tint of the brand colour, for soft backgrounds. Oxygen themes
 * expose colours as CSS variables, which `alpha()` cannot parse, so the
 * channel variable is used when present.
 */
export function primaryTint(theme: Theme, opacity: number): string {
  const channel = (theme as ThemeWithVars).vars?.palette.primary.mainChannel;
  return channel ? `rgba(${channel} / ${opacity})` : alpha(theme.palette.primary.main, opacity);
}

/**
 * The paper colour at (near) full opacity. Acrylic Oxygen themes make surfaces
 * translucent, which suits cards on a page but not a dropdown or a button that
 * floats over text.
 */
export function opaquePaper(theme: Theme, opacity = 0.98): string {
  const channel = (theme as ThemeWithVars).vars?.palette.background.paperChannel;
  return channel ? `rgba(${channel} / ${opacity})` : theme.palette.background.paper;
}

/** True when the visitor has asked for reduced motion; smooth scrolling is skipped then. */
export function prefersReducedMotion(): boolean {
  return typeof window !== "undefined" && !!window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
}
