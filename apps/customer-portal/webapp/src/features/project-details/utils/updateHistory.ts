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

import { alpha, type Theme } from "@wso2/oxygen-ui";

/**
 * User-visible message for failed update-history mutations.
 *
 * @param error - Thrown value from save handler.
 * @returns Short error string.
 */
export function getUpdateHistoryErrorMessage(error: unknown): string {
  return error instanceof Error
    ? error.message
    : "Failed to save update history.";
}

/**
 * Background fill for a single update history entry card.
 *
 * @param theme - MUI theme.
 * @returns CSS color string.
 */
export function getUpdateHistoryEntryBackground(theme: Theme): string {
  return alpha(
    theme.palette.text.secondary,
    theme.palette.mode === "dark" ? 0.22 : 0.12,
  );
}

/**
 * Outline color for history cards and dividers in light / dark mode.
 *
 * @param theme - MUI theme.
 * @returns CSS color string.
 */
export function getUpdateHistoryOutlineColor(theme: Theme): string {
  switch (theme.palette.mode) {
    case "dark":
      return alpha(theme.palette.common.white, 0.28);
    default:
      return alpha(theme.palette.common.black, 0.12);
  }
}
