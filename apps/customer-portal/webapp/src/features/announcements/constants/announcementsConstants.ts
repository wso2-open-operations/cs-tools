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
  AnnouncementSortField,
  type AnnouncementSortOption,
} from "@features/announcements/types/announcements";

/** Page size for announcements list (aligned with `useGetProjectCasesPage`). */
export const ANNOUNCEMENTS_PAGE_SIZE = 10;

/** List page title. */
export const ANNOUNCEMENTS_PAGE_TITLE = "Announcements";

/** List page subtitle. */
export const ANNOUNCEMENTS_PAGE_DESCRIPTION =
  "View and manage announcements for your project";

/** Back control label (list + detail). */
export const ANNOUNCEMENTS_BACK_LABEL = "Back";

/** Search field placeholder (list search bar + `ListSearchBar`). */
export const ANNOUNCEMENTS_SEARCH_PLACEHOLDER = "Search announcements...";

/** Entity label for results bar and pagination copy. */
export const ANNOUNCEMENTS_LIST_ENTITY_LABEL = "announcements";

/** Sort row: label for created date column. */
export const ANNOUNCEMENTS_SORT_CREATED_LABEL = "Created date";

/** Sort row: label for state column. */
export const ANNOUNCEMENTS_SORT_STATE_LABEL = "Status";

/** Options passed to `ListResultsBar`. */
export const ANNOUNCEMENTS_SORT_FIELD_OPTIONS: AnnouncementSortOption[] = [
  {
    value: AnnouncementSortField.CreatedOn,
    label: ANNOUNCEMENTS_SORT_CREATED_LABEL,
    kind: "chronological",
  },
  {
    value: AnnouncementSortField.State,
    label: ANNOUNCEMENTS_SORT_STATE_LABEL,
    kind: "ordinal",
  },
];

/** Empty list when no filters/search. */
export const ANNOUNCEMENTS_EMPTY_MESSAGE_DEFAULT = "No announcements yet.";

/** Empty list when filters/search yield no rows. */
export const ANNOUNCEMENTS_EMPTY_MESSAGE_REFINED =
  "No announcements found. Try adjusting your filters or search query.";

/** Filters toggle when no active refinements. */
export const ANNOUNCEMENTS_FILTERS_BUTTON_LABEL = "Filters";

/** Prefix for clear action; append count in UI. */
export const ANNOUNCEMENTS_CLEAR_FILTERS_LABEL = "Clear Filters";

/** Detail: description section heading. */
export const ANNOUNCEMENT_DETAILS_DESCRIPTION_HEADING = "Description";

/** Detail: placeholder when HTML body is empty after normalization. */
export const ANNOUNCEMENT_DETAILS_BODY_EMPTY = "Nothing";

/** Detail: inline error under `ErrorIndicator`. */
export const ANNOUNCEMENT_DETAILS_ERROR_MESSAGE =
  "Could not load announcement details.";

/** `ErrorBanner` when case fetch fails. */
export const ANNOUNCEMENT_DETAILS_FETCH_ERROR_BANNER =
  "Could not load announcement details.";

/** `ErrorIndicator` entity label on detail error card. */
export const ANNOUNCEMENT_DETAILS_ERROR_ENTITY_NAME = "announcement details";

/** Allowed case-state option values for announcement filters (open / published-style states). */
export const ANNOUNCEMENT_CASE_STATE_ALLOWED_VALUES = ["1", "3"] as const;

/** Empty-state block vertical padding (MUI `sx` spacing). */
export const ANNOUNCEMENTS_EMPTY_STATE_CONTAINER_PY = 6;

/** Empty-state illustration max width (px). */
export const ANNOUNCEMENTS_EMPTY_STATE_ICON_MAX_WIDTH_PX = 200;

/** Empty-state illustration bottom margin (px). */
export const ANNOUNCEMENTS_EMPTY_STATE_ICON_MARGIN_BOTTOM_PX = 16;
