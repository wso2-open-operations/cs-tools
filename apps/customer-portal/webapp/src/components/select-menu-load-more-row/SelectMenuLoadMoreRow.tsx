// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import { CircularProgress, MenuItem } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * Sentinel `Select` / `TextField select` value for the load-more row only.
 * Do not use as real form state.
 */
export const SELECT_MENU_LOAD_MORE_ROW_VALUE =
  "__customer_portal_select_load_more__";

export interface SelectMenuLoadMoreRowProps {
  /** When true, renders a disabled row with a spinner under the last real option. */
  visible: boolean;
}

/**
 * Footer row for paginated select menus while the next page is loading.
 *
 * @param {SelectMenuLoadMoreRowProps} props - visibility flag.
 * @returns {JSX.Element | null} Disabled menu row with CircularProgress, or null.
 */
export function SelectMenuLoadMoreRow({
  visible,
}: SelectMenuLoadMoreRowProps): JSX.Element | null {
  if (!visible) {
    return null;
  }

  return (
    <MenuItem
      disableRipple
      disabled
      value={SELECT_MENU_LOAD_MORE_ROW_VALUE}
      sx={{
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        py: 1.5,
        minHeight: 48,
        cursor: "default",
        opacity: 1,
        "&.Mui-disabled": { opacity: 1 },
      }}
    >
      <CircularProgress size={22} />
    </MenuItem>
  );
}
