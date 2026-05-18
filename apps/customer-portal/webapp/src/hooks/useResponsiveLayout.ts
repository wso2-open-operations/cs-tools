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

import { useMediaQuery, useTheme } from "@wso2/oxygen-ui";

/**
 * Mid-size touch devices (sm–md, coarse pointer), e.g. iPad Mini portrait.
 * Used for overlay sidebar and hiding cramped dashboard tables — not phones or
 * resized desktop windows (fine pointer).
 *
 * @returns {boolean} True for mid-size touch tablet viewports.
 */
export function useIsMidSizeTouchViewport(): boolean {
  const theme = useTheme();
  const isMidSizeWidth = useMediaQuery(theme.breakpoints.between("sm", "md"));
  const isCoarsePointer = useMediaQuery("(pointer: coarse)");

  return isMidSizeWidth && isCoarsePointer;
}

/**
 * Viewports below lg use the stacked header: row 1 brand/actions, row 2 project
 * switcher and search. Covers phones and tablets; laptops (e.g. MacBook Air at
 * 1280px+) use the single-row header.
 *
 * @returns {boolean} True when the stacked (two-row) header layout applies.
 */
export function useIsStackedHeaderLayout(): boolean {
  const theme = useTheme();
  return useMediaQuery(theme.breakpoints.down("lg"));
}
