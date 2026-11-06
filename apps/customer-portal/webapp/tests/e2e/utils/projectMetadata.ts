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

//
// Waiting on the per-project metadata the UI gates itself on.
//
// These are read off the app's own requests rather than fetched separately:
// the tokens live in the Asgardeo SDK's sessionStorage, so the page is the only
// thing that can call the backend as the signed-in user — and reading what the
// page actually received is what the UI decided on, which is the point.
//
// Every waiter here must be armed **before** the navigation that triggers the
// request.
//

import { type Page, type Response } from "../fixtures/test";

/** How long to allow for the shell to boot and issue its metadata requests. */
const METADATA_TIMEOUT_MS = 60_000;

/** The subset of `GET /projects/{id}/features` the dashboard specs read
 * (`ProjectFeatures` in src/features/project-hub/types/projects.ts). */
export interface ProjectFeaturesBody {
  hasDeploymentReadAccess?: boolean;
  hasDeploymentWriteAccess?: boolean;
  /** Becomes `permissions.hasEngagements`, which gates the Engagements nav item
   * and the dashboard's Outstanding Engagements donut. */
  hasEngagementsReadAccess?: boolean;
}

/** One entry of a stats response's engagement-type breakdown. */
export interface EngagementTypeCount {
  id: string;
  label: string;
  count?: number;
}

/** The subset of `GET /projects/{id}/stats/cases?caseTypes=engagement` the
 * dashboard specs read. */
export interface EngagementStatsBody {
  outstandingEngagementTypeCount?: EngagementTypeCount[];
}

/** An id/label pair as `GET /projects/{id}/filters` returns it. */
export interface FilterOption {
  id: string;
  label: string;
}

/** The subset of `GET /projects/{id}/filters` the dashboard specs read. */
export interface ProjectFiltersBody {
  caseStates?: FilterOption[];
  issueTypes?: FilterOption[];
  severities?: FilterOption[];
}

/** One project contact from `GET /projects/{id}/contacts`. */
export interface ProjectContact {
  email: string;
  firstName?: string | null;
  lastName?: string | null;
}

/**
 * Starts waiting for a successful GET on one of a project's endpoints.
 *
 * @param page - Test page.
 * @param projectId - Project whose endpoint to match.
 * @param suffix - Path after `/projects/{id}`, e.g. "/features".
 * @returns Promise for the matching response.
 */
function projectGet(
  page: Page,
  projectId: string,
  suffix: string,
): Promise<Response> {
  return page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      response.ok() &&
      new URL(response.url()).pathname.endsWith(
        `/projects/${projectId}${suffix}`,
      ),
    { timeout: METADATA_TIMEOUT_MS },
  );
}

/**
 * Starts waiting for the project's feature flags.
 *
 * @param page - Test page.
 * @param projectId - Project to match.
 * @returns Promise for the parsed body.
 */
export async function projectFeaturesResponse(
  page: Page,
  projectId: string,
): Promise<ProjectFeaturesBody> {
  const response = await projectGet(page, projectId, "/features");
  return (await response.json()) as ProjectFeaturesBody;
}

/**
 * Whether the portal treats the project as having deployments.
 *
 * Mirrors `getProjectPermissions` (src/utils/permission.ts): either the read or
 * the write flag turns `permissions.hasDeployments` on.
 *
 * @param features - The project's features.
 * @returns The derived `hasDeployments`.
 */
export function hasDeployments(features: ProjectFeaturesBody): boolean {
  return (
    !!features.hasDeploymentReadAccess || !!features.hasDeploymentWriteAccess
  );
}

/**
 * Starts waiting for the project's filter metadata — the case states, issue
 * types and severities the filter Selects are built from.
 *
 * @param page - Test page.
 * @param projectId - Project to match.
 * @returns Promise for the parsed body.
 */
export async function projectFiltersResponse(
  page: Page,
  projectId: string,
): Promise<ProjectFiltersBody> {
  const response = await projectGet(page, projectId, "/filters");
  return (await response.json()) as ProjectFiltersBody;
}

/**
 * Starts waiting for the project's contacts — the people the Created By
 * filter offers.
 *
 * @param page - Test page.
 * @param projectId - Project to match.
 * @returns Promise for the parsed body.
 */
export async function projectContactsResponse(
  page: Page,
  projectId: string,
): Promise<ProjectContact[]> {
  const response = await projectGet(page, projectId, "/contacts");
  return (await response.json()) as ProjectContact[];
}

/**
 * Starts waiting for the dashboard's engagement-only stats — the response the
 * Outstanding Engagements donut and its legend are drawn from.
 *
 * Matched on `caseTypes` being exactly `engagement`: the dashboard calls the
 * same endpoint several times (combined, default-case, SR), and only this one
 * feeds the donut.
 *
 * The request is only made when the project has engagements
 * (`permissions.hasEngagements`), so on a project without them this never
 * resolves. A rejection is pre-handled here so a caller that skips without
 * awaiting it does not leave an unhandled rejection behind; a caller that does
 * await it still sees the timeout.
 *
 * @param page - Test page.
 * @param projectId - Project to match.
 * @returns Promise for the parsed body.
 */
export function engagementStatsResponse(
  page: Page,
  projectId: string,
): Promise<EngagementStatsBody> {
  const body = page
    .waitForResponse(
      (response) => {
        const url = new URL(response.url());
        const caseTypes = url.searchParams.getAll("caseTypes");
        return (
          response.request().method() === "GET" &&
          response.ok() &&
          url.pathname.endsWith(`/projects/${projectId}/stats/cases`) &&
          caseTypes.length === 1 &&
          caseTypes[0] === "engagement"
        );
      },
      { timeout: METADATA_TIMEOUT_MS },
    )
    .then(async (response) => (await response.json()) as EngagementStatsBody);
  body.catch(() => undefined);
  return body;
}
