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

import type { SavePortalBanner } from "@features/csm-admin/banners/usePortalBanners";

/** The datetime-local field, as the timestamp the banner API stores. Empty stays null. */
export function toApiTimestamp(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  const date = new Date(trimmed);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
}

/** An API timestamp, as the value a datetime-local field can show. */
export function fromApiTimestamp(value: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (part: number): string => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** The same check the banner API applies: an end must be later than a start. */
export function scheduleError(startsOn: string, expiresOn: string): string {
  const startsAt = toApiTimestamp(startsOn);
  const expiresAt = toApiTimestamp(expiresOn);
  if ((startsOn.trim() !== "" && startsAt === null) || (expiresOn.trim() !== "" && expiresAt === null)) {
    return "Enter a valid start and end.";
  }
  if (startsAt && expiresAt && new Date(expiresAt).getTime() <= new Date(startsAt).getTime()) {
    return "Ends must be after starts.";
  }
  return "";
}

/**
 * Before the start, a banner stays a draft. After the end, it is unpublished.
 */
export function statusOnSchedule(
  status: SavePortalBanner["status"],
  startsOn: string,
  expiresOn: string,
  now: Date,
): SavePortalBanner["status"] {
  const expiresAt = toApiTimestamp(expiresOn);
  const startsAt = toApiTimestamp(startsOn);
  if (expiresAt && new Date(expiresAt).getTime() <= now.getTime()) return "unpublished";
  if (startsAt && new Date(startsAt).getTime() > now.getTime()) return "draft";
  return status;
}

/** A draft or unpublished status the dates require, or null when the engineer may choose. */
export function scheduleLocksStatus(startsOn: string, expiresOn: string, now: Date): SavePortalBanner["status"] | null {
  const forced = statusOnSchedule("published", startsOn, expiresOn, now);
  return forced === "published" ? null : forced;
}

/**
 * The body Save will send to POST /portal-banners or PUT /portal-banners/{id}.
 * An unpublished banner is stored with both portals off.
 */
export function toSavePortalBanner(input: {
  status: SavePortalBanner["status"];
  sortOrder: number;
  visibleInCustomerPortal: boolean;
  visibleInCsmPortal: boolean;
  html: string;
  startsOn: string;
  expiresOn: string;
}): SavePortalBanner {
  const unpublished = input.status === "unpublished";
  return {
    status: input.status,
    sortOrder: input.sortOrder,
    visibleInCustomerPortal: unpublished ? false : input.visibleInCustomerPortal,
    visibleInCsmPortal: unpublished ? false : input.visibleInCsmPortal,
    html: input.html,
    startsAt: toApiTimestamp(input.startsOn),
    expiresAt: toApiTimestamp(input.expiresOn),
  };
}
