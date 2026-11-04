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


import type {
  CatalogReleaseLine,
  CatalogSupportStatus,
  CatalogVersion,
  CatalogVersionWire,
} from "@features/csm-products/types/productCatalog";

/**
 * The one place the products API's wire format is converted to the shapes the
 * Products section works with: a date field arrives as an RFC 3339 timestamp
 * (`2024-01-15T00:00:00Z`) and becomes `"2024-01-15"`; the support status
 * arrives upper case and becomes lower case. Writes need no conversion:
 * the API takes `YYYY-MM-DD` dates and either case.
 */

const SUPPORT_STATUSES: readonly CatalogSupportStatus[] = ["available", "extended", "deprecated", "discontinued"];

/** The calendar date of a date field. The API sends dates at midnight UTC, so the first ten characters are the date. */
export function wireDate(value: string | null | undefined): string | null {
  if (!value) return null;
  return /^\d{4}-\d{2}-\d{2}/.test(value) ? value.slice(0, 10) : null;
}

function wireEnum<T extends string>(value: string | null | undefined, allowed: readonly T[]): T | null {
  const lower = value?.toLowerCase();
  return lower && (allowed as readonly string[]).includes(lower) ? (lower as T) : null;
}

export function fromWireVersion(wire: CatalogVersionWire): CatalogVersion {
  return {
    ...wire,
    currentSupportStatus: wireEnum(wire.currentSupportStatus, SUPPORT_STATUSES),
    releaseDate: wireDate(wire.releaseDate),
    earliestPossibleSupportEolDate: wireDate(wire.earliestPossibleSupportEolDate),
    supportEolDate: wireDate(wire.supportEolDate),
    supportEolAnnouncementDate: wireDate(wire.supportEolAnnouncementDate),
    tags: wire.tags ?? [],
  };
}

export function fromWireReleaseLine(wire: CatalogReleaseLine): CatalogReleaseLine {
  return { ...wire, startDate: wireDate(wire.startDate) };
}
