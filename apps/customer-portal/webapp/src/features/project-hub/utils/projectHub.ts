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
  PROJECT_HUB_SUBTITLE_SELECT,
  formatProjectHubTitle,
} from "@features/project-hub/constants/projectHubConstants";
import { ProjectHubContentView } from "@features/project-hub/types/projectHub";

/**
 * Resolves which main content block to render under the hub header.
 *
 * @param isRedirectingToSingleProject - Auto-redirect when exactly one project and no search.
 * @param isAuthLoading - Asgardeo still loading.
 * @param isLoading - Projects query loading.
 * @param isError - Projects query failed.
 * @param projectsLength - Length of the current flattened projects list.
 * @param isCheckingAllSuspended - Background-paginating to confirm all projects are suspended.
 * @returns {ProjectHubContentView} View id for the hub body.
 */
export function resolveProjectHubContentView(
  isRedirectingToSingleProject: boolean,
  isAuthLoading: boolean,
  isLoading: boolean,
  isError: boolean,
  projectsLength: number,
  isCheckingAllSuspended: boolean = false,
): ProjectHubContentView {
  if (isRedirectingToSingleProject) {
    return ProjectHubContentView.REDIRECT_LOADER;
  }
  if (isAuthLoading) {
    return ProjectHubContentView.AUTH_PENDING;
  }
  if (isLoading || isCheckingAllSuspended) {
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

/** Hub page title — always shows the project count. */
export function resolveProjectHubHeaderTitle(totalRecords: number): string {
  return formatProjectHubTitle(totalRecords);
}

/** Hub subtitle — always the standard select prompt. */
export function resolveProjectHubHeaderSubtitle(): string {
  return PROJECT_HUB_SUBTITLE_SELECT;
}

/** Show the search bar when the user has more than one project or has an active search query. */
export function shouldShowProjectHubSearchBar(
  totalRecords: number,
  searchQuery: string,
): boolean {
  return totalRecords > PROJECT_HUB_MIN_PROJECTS_FOR_SEARCH || searchQuery.trim().length > 0;
}

/**
 * Whether to hide the title/search header block (error or empty state without search).
 */
export function shouldHideProjectHubHeaderBlock(
  isError: boolean,
  isLoading: boolean,
  isAuthLoading: boolean,
  projectsLength: number,
  searchQuery: string,
): boolean {
  const trimmedSearch = searchQuery.trim();
  return (
    isError ||
    (isLoading && !trimmedSearch) ||
    (!isAuthLoading && projectsLength === 0 && !trimmedSearch)
  );
}
