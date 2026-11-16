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

/** Company name used when `CUSTOMER_PORTAL_COMPANY_NAME` is not set. */
export const DEFAULT_COMPANY_NAME = "WSO2";

/**
 * A trimmed, non-empty string from `window.config`, else undefined.
 *
 * @param value - The raw config value.
 * @returns {string | undefined} The trimmed value, or undefined when unset or blank.
 */
export function configuredString(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

/**
 * Company name shown in the portal's labels and PDF reports
 * (`CUSTOMER_PORTAL_COMPANY_NAME`), {@link DEFAULT_COMPANY_NAME} when unset or
 * blank. Read at call time.
 *
 * @returns {string} The company name.
 */
export function companyName(): string {
  return (
    configuredString(window.config?.CUSTOMER_PORTAL_COMPANY_NAME) ??
    DEFAULT_COMPANY_NAME
  );
}

/**
 * Whether `CUSTOMER_PORTAL_COMPANY_NAME` is set.
 *
 * @returns {boolean} True when a company name is configured.
 */
export function hasConfiguredCompanyName(): boolean {
  return configuredString(window.config?.CUSTOMER_PORTAL_COMPANY_NAME) !== undefined;
}
