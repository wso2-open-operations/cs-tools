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

import { CaseType } from "@features/support/constants/supportConstants";
import type { AnnouncementFilterValues } from "@features/support/constants/supportConstants";
import type { CaseMetadataResponse } from "@features/support/types/cases";
import type { CaseSearchRequest } from "@features/support/types/cases";
import { mapSeverityToDisplay } from "@features/support/utils/support";
import type { FilterDefinition } from "@components/list-view/ListFiltersPanel";
import type { SortOrder } from "@/types/common";
import {
  AnnouncementFilterMetadataKey,
  AnnouncementSortField,
  type AnnouncementFilterDefinition,
  type AnnouncementFilterOption,
} from "@features/announcements/types/announcements";
import {
  ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES,
  ANNOUNCEMENTS_CLEAR_FILTERS_LABEL,
} from "@features/announcements/constants/announcementsConstants";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

/**
 * Builds the case search payload for the announcements list (announcement case type only).
 *
 * @param filters - Filter field values from the UI.
 * @param searchTerm - Raw search string.
 * @param sortOrder - List sort direction.
 * @returns Search request body without pagination (offset/limit passed separately).
 */
export function buildAnnouncementCaseSearchRequest(
  filters: AnnouncementFilterValues,
  searchTerm: string,
  sortOrder: SortOrder,
): Omit<CaseSearchRequest, "pagination"> {
  return {
    filters: {
      caseTypes: [CaseType.ANNOUNCEMENT],
      statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
      searchQuery: searchTerm.trim() || undefined,
    },
    sortBy: {
      field: AnnouncementSortField.CreatedOn,
      order: sortOrder,
    },
  };
}

/**
 * Total page count for list pagination (at least one page when empty).
 *
 * @param totalRecords - API total count.
 * @param pageSize - Page size.
 * @returns Page count (minimum 1).
 */
export function getAnnouncementTotalPages(
  totalRecords: number,
  pageSize: number,
): number {
  return Math.max(1, Math.ceil(totalRecords / pageSize));
}

/**
 * Options for `ListFiltersPanel.resolveOptions` from project filter metadata.
 *
 * @param def - Filter definition (from `ANNOUNCEMENT_FILTER_DEFINITIONS`).
 * @param filterMetadata - Project filters API response.
 * @returns Dropdown options; case-state options restricted to allowed values.
 */
export function resolveAnnouncementListFilterOptions(
  def: FilterDefinition,
  filterMetadata: CaseMetadataResponse | undefined,
): AnnouncementFilterOption[] {
  const raw = filterMetadata?.[def.metadataKey as keyof CaseMetadataResponse];
  if (!Array.isArray(raw)) return [];
  const options: AnnouncementFilterOption[] = raw.map(
    (item: { label: string; id: string }) => ({
      label: item.label,
      value: def.useLabelAsValue ? item.label : item.id,
    }),
  );
  switch (def.metadataKey) {
    case AnnouncementFilterMetadataKey.CaseStates:
      return options.filter((o) =>
        (ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES as readonly string[]).includes(
          o.value,
        ),
      );
    case AnnouncementFilterMetadataKey.Severities:
      return options;
    default:
      return options;
  }
}

/**
 * Builds select options for `AnnouncementsFilters` (label transforms + case-state filter).
 *
 * @param def - Filter definition entry.
 * @param filterMetadata - Project filters API response.
 * @returns Options for the filter Select.
 */
export function buildAnnouncementsFilterSelectOptions(
  def: AnnouncementFilterDefinition,
  filterMetadata: CaseMetadataResponse | undefined,
): AnnouncementFilterOption[] {
  const metadataOptions = filterMetadata?.[def.metadataKey];
  let options: AnnouncementFilterOption[] = Array.isArray(metadataOptions)
    ? metadataOptions.map((item: { label: string; id: string }) => {
        let label: string;
        switch (def.metadataKey) {
          case AnnouncementFilterMetadataKey.Severities:
            label = mapSeverityToDisplay(item.label);
            break;
          case AnnouncementFilterMetadataKey.CaseStates:
          default:
            label = item.label;
            break;
        }
        return {
          label,
          value: def.useLabelAsValue ? item.label : item.id,
        };
      })
    : [];

  switch (def.metadataKey) {
    case AnnouncementFilterMetadataKey.CaseStates:
      options = options.filter((option) =>
        (ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES as readonly string[]).includes(
          option.value,
        ),
      );
      break;
    case AnnouncementFilterMetadataKey.Severities:
    default:
      break;
  }

  return options;
}

/**
 * Label for the search bar clear action including active refinement count.
 *
 * @param activeFiltersCount - From `countListSearchAndFilters`.
 * @returns e.g. `Clear Filters (2)`.
 */
export function formatAnnouncementsClearFiltersButtonLabel(
  activeFiltersCount: number,
): string {
  return `${ANNOUNCEMENTS_CLEAR_FILTERS_LABEL} (${activeFiltersCount})`;
}

/**
 * Normalizes announcement HTML:
 * - Converts empty `<code>` blocks to a newline (`<br />`) so we do not render useless whitespace.
 * - Converts `<code><n/><code/>`-style placeholders (and bare `<n/>`) into newlines.
 *
 * @param html - Raw HTML from backend.
 * @returns Normalized HTML.
 */
export function normalizeAnnouncementDescriptionHtml(html: string): string {
  let normalized = html;

  normalized = normalized.replace(
    /<p>\s*<code>\s*(?:&nbsp;|\s|<br\s*\/?>)*<\/code>\s*<\/p>/gi,
    "<br />",
  );

  normalized = normalized.replace(
    /<code>\s*(?:&nbsp;|\s|<br\s*\/?>)*<\/code>/gi,
    "<br />",
  );

  normalized = normalized.replace(
    /<code>\s*<n\s*\/>\s*<code\s*\/>\s*/gi,
    "<br />",
  );

  normalized = normalized.replace(/<code>\s*<n\s*\/>\s*<\/code>/gi, "<br />");

  normalized = normalized.replace(/<n\s*\/>/gi, "<br />");

  return normalized;
}

/**
 * True when normalized HTML has no visible text (after stripping tags and `<br>`).
 *
 * @param html - Raw or normalized HTML.
 * @returns Whether the description should show the empty placeholder.
 */
export function isAnnouncementDescriptionEffectivelyEmpty(
  html: string,
): boolean {
  const normalizedHtml = normalizeAnnouncementDescriptionHtml(html);
  const normalizedText = normalizedHtml
    .replace(/<br\s*\/?>/gi, "")
    .replace(/&nbsp;/gi, " ")
    .replace(/<[^>]*>/g, "")
    .trim();
  return !normalizedText;
}

/**
 * Formats an API date string (ISO or ServiceNow-style `YYYY-MM-DD HH:mm:ss`) for
 * list and detail views using the runtime locale.
 *
 * @param raw - Raw timestamp from the API.
 * @returns Formatted date/time or `"--"` when missing or unparsable.
 */
export function formatAnnouncementDateDisplay(
  raw: string | null | undefined,
): string {
  if (raw == null || String(raw).trim() === "") {
    return "--";
  }
  const formatted = formatBackendTimestampForDisplay(String(raw), {
    dateStyle: "medium",
    timeStyle: "short",
  });
  return formatted ?? "--";
}
