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

import { UPDATE_STREAM_BY_DEPLOYMENT_PROFILE } from "@features/project-details/constants/productConstants";

/**
 * Trims a deployment profile and treats blank as absent.
 *
 * @param profile - Raw deployment profile from a version record.
 * @returns {string | null} The trimmed profile, or null when there is none.
 */
export function normalizeDeploymentProfile(
  profile: string | null | undefined,
): string | null {
  const trimmed = profile?.trim();
  return trimmed ? trimmed : null;
}

/**
 * Label for a product version that tells apart version records sharing one
 * version string, e.g. "4.6.0 · API Control Plane". A version with no profile
 * is just the version.
 *
 * @param version - Version string (e.g. "4.6.0").
 * @param profile - Deployment profile of that version record, if any.
 * @returns {string} Version label, with the profile appended when present.
 */
export function formatVersionWithProfile(
  version: string,
  profile?: string | null,
): string {
  const normalized = normalizeDeploymentProfile(profile);
  return normalized ? `${version} · ${normalized}` : version;
}

/**
 * Update-levels product key for a deployed product. The update-levels service
 * keeps a separate level stream per deployment profile of some products (for
 * API Manager: wso2am-acp, wso2am-universal-gw, wso2am-tm), so a profile with
 * its own stream reads levels from that stream. Anything else (no profile,
 * "All In One", a profile with no stream) keeps the product's own key.
 *
 * @param productKey - Product key the update-levels service knows (abbreviation, e.g. "wso2am").
 * @param profile - Deployment profile of the deployed product's version, if any.
 * @returns {string} Key to look the update levels up under.
 */
export function getUpdateStreamKey(
  productKey: string,
  profile?: string | null,
): string {
  const normalized = normalizeDeploymentProfile(profile);
  if (!normalized) return productKey;
  return (
    UPDATE_STREAM_BY_DEPLOYMENT_PROFILE[productKey]?.[normalized.toLowerCase()] ??
    productKey
  );
}
