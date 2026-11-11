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

import { formatLimitMb } from "@config/attachmentLimitsConfig";

/** Max height for Select menu lists backed by paginated APIs (~10 rows visible, scrollbar when more). */
export const PAGINATED_SELECT_MENU_MAX_HEIGHT_PX = 320;

/** Standard empty-state label when a dropdown has no options after load. */
export const EMPTY_DROPDOWN_PLACEHOLDER = "Not available";

/** Generic placeholder for empty/null value displays. */
export const NULL_PLACEHOLDER = "--";

// Auto-dismiss timeout in milliseconds (error and success banners).
export const ERROR_BANNER_TIMEOUT_MS = 5000;

// Footer height in pixels.
export const FOOTER_HEIGHT_PX = 64;

// Header height in pixels.
export const HEADER_HEIGHT_PX = 64;

/** Overlay sidebar drawer width; matches Oxygen Sidebar default expanded width. */
export const SIDEBAR_DRAWER_WIDTH_PX = 250;

// Vertical gap in pixels.
export const BANNER_FOOTER_GAP_PX = 24;

// Vertical gap from the top in pixels.
export const BANNER_HEADER_GAP_PX = 24;

// Horizontal gap in pixels.
export const BANNER_RIGHT_GAP_PX = 24;

export const ERROR_UNSUPPORTED_TYPE =
  "This image format can't be pasted inline. Only JPEG, PNG, and WebP images support inline display. You can still upload this file as an attachment.";
/** Most inline images one comment or description may hold. */
export const INLINE_IMAGE_MAX_COUNT = 5;
/** @deprecated Use INLINE_IMAGE_MAX_COUNT. */
export const MAX_COMMENT_IMAGES = INLINE_IMAGE_MAX_COUNT;
/** The refusal for adding an image to a draft that already holds the maximum. */
export function imageCountExceededMessage(): string {
  return `A comment or description can have at most ${INLINE_IMAGE_MAX_COUNT} images.`;
}
/** The refusal for adding an image that takes the draft's images over the total limit. */
export function imageTotalSizeExceededMessage(totalMaxBytes: number): string {
  return `Images in one comment or description can't exceed ${formatLimitMb(totalMaxBytes)} MB in total.`;
}
/** The refusal for an image over the configured inline image limit. */
export function imageSizeExceededMessage(totalMaxBytes: number): string {
  return imageTotalSizeExceededMessage(totalMaxBytes);
}
