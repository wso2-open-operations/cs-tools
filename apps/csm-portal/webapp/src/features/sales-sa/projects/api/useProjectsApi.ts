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

// React Query hooks for SPL's project domain. Search/get/contacts/cases
// used to call this backend's own /spl/projects* routes (a ServiceNow-shaped
// translation of entity-service data); all four now call CS Portal's own
// /projects and /cases routes directly -- see
// ../../accounts/api/useAccountsApi.ts for the pattern this mirrors, and
// cs-tools' csm-portal-backend main.go SPL route registration comment for
// why.
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { ProjectDetails, Contact, CaseDetails } from "../projectTypes";

interface EntityProjectAccountRef {
  id: string;
  name: string;
  number: string;
}
interface EntityProjectView {
  id: string;
  name: string;
  key: string;
  startDate: string | null;
  endDate: string | null;
  account: EntityProjectAccountRef | null;
}
interface EntitySearchProjectsResponse {
  projects: EntityProjectView[];
  total: number;
}
interface EntityProjectDetail extends EntityProjectView {
  closureState: string | null;
  totalQueryHours: number | null;
  remainingQueryHours: number | null;
  subscriptionType: string | null;
}

function toProjectDetails(p: EntityProjectView | EntityProjectDetail): ProjectDetails {
  const detail = p as Partial<EntityProjectDetail>;
  return {
    id: p.id,
    // Postgres has no project "number" distinct from "key" -- the same
    // documented substitution the old backend translation layer made.
    number: p.key,
    sysId: p.id,
    name: p.name,
    key: p.key,
    startDate: p.startDate ?? "",
    endDate: p.endDate ?? "",
    remainingQueryHours: detail.remainingQueryHours != null ? String(detail.remainingQueryHours) : "",
    closureState: detail.closureState ?? "",
    accountNumber: p.account?.number,
    accountId: p.account?.id,
    accountName: p.account?.name,
    totalQueryHours: detail.totalQueryHours != null ? String(detail.totalQueryHours) : undefined,
    projectType: detail.subscriptionType ?? undefined,
  };
}

export function useSearchSplProjects(params: {
  phrase?: string;
  offset: number;
  limit: number;
  enabled?: boolean;
}): UseQueryResult<ProjectDetails[], Error> {
  const api = useBackendApi();
  const { phrase, offset, limit, enabled = true } = params;

  return useQuery<ProjectDetails[], Error>({
    queryKey: ["projects-search", phrase ?? "", offset, limit],
    queryFn: async () => {
      const body = { searchQuery: phrase ?? "", pagination: { offset, limit } };
      const resp = await api.post<typeof body, EntitySearchProjectsResponse>("/projects/search", body);
      return (resp?.projects ?? []).map(toProjectDetails);
    },
    enabled,
  });
}

export function useGetSplProject(id: string): UseQueryResult<ProjectDetails | null, Error> {
  const api = useBackendApi();
  return useQuery<ProjectDetails | null, Error>({
    queryKey: ["project", id],
    queryFn: async () => {
      const p = await api.get<EntityProjectDetail>(`/projects/${encodeURIComponent(id)}`);
      return p ? toProjectDetails(p) : null;
    },
    enabled: Boolean(id),
  });
}

interface EntityProjectContact {
  name: string | null;
  email: string;
  registrationState: string;
}
interface EntitySearchProjectContactsResponse {
  contacts: EntityProjectContact[];
  total: number;
}

export function useGetProjectContacts(
  projectId: string,
  offset: number,
  limit: number,
): UseQueryResult<Contact[], Error> {
  const api = useBackendApi();
  return useQuery<Contact[], Error>({
    queryKey: ["project-contacts", projectId, offset, limit],
    queryFn: async () => {
      const body = { pagination: { offset, limit } };
      const resp = await api.post<typeof body, EntitySearchProjectContactsResponse>(
        `/projects/${encodeURIComponent(projectId)}/contacts/search`,
        body,
      );
      return (resp?.contacts ?? []).map((c) => ({
        contactName: c.name ?? "",
        email: c.email,
        state: c.registrationState,
      }));
    },
    enabled: Boolean(projectId),
  });
}

const caseStateToDisplay: Record<string, string> = {
  open: "Open",
  work_in_progress: "Work In Progress",
  awaiting_info: "Awaiting Info",
  solution_proposed: "Solution Proposed",
  waiting_on_wso2: "Waiting on WSO2",
  reopened: "Reopened",
  closed: "Closed",
};

interface EntityRef {
  id: string;
  name: string;
}
interface EntitySearchCaseView {
  id: string;
  internalId: string;
  number: string;
  subject: string | null;
  state: string | null;
  product: EntityRef | null;
}
interface EntitySearchCasesResponse {
  cases: EntitySearchCaseView[];
  total: number;
}

/**
 * Project cases, via CS Portal's own POST /cases/search filtered by
 * projectId -- projectId must be the project's real UUID (ProjectDetails.id
 * / sysId), not its key/number. stateFilters/caseTypeFilters are accepted
 * for call-site compatibility but not applied: SPL's own project-cases view
 * never offered those controls even before this merge, and entity-service's
 * case search has no "type" filter at all (rejected as unsupported) --
 * silently ignoring rather than erroring, a known, documented gap carried
 * over unchanged from the backend translation layer this replaces.
 */
export function useGetProjectCases(params: {
  projectId: string;
  offset: number;
  limit: number;
  stateFilters: string[];
  caseTypeFilters: string[];
}): UseQueryResult<CaseDetails[], Error> {
  const api = useBackendApi();
  const { projectId, offset, limit } = params;

  return useQuery<CaseDetails[], Error>({
    queryKey: ["project-cases", projectId, offset, limit],
    queryFn: async () => {
      const body = {
        filters: { filters: [{ field: "projectId", op: "in", values: [projectId] }] },
        sortBy: { field: "createdOn", order: "desc" },
        pagination: { offset, limit },
      };
      const resp = await api.post<typeof body, EntitySearchCasesResponse>("/cases/search", body);
      return (resp?.cases ?? []).map((cv) => {
        const state = cv.state ?? "";
        return {
          id: cv.id,
          sysId: cv.id,
          number: cv.number,
          caseId: cv.internalId,
          caseType: "case",
          shortDescription: cv.subject ?? "",
          priority: "",
          state: caseStateToDisplay[state] ?? state,
        } satisfies CaseDetails;
      });
    },
    enabled: Boolean(projectId),
  });
}
