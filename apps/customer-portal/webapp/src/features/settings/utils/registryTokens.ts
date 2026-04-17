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
  REGISTRY_TOKEN_DATE_LOCALE,
  REGISTRY_TOKEN_DESC_SYSTEM_GENERATED,
  REGISTRY_TOKEN_TIMESTAMP_NEVER_LABEL,
  SETTINGS_NULL_PLACEHOLDER,
} from "@features/settings/constants/settingsConstants";
import {
  RegistryTokenDisplayStatus,
} from "@features/settings/types/settings";
import type { RegistryToken } from "@features/settings/types/registryTokens";

/**
 * Derives display status from token fields.
 *
 * @param token - Registry token row.
 * @returns {RegistryTokenDisplayStatus} Active, Expired, or Revoked.
 */
export function getRegistryTokenDisplayStatus(
  token: RegistryToken,
): RegistryTokenDisplayStatus {
  if (token.disable) return RegistryTokenDisplayStatus.Revoked;
  if (token.expiresAt && token.expiresAt > 0) {
    const nowSec = Math.floor(Date.now() / 1000);
    if (token.expiresAt < nowSec) return RegistryTokenDisplayStatus.Expired;
  }
  return RegistryTokenDisplayStatus.Active;
}

/**
 * Chip color for MUI `Chip` from token status.
 *
 * @param status - Display status string.
 * @returns Chip color prop.
 */
export function getRegistryTokenStatusChipColor(
  status: string,
): "success" | "error" | "default" | "warning" {
  switch (status) {
    case RegistryTokenDisplayStatus.Active:
      return "success";
    case RegistryTokenDisplayStatus.Expired:
      return "error";
    case RegistryTokenDisplayStatus.Revoked:
      return "default";
    default:
      return "warning";
  }
}

/**
 * Formats unix seconds as DD/MM/YYYY.
 *
 * @param ts - Unix timestamp in seconds.
 * @returns Human-readable date or "Never".
 */
export function formatRegistryTokenTimestamp(ts?: number): string {
  if (!ts || ts <= 0) return REGISTRY_TOKEN_TIMESTAMP_NEVER_LABEL;
  const d = new Date(ts * 1000);
  return d.toLocaleDateString(REGISTRY_TOKEN_DATE_LOCALE, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/**
 * Formats ISO date string to DD/MM/YYYY.
 *
 * @param iso - ISO date from API.
 * @returns Formatted date or null placeholder.
 */
export function formatRegistryTokenIsoDate(iso?: string | null): string {
  if (!iso) return SETTINGS_NULL_PLACEHOLDER;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return SETTINGS_NULL_PLACEHOLDER;
  return d.toLocaleDateString(REGISTRY_TOKEN_DATE_LOCALE, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/**
 * Parses backend metadata description into a short summary.
 *
 * @param rawDesc - Raw description string.
 * @returns Display string.
 */
export function formatRegistryTokenDescription(rawDesc?: string): string {
  if (!rawDesc) return SETTINGS_NULL_PLACEHOLDER;

  if (rawDesc.includes("##")) {
    const parts = rawDesc.split("##");

    if (parts.length >= 5) {
      const tokenType = parts[2];
      const createdFor = parts[3];
      return `${tokenType} token for ${createdFor}`;
    }

    return REGISTRY_TOKEN_DESC_SYSTEM_GENERATED;
  }

  return rawDesc;
}

/**
 * Whether the token expires within the next N days (still active).
 *
 * @param token - Registry token.
 * @param days - Window in days.
 * @returns True if expiring soon.
 */
/**
 * @param base - Tab title without count (e.g. "User Tokens").
 * @param count - Number of tokens in that tab.
 * @returns Label like `User Tokens (3)`.
 */
export function formatRegistrySubTabLabel(base: string, count: number): string {
  return `${base} (${count})`;
}

export function registryTokenExpiresWithinDays(
  token: RegistryToken,
  days: number,
): boolean {
  if (!token.expiresAt || token.expiresAt <= 0) return false;
  if (token.disable) return false;
  const nowSec = Math.floor(Date.now() / 1000);
  if (token.expiresAt < nowSec) return false;
  const thresholdSec = nowSec + days * 86400;
  return token.expiresAt <= thresholdSec;
}
