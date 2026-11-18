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

/** Billing email used when `CUSTOMER_PORTAL_BILLING_EMAIL` is not set. */
export const DEFAULT_BILLING_EMAIL = "billing@wso2.com";

/**
 * One plain address: an unquoted local part (no whitespace and none of RFC
 * 5322's specials, so no "mailto:" prefix either) whose dots each sit between
 * two non-empty parts, "@", and a hostname.
 */
const PLAIN_EMAIL =
  /^[^\s()<>[\]:;@\\,".]+(\.[^\s()<>[\]:;@\\,".]+)*@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*$/;

/**
 * Billing team's email address shown on the suspended-project notice
 * (`CUSTOMER_PORTAL_BILLING_EMAIL`), {@link DEFAULT_BILLING_EMAIL} when unset,
 * blank or not a plain address. Read at call time.
 *
 * @returns {string} The billing email address.
 */
export function billingEmail(): string {
  const configured = configuredString(window.config?.CUSTOMER_PORTAL_BILLING_EMAIL);
  return configured && PLAIN_EMAIL.test(configured) ? configured : DEFAULT_BILLING_EMAIL;
}

/**
 * `mailto:` link for {@link billingEmail}. The local part is percent-encoded:
 * `#`, `?` and `%` are legal in it but would otherwise start the link's
 * fragment or query, or be read as an escape (RFC 6068). The domain is a plain
 * hostname, so it needs none.
 *
 * @returns {string} The mailto link.
 */
export function billingEmailHref(): string {
  const address = billingEmail();
  const at = address.lastIndexOf("@");
  return `mailto:${encodeURIComponent(address.slice(0, at))}@${address.slice(at + 1)}`;
}

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
export const resolveLogoUrl = (url: string | undefined): string | undefined => {
  if (!url) return url;
  if (url.startsWith("/") || /^[a-z][a-z0-9+.-]*:/i.test(url)) return url;
  return `/${url.replace(/^\.\//, "")}`;
};
