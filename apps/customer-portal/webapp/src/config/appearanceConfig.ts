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

import { companyName } from "@config/brandingConfig";

const DEFAULT_PORTAL_NAME = "Customer Portal";
const COMPANY_LEGAL_SUFFIX = "LLC";
const DEFAULT_LOGO_LIGHT_MODE_URL = "/logo-dark.svg";
const DEFAULT_LOGO_DARK_MODE_URL = "/logo-white.svg";

/**
 * Returns the trimmed config value, or undefined when it is unset or blank.
 *
 * @param {unknown} value - Raw `window.config` value.
 * @returns {string | undefined} The trimmed value, or undefined.
 */
const optionalString = (value: unknown): string | undefined =>
  typeof value === "string" && value.trim() ? value.trim() : undefined;

/**
 * Resolves a configured logo URL so a bare path such as `logos/acme.svg` is
 * read from the site root rather than relative to the current page, which
 * would break on deep links like `/projects/1/dashboard`. Absolute paths and
 * URLs with a scheme (`https:`, `data:`, ...) or a host (`//cdn/...`) are
 * returned unchanged.
 *
 * @param {string | undefined} url - Trimmed configured URL.
 * @returns {string | undefined} The URL, anchored to the site root when relative.
 */
const resolveLogoUrl = (url: string | undefined): string | undefined => {
  if (!url) return url;
  if (url.startsWith("/") || /^[a-z][a-z0-9+.-]*:/i.test(url)) return url;
  return `/${url.replace(/^\.\//, "")}`;
};

const logoLightModeUrl = resolveLogoUrl(
  optionalString(window.config?.CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL),
);

/**
 * User-visible name and logo settings, read from `window.config` with the WSO2 defaults.
 * A deployment that supplies only `CUSTOMER_PORTAL_LOGO_LIGHT_MODE_URL` uses it in both
 * colour schemes.
 */
export const appearanceConfig = {
  /** Product name shown beside the logo in the header and in the browser tab. */
  name:
    optionalString(window.config?.CUSTOMER_PORTAL_APP_NAME) ??
    DEFAULT_PORTAL_NAME,
  /** Company name in the footer copyright line, "<name> LLC". */
  companyName: `${companyName()} ${COMPANY_LEGAL_SUFFIX}`,
  /** Logo for the light colour scheme (also the tab icon). */
  logoLightModeUrl: logoLightModeUrl ?? DEFAULT_LOGO_LIGHT_MODE_URL,
  /** Logo for the dark colour scheme (also the tab icon). */
  logoDarkModeUrl:
    resolveLogoUrl(
      optionalString(window.config?.CUSTOMER_PORTAL_LOGO_DARK_MODE_URL),
    ) ??
    logoLightModeUrl ??
    DEFAULT_LOGO_DARK_MODE_URL,
} as const;
