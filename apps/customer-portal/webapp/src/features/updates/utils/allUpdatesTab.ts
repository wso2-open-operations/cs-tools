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

import type {
  AllUpdatesFilterValidation,
  AllUpdatesTabFilterState,
  ProductUpdateLevelEntry,
  ProductUpdateLevelsItem,
} from "@features/updates/types/updates";

/**
 * Validates All Updates tab filters before running search.
 *
 * @param f - Current filter state.
 * @returns {AllUpdatesFilterValidation} Valid range or invalid.
 */
export function validateAllUpdatesFilter(
  f: AllUpdatesTabFilterState,
): AllUpdatesFilterValidation {
  const start = Number(f.startLevel);
  const end = Number(f.endLevel);
  if (
    !f.productName ||
    !f.productVersion ||
    f.startLevel === "" ||
    f.endLevel === "" ||
    !Number.isFinite(start) ||
    !Number.isFinite(end) ||
    start < 0 ||
    end < 0 ||
    start > end
  ) {
    return { valid: false };
  }
  return { valid: true, start, end };
}

/**
 * Applies dependent-field resets when a single filter field changes.
 *
 * @param prev - Previous filter state.
 * @param field - Field being updated.
 * @param value - New value.
 * @returns {AllUpdatesTabFilterState} Next filter state.
 */
export function getNextAllUpdatesFilterAfterChange(
  prev: AllUpdatesTabFilterState,
  field: keyof AllUpdatesTabFilterState,
  value: string,
): AllUpdatesTabFilterState {
  const next: AllUpdatesTabFilterState = { ...prev, [field]: value };
  switch (field) {
    case "productName":
      return {
        ...next,
        productVersion: "",
        startLevel: "",
        endLevel: "",
      };
    case "productVersion":
      return { ...next, startLevel: "", endLevel: "" };
    case "startLevel":
      return { ...next, endLevel: "" };
    default:
      return next;
  }
}

/**
 * Derives unique product names from product update levels (GET /updates/product-update-levels).
 *
 * @param data - Product update levels response.
 * @returns {string[]} Sorted unique product names.
 */
export function getProductNamesFromProductLevels(
  data: ProductUpdateLevelsItem[] | undefined,
): string[] {
  if (!data?.length) return [];
  const names = [...new Set(data.map((d) => d.productName))];
  return names.sort();
}

/**
 * Derives version entries for a selected product.
 *
 * @param data - Product update levels response.
 * @param productName - Selected product.
 * @returns {ProductUpdateLevelEntry[]} Matching version entries.
 */
export function getVersionEntriesForProduct(
  data: ProductUpdateLevelsItem[] | undefined,
  productName: string,
): ProductUpdateLevelEntry[] {
  if (!data?.length || !productName) return [];
  const item = data.find((d) => d.productName === productName);
  return item?.productUpdateLevels ?? [];
}

/**
 * Returns sorted update levels for the selected product and version.
 *
 * @param data - Product update levels response.
 * @param productName - Selected product.
 * @param productVersion - Selected version (productBaseVersion).
 * @returns {number[]} Sorted update levels.
 */
export function getUpdateLevelsForProductVersion(
  data: ProductUpdateLevelsItem[] | undefined,
  productName: string,
  productVersion: string,
): number[] {
  const entries = getVersionEntriesForProduct(data, productName);
  const entry = entries.find((e) => e.productBaseVersion === productVersion);
  if (!entry?.updateLevels?.length) return [];
  return [...entry.updateLevels].sort((a, b) => a - b);
}
