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

/** Debounce for project search input (ms). */
export const PROJECT_HUB_SEARCH_DEBOUNCE_MS = 300;

/** Page size for infinite project list (must match `useInfiniteProjects`). */
export const PROJECT_HUB_PROJECTS_PAGE_SIZE = 20;

/** When total projects exceed this, require search before listing all cards. */
export const PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH = 4;

/** Skeleton cards shown while loading the hub grid. */
export const PROJECT_HUB_SKELETON_CARD_COUNT = 3;

export const PROJECT_HUB_REDIRECT_LOADER_MESSAGE =
  "Redirecting, this may take a moment…";

export const PROJECT_HUB_ERROR_TITLE = "Something Went Wrong";

export const PROJECT_HUB_ERROR_SUBTITLE =
  "We couldn't load the data right now. Please try again or refresh the page.";

export const PROJECT_HUB_EMPTY_SEARCH_TITLE = "No Projects Found";

export const PROJECT_HUB_EMPTY_DEFAULT_TITLE = "No Projects Yet";

export const PROJECT_HUB_EMPTY_SEARCH_SUBTITLE =
  "Try adjusting your search query";

export const PROJECT_HUB_EMPTY_DEFAULT_SUBTITLE =
  "Projects will appear here once they are created or assigned to you";

export const PROJECT_HUB_SEARCH_PLACEHOLDER = "Search projects...";

export const PROJECT_HUB_TITLE_SELECT = "Select Your Project";

export const PROJECT_HUB_SUBTITLE_SELECT =
  "Choose a project to access your support cases, chat history, and dashboard";

export const PROJECT_HUB_SUBTITLE_MANY_PROJECTS =
  "Please use the search bar below to find your project";

/**
 * @param totalRecords - Total project count from API.
 * @returns Hub title when the user has more than `PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH` projects.
 */
export function formatProjectHubManyProjectsTitle(totalRecords: number): string {
  return `You have ${totalRecords} projects`;
}

export const PROJECT_CARD_DATE_LOCALE = "en-US";

export const PROJECT_CARD_STATS_NULL_PLACEHOLDER = "--";

export const PROJECT_CARD_STATS_OUTSTANDING_ITEMS_LABEL =
  "Outstanding Items";

export const PROJECT_CARD_STATS_ACTIVE_CHATS_LABEL = "Active Chats";

export const PROJECT_CARD_INFO_TITLE_BLOCK_HEIGHT = "3.2rem";

export const PROJECT_CARD_VIEW_DASHBOARD_LABEL = "View Dashboard";

export const PROJECT_CARD_ERROR_ENTITY_OUTSTANDING_CASES =
  "Outstanding support cases";

export const PROJECT_CARD_ERROR_ENTITY_ACTIVE_CHATS = "Active Chats";

/** ServiceNow deep-link redirect (`ServiceNowCaseRedirectPage`). */
export const SERVICENOW_REDIRECT_NO_CASE_ID =
  "No case ID provided in the URL.";

export const SERVICENOW_REDIRECT_RESOLVE_ERROR =
  "Something went wrong";
