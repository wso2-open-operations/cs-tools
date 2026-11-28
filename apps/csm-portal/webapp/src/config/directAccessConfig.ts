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
/**
 * Whether the portal may be opened at its own address, from
 * `window.config.CSM_PORTAL_DIRECT_ACCESS_ENABLED` (a boolean or the string
 * "true"/"false"). On unless it is `false`, so a deployment that does not set
 * the key behaves as it always has. Read at call time.
 *
 * Turned off where the portal is reached through a host application instead.
 * Inside a host the key is never consulted, so the portal always works there.
 * It steers people to the host and is not an access control: the portal's files
 * stay public, because the host loads them from this same deployment.
 */
export function directAccessEnabled(): boolean {
  const raw = window.config?.CSM_PORTAL_DIRECT_ACCESS_ENABLED;
  if (typeof raw === "string") return raw.trim().toLowerCase() !== "false";
  return raw !== false;
}

/**
 * Where the portal is mounted in the host application, e.g.
 * "https://host.example/csm", from `window.config.CSM_PORTAL_HOST_APP_URL`.
 * Returns "" when it is unset or not an absolute http(s) URL. Read at call time.
 */
export function hostAppUrl(): string {
  const raw = window.config?.CSM_PORTAL_HOST_APP_URL;
  if (typeof raw !== "string" || !raw.trim()) return "";
  try {
    const url = new URL(raw.trim());
    if (url.protocol !== "https:" && url.protocol !== "http:") return "";
    return url.href.replace(/\/+$/, "");
  } catch {
    return "";
  }
}

/** The parts of the current address that say which page is open. */
export interface PageAddress {
  origin: string;
  pathname: string;
  search: string;
  hash: string;
}

/**
 * The same page inside the host application, for someone who opened the portal
 * directly while direct access is off. Null when there is nowhere to send them:
 * no usable host URL, or one on this portal's own origin, which would only
 * lead back here.
 */
export function hostAppAddressFor(page: PageAddress): string | null {
  const base = hostAppUrl();
  if (!base || new URL(base).origin === page.origin) return null;
  const path = page.pathname === "/" ? "" : page.pathname;
  return `${base}${path}${page.search}${page.hash}`;
}
