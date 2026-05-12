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

import type { UIEvent } from "react";
import { PAGINATED_SELECT_MENU_MAX_HEIGHT_PX } from "@constants/common";

/**
 * MenuList props for paginated selects: fixed max height + optional scroll handler.
 *
 * @param {((e: UIEvent<HTMLElement>) => void) | undefined} onScroll - Near-bottom handler for fetchNextPage.
 * @returns {object} MUI MenuProps.MenuListProps fragment.
 */
export function paginatedSelectMenuListProps(
  onScroll?: (e: UIEvent<HTMLElement>) => void,
): {
  onScroll?: (e: UIEvent<HTMLElement>) => void;
  sx: { maxHeight: number; overflowY: "auto" };
} {
  return {
    ...(onScroll ? { onScroll } : {}),
    sx: {
      maxHeight: PAGINATED_SELECT_MENU_MAX_HEIGHT_PX,
      overflowY: "auto",
    },
  };
}

/**
 * Strips pure-white inline background declarations from style attributes so
 * dark-mode containers no longer render white boxes on a dark background.
 * Everything else (code-block backgrounds, borders, shadows, text colors) is
 * intentionally left untouched so light-mode and structural styling stay intact.
 *
 * @param html - Raw HTML string.
 * @returns HTML with pure-white background declarations removed.
 */
export function stripLightModeInlineStyles(html: string): string {
  return html.replace(
    /style\s*=\s*"([^"]*)"/gi,
    (_match, styleContent: string) => {
      const declarations = styleContent.split(";");
      const filtered = declarations.filter((decl) => {
        const normalized = decl.toLowerCase().replace(/\s+/g, " ").trim();
        if (!normalized) return false;
        if (
          /^background(-color)?\s*:\s*(#fff(fff)?|white|#f4f4f4|#f5f5f5|#f0f0f0|#f9f9f9|#f8f8f8|#fafafa|#e9e9e9)\s*$/.test(
            normalized,
          )
        )
          return false;
        if (/^color\s*:/.test(normalized) && isDarkColor(normalized))
          return false;
        return true;
      });
      const cleaned = filtered.join(";").replace(/;+$/, "").trim();
      if (!cleaned) return "";
      return `style="${cleaned}"`;
    },
  );
}

function isDarkColor(colorDecl: string): boolean {
  // Named dark colors
  if (/^color\s*:\s*(black|#000(000)?|#1[0-9a-f]{5}|#2[0-9a-f]{5})\s*$/.test(colorDecl))
    return true;
  // rgb(r, g, b) where all channels are below 100 (dark)
  const rgbMatch = colorDecl.match(/^color\s*:\s*rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)\s*$/);
  if (rgbMatch) {
    const [, r, g, b] = rgbMatch.map(Number);
    return r < 100 && g < 100 && b < 100;
  }
  // 3-digit or 6-digit hex colors that are dark (luminance heuristic)
  const hex3 = colorDecl.match(/^color\s*:\s*#([0-9a-f]{3})\s*$/);
  if (hex3) {
    const [rv, gv, bv] = hex3[1].split("").map((c) => parseInt(c + c, 16));
    return rv < 100 && gv < 100 && bv < 100;
  }
  const hex6 = colorDecl.match(/^color\s*:\s*#([0-9a-f]{6})\s*$/);
  if (hex6) {
    const rv = parseInt(hex6[1].slice(0, 2), 16);
    const gv = parseInt(hex6[1].slice(2, 4), 16);
    const bv = parseInt(hex6[1].slice(4, 6), 16);
    return rv < 100 && gv < 100 && bv < 100;
  }
  return false;
}
