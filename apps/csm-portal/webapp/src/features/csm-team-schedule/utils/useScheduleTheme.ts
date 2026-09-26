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

import type { CSSProperties } from "react";

/**
 * The prototype's structural tokens, pointed at the portal's own palette.
 *
 * These are `var()` references, not copied values, and that distinction is the
 * whole point. Oxygen switches light and dark by flipping
 * `--oxygen-palette-*` on the document under a `data-color-scheme` attribute;
 * nothing re-renders. An earlier version of this read `theme.palette.*` into a
 * memo, captured literals, and then sat frozen in light colours while the user
 * cycled the header's mode button -- the page simply never followed.
 *
 * Referencing the variables means the page follows the mode toggle, every named
 * theme (classic, acrylic orange, acrylic purple, high contrast) and any theme
 * added later, with no JavaScript in the loop at all.
 *
 * The rotation hues are not here: those need a different value per mode rather
 * than a pointer at one, so they live in the stylesheet keyed on the same
 * attribute -- see teamSchedule.css.
 */
export const SCHEDULE_THEME_VARS: CSSProperties = {
  "--ground": "var(--oxygen-palette-background-default)",
  "--surface": "var(--oxygen-palette-background-paper)",
  /*
   * An opaque surface, for anything that has to sit over scrolling content.
   *
   * The portal's paper colour carries alpha (#ffffffc5 in the light themes),
   * which is fine for a card sitting still and wrong for a sticky one: the
   * roster's engineer column let thirty days of cells scroll straight through
   * it, and the lane headings had the same problem. background-default has no
   * alpha, so things pinned over the page actually cover it.
   */
  "--surface-solid": "var(--oxygen-palette-background-default)",
  "--surface-2": "var(--oxygen-palette-action-hover)",
  "--surface-3": "var(--oxygen-palette-action-selected)",
  "--ink": "var(--oxygen-palette-text-primary)",
  "--ink-2": "var(--oxygen-palette-text-secondary)",
  "--muted": "var(--oxygen-palette-text-secondary)",
  "--faint": "var(--oxygen-palette-text-disabled)",
  "--line": "var(--oxygen-palette-divider)",
  "--line-2": "var(--oxygen-palette-divider)",
  "--accent": "var(--oxygen-palette-primary-main)",
  "--accent-ink": "var(--oxygen-palette-primary-main)",
  "--accent-soft": "var(--oxygen-palette-action-selected)",
  "--accent-soft-2": "var(--oxygen-palette-action-hover)",
  "--today": "var(--oxygen-palette-primary-main)",
  "--now": "var(--oxygen-palette-error-main)",
  "--weekend": "var(--oxygen-palette-action-hover)",
} as CSSProperties;
