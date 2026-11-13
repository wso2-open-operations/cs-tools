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

import {
  companyName,
  configuredString,
  DEFAULT_COMPANY_NAME,
  hasConfiguredCompanyName,
} from "@config/brandingConfig";

/** Company name the PDF footers print when `CUSTOMER_PORTAL_COMPANY_NAME` is not set. */
export const DEFAULT_REPORT_COMPANY_NAME = DEFAULT_COMPANY_NAME;

/** What follows the company name in the footer line. */
export const REPORT_SUPPORT_LINE_SUFFIX = " Support 24x7";

/**
 * Company name for the PDF report footers (`CUSTOMER_PORTAL_COMPANY_NAME`),
 * {@link DEFAULT_REPORT_COMPANY_NAME} when unset or blank. Read at call time.
 *
 * @returns {string} The company name.
 */
export function reportCompanyName(): string {
  return companyName();
}

/**
 * Whether `CUSTOMER_PORTAL_COMPANY_NAME` is set. A report that prints no company
 * today only adds one when it is.
 *
 * @returns {boolean} True when a company name is configured.
 */
export function hasConfiguredReportCompanyName(): boolean {
  return hasConfiguredCompanyName();
}

/**
 * Footer line of the PDF reports: "<company> Support 24x7", which is
 * "WSO2 Support 24x7" when no company name is configured.
 *
 * @returns {string} The footer text.
 */
export function reportSupportLine(): string {
  return `${reportCompanyName()}${REPORT_SUPPORT_LINE_SUFFIX}`;
}

/**
 * Logo for the PDF report footers. The reports print on white, so the light-mode
 * logo (`CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL`) is used, and the dark-mode one
 * (`CUSTOMER_PORTAL_LOGO_DARK_MODE_URL`) only when that is not set. Undefined
 * when neither is set: the footer then has no logo, as before. Read at call time.
 *
 * @returns {string | undefined} The logo URL, or undefined.
 */
export function reportLogoUrl(): string | undefined {
  return (
    configuredString(window.config?.CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL) ??
    configuredString(window.config?.CUSTOMER_PORTAL_LOGO_DARK_MODE_URL)
  );
}
