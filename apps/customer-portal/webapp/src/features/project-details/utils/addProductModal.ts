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

/** Default form state for Add Product modal. */
export const ADD_PRODUCT_MODAL_INITIAL_FORM = {
  productId: "",
  versionId: "",
  cores: "",
  tps: "",
  description: "",
} as const;

export type AddProductModalFormState = {
  productId: string;
  versionId: string;
  cores: string;
  tps: string;
  description: string;
};

/**
 * Parses a numeric string from the add-product form; returns undefined if empty or invalid.
 *
 * @param value - Raw input string.
 * @returns A finite number or undefined.
 */
export function parseValidNumber(value: string): number | undefined {
  if (!value || !value.trim()) return undefined;
  const num = Number(value);
  return Number.isFinite(num) ? num : undefined;
}
