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

/** Product name shown beside the logo in the header and the browser tab when unconfigured. */
export const DEFAULT_APP_NAME = "CSM Portal";

/** Header/tab-icon logo for the light colour scheme when unconfigured. */
export const DEFAULT_LOGO_LIGHT_MODE_URL = "/logo-dark.svg";

/** Header/tab-icon logo for the dark colour scheme when unconfigured. */
export const DEFAULT_LOGO_DARK_MODE_URL = "/logo-white.svg";

/** The trimmed value, or undefined when it is unset, blank, or not a string. */
function optionalString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

/**
 * Anchors a bare configured logo path ("logos/acme.svg") to the site root so
 * it resolves the same way on every route, not relative to whatever page
 * happens to be open. A path that already starts with "/" or carries a
 * scheme ("https:", "data:", ...) is returned unchanged.
 */
function resolveLogoUrl(url: string | undefined): string | undefined {
  if (!url) return url;
  if (url.startsWith("/") || /^[a-z][a-z0-9+.-]*:/i.test(url)) return url;
  return `/${url.replace(/^\.\//, "")}`;
}

/**
 * Product name shown beside the logo in the header and in the browser tab,
 * from `window.config.CSM_PORTAL_APP_NAME`. Defaults to {@link DEFAULT_APP_NAME}
 * when unset. Read at call time so a test can set `window.config` per case.
 */
export function appName(): string {
  return optionalString(window.config?.CSM_PORTAL_APP_NAME) ?? DEFAULT_APP_NAME;
}

/**
 * Logo for the light colour scheme (also the light-mode tab icon), from
 * `window.config.CSM_PORTAL_LOGO_LIGHT_MODE_URL`. Defaults to
 * {@link DEFAULT_LOGO_LIGHT_MODE_URL} when unset. Read at call time.
 */
export function logoLightModeUrl(): string {
  return (
    resolveLogoUrl(optionalString(window.config?.CSM_PORTAL_LOGO_LIGHT_MODE_URL)) ??
    DEFAULT_LOGO_LIGHT_MODE_URL
  );
}

/**
 * Logo for the dark colour scheme (also the dark-mode tab icon), from
 * `window.config.CSM_PORTAL_LOGO_DARK_MODE_URL`. When that key is unset, falls
 * back to whatever the light-mode logo resolves to (so setting only the light
 * logo covers both colour schemes), and only then to
 * {@link DEFAULT_LOGO_DARK_MODE_URL}. Read at call time.
 */
export function logoDarkModeUrl(): string {
  return (
    resolveLogoUrl(optionalString(window.config?.CSM_PORTAL_LOGO_DARK_MODE_URL)) ??
    resolveLogoUrl(optionalString(window.config?.CSM_PORTAL_LOGO_LIGHT_MODE_URL)) ??
    DEFAULT_LOGO_DARK_MODE_URL
  );
}
