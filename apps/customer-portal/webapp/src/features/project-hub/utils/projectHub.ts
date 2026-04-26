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
  PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH,
  PROJECT_HUB_SUBTITLE_MANY_PROJECTS,
  PROJECT_HUB_SUBTITLE_SELECT,
  PROJECT_HUB_TITLE_SELECT,
  formatProjectHubManyProjectsTitle,
} from "@features/project-hub/constants/projectHubConstants";
import { ProjectHubContentView } from "@features/project-hub/types/projectHub";

/**
 * Resolves which main content block to render under the hub header.
 *
 * @param isRedirectingToSingleProject - Auto-redirect when exactly one project and no search.
 * @param isAuthLoading - Asgardeo still loading.
 * @param isLoading - Projects query loading.
 * @param isError - Projects query failed.
 * @param totalRecords - Total projects from API metadata.
 * @param searchQuery - Current search input.
 * @param projectsLength - Projects on the current flattened list.
 * @returns {ProjectHubContentView} View id for the hub body.
 */
export function resolveProjectHubContentView(
  isRedirectingToSingleProject: boolean,
  isAuthLoading: boolean,
  isLoading: boolean,
  isError: boolean,
  _totalRecords: number,
  _searchQuery: string,
  projectsLength: number,
): ProjectHubContentView {
  if (isRedirectingToSingleProject) {
    return ProjectHubContentView.REDIRECT_LOADER;
  }
  if (isAuthLoading) {
    return ProjectHubContentView.AUTH_PENDING;
  }
  if (isLoading) {
    return ProjectHubContentView.LOADING_SKELETONS;
  }
  if (isError) {
    return ProjectHubContentView.ERROR;
  }
  if (projectsLength === 0) {
    return ProjectHubContentView.EMPTY_STATE;
  }
  return ProjectHubContentView.PROJECT_LIST;
}

/**
 * Hub page title next to the folder icon.
 *
 * @param totalRecords - Total project count.
 * @param hasSearchQuery - Whether the user typed a search.
 * @returns Title string.
 */
export function resolveProjectHubHeaderTitle(
  totalRecords: number,
  hasSearchQuery: boolean,
): string {
  switch (true) {
    case totalRecords > PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH && !hasSearchQuery:
      return formatProjectHubManyProjectsTitle(totalRecords);
    default:
      return PROJECT_HUB_TITLE_SELECT;
  }
}

/**
 * Hub subtitle under the title.
 *
 * @param totalRecords - Total project count.
 * @param hasSearchQuery - Whether the user typed a search.
 * @returns Subtitle string.
 */
export function resolveProjectHubHeaderSubtitle(
  totalRecords: number,
  hasSearchQuery: boolean,
): string {
  switch (true) {
    case totalRecords > PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH && !hasSearchQuery:
      return PROJECT_HUB_SUBTITLE_MANY_PROJECTS;
    default:
      return PROJECT_HUB_SUBTITLE_SELECT;
  }
}

/**
 * Whether to show the search field (many projects or active search).
 *
 * @param totalRecords - Total project count.
 * @param searchQuery - Raw search input.
 */
export function shouldShowProjectHubSearchBar(
  totalRecords: number,
  searchQuery: string,
): boolean {
  return (
    totalRecords > PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH ||
    searchQuery.trim().length > 0
  );
}

/**
 * Search-only layout: many projects and user has not searched yet.
 *
 * @param totalRecords - Total project count.
 * @param searchQuery - Raw search input.
 * @param isLoading - Projects loading.
 * @param isAuthLoading - Auth loading.
 * @param isError - Query error.
 */
export function shouldShowProjectHubSearchOnlyLayout(
  _totalRecords: number,
  _searchQuery: string,
  _isLoading: boolean,
  _isAuthLoading: boolean,
  _isError: boolean,
): boolean {
  return false;
}

/**
 * Whether to hide the title/search header block (error empty state without search).
 *
 * @param isError - Query error.
 * @param isLoading - Projects loading.
 * @param isAuthLoading - Auth loading.
 * @param projectsLength - Current list length.
 * @param searchQuery - Raw search input.
 */
export function shouldHideProjectHubHeaderBlock(
  isError: boolean,
  isLoading: boolean,
  isAuthLoading: boolean,
  projectsLength: number,
  searchQuery: string,
): boolean {
  return (
    isError ||
    (!isLoading &&
      !isAuthLoading &&
      projectsLength === 0 &&
      !searchQuery.trim())
  );
}
