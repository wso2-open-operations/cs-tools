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
