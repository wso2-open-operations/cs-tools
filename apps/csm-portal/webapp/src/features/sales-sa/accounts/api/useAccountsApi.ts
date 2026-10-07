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

// React Query hooks for SPL's account domain. Search/get/projects used to
// call this backend's own /spl/accounts* routes (a ServiceNow-shaped
// translation of entity-service data); all three now call CS Portal's own
// /accounts and /projects routes directly, since SPL's data source for them
// is the exact same entity-service data those routes already serve raw --
// see cs-tools' csm-portal-backend main.go SPL route registration comment.
// Escalations (read and create) and the Google Drive file listing also
// dropped their /spl/ prefix (the backend routes they call moved off it
// too), but keep calling their own dedicated routes: no entity-service
// equivalent exists for either escalation operation (CreateEscalation is an
// explicit stub there), so nothing to merge onto.
import { useQuery, useMutation, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type {
  AccountDetails,
  ABTTeamMembersDetails,
  EscalationDetails,
} from "./accountTypes";
import type { ProjectDetails } from "@features/sales-sa/projects/projectTypes";

// --- entity-service response shapes (this app's own copy of the JSON
// contract entity-service returns raw through CS Portal's /accounts and
// /projects routes -- see useGetSplAccount.ts/useSearchSplAccounts.ts in
// @features/csm-accounts for the reference shape this mirrors). ---

interface EntityPersonRef {
  id: string | null;
  name: string;
  email?: string | null;
}
interface EntityTeamRef {
  id: string;
  name: string;
}
interface EntityAccountView {
  id: string;
  number: string;
  name: string;
  region: string | null;
  country: string | null;
  city: string | null;
  driveLocation: string | null;
  accountManager: EntityPersonRef | null;
  technicalOwner: EntityPersonRef | null;
  customerSuccessManager: EntityPersonRef | null;
  creTeam: EntityTeamRef | null;
  deactivationDate: string | null;
}
interface EntitySearchAccountsResponse {
  accounts: EntityAccountView[];
  total: number;
}
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

// toAccountDetails adapts entity-service's raw account shape into the
// AccountDetails shape every SPL account UI component already renders --
// keeps ListAccounts.tsx/ListAccountDetail.tsx/etc. unchanged. ARR/rating
// have no entity-service equivalent (SupportTier/ArrToday are always nil on
// this data source -- a documented gap, not new here: SPL's own
// ServiceNow/Postgres translation layer always left these blank too).
function toAccountDetails(a: EntityAccountView): AccountDetails {
  return {
    id: a.id,
    number: a.number,
    name: a.name,
    region: a.region ?? "",
    country: a.country ?? "",
    city: a.city ?? "",
    arr: "",
    accountManager: a.accountManager?.name ?? "",
    technicalOwner: a.technicalOwner?.name ?? "",
    customerSuccessManager: a.customerSuccessManager?.name ?? "",
    rating: "",
    driveLocation: a.driveLocation ?? "",
    // integrationCSTeam* mirrors the field names the OLD /accounts/{id}
    // response used (see ListAccountDetail.tsx/TeamMembersDrawer.tsx, which
    // read these through AccountDetails' DataStruct index signature) so
    // those components need no changes -- only the source of the value
    // changed, from a ServiceNow group lookup to entity-service's own
    // creTeam ref.
    integrationCSTeamName: a.creTeam?.name ?? "",
    integrationCSTeamSysId: a.creTeam?.id ?? "",
    _deactivationDate: a.deactivationDate,
  };
}

function toProjectDetails(p: EntityProjectView): ProjectDetails {
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
    remainingQueryHours: "",
    closureState: "",
    accountNumber: p.account?.number,
    accountId: p.account?.id,
    accountName: p.account?.name,
  };
}

export function useSearchSplAccounts(params: {
  email?: string;
  phrase?: string;
  offset: number;
  limit: number;
  active: boolean;
  enabled?: boolean;
}): UseQueryResult<AccountDetails[], Error> {
  const api = useBackendApi();
  const { email, phrase, offset, limit, active, enabled = true } = params;

  return useQuery<AccountDetails[], Error>({
    queryKey: ["accounts-search", email ?? "", phrase ?? "", offset, limit, active],
    queryFn: async () => {
      const body: Record<string, unknown> = {
        filters: {
          searchQuery: phrase ?? "",
          ownerEmail: email ?? "",
          // Server-side filter (SearchAccountsFilters.active in
          // entity-service's openapi.yaml, applied in account_repo.go) --
          // only sent for Active Accounts mode, same as the request omits
          // pod/classification when unused. Filtering client-side instead
          // (as this used to) applies AFTER the server's own offset/limit,
          // so a page containing inactive accounts would silently return
          // fewer than a full page and could stall "next page" navigation.
          ...(active ? { active: true } : {}),
        },
        pagination: { offset, limit },
      };
      const resp = await api.post<typeof body, EntitySearchAccountsResponse>("/accounts/search", body);
      return (resp?.accounts ?? []).map(toAccountDetails);
    },
    enabled,
  });
}

export function useGetSplAccount(id: string): UseQueryResult<AccountDetails | null, Error> {
  const api = useBackendApi();
  return useQuery<AccountDetails | null, Error>({
    queryKey: ["account", id],
    queryFn: async () => {
      const a = await api.get<EntityAccountView>(`/accounts/${encodeURIComponent(id)}`);
      return a ? toAccountDetails(a) : null;
    },
    enabled: Boolean(id),
  });
}

export function useGetAccountProjects(
  accountId: string,
  offset: number,
  limit: number,
): UseQueryResult<ProjectDetails[], Error> {
  const api = useBackendApi();
  return useQuery({
    queryKey: ["account-projects", accountId, offset, limit],
    queryFn: async () => {
      const resp = await api.post<Record<string, unknown>, EntitySearchProjectsResponse>("/projects/search", {
        accountId,
        pagination: { offset, limit },
      });
      return (resp?.projects ?? []).map(toProjectDetails);
    },
    enabled: Boolean(accountId),
  });
}

export function useGetAccountEscalations(
  accountId: string,
  offset: number,
  limit: number,
): UseQueryResult<EscalationDetails[], Error> {
  const api = useBackendApi();
  return useQuery<EscalationDetails[], Error>({
    queryKey: ["spl-account-escalations", accountId, offset, limit],
    queryFn: () =>
      api
        .get<EscalationDetails[]>(
          `/accounts/${encodeURIComponent(accountId)}/escalations?offset=${offset}&limit=${limit}`,
        )
        .then((r) => r ?? []),
    enabled: Boolean(accountId),
  });
}

export interface EscalateCaseRequest {
  justification: string;
  requestSource: string;
  reason: string;
  severity: string;
}

/** POST /accounts/{accountId}/cases/{caseId}/escalate. */
export function useEscalateCase(accountId: string, caseId: string) {
  const api = useBackendApi();
  return useMutation<unknown, Error, EscalateCaseRequest>({
    mutationFn: (body) =>
      api.post(
        `/accounts/${encodeURIComponent(accountId)}/cases/${encodeURIComponent(caseId)}/escalate`,
        body,
      ),
  });
}

export function useGetAbtTeamMembers(teamId: string): UseQueryResult<ABTTeamMembersDetails[], Error> {
  const api = useBackendApi();
  return useQuery<ABTTeamMembersDetails[], Error>({
    queryKey: ["team-members", teamId],
    queryFn: () => api.get<ABTTeamMembersDetails[]>(`/teams/${encodeURIComponent(teamId)}/members`).then((r) => r ?? []),
    enabled: Boolean(teamId),
  });
}

export interface DriveFile {
  id: string;
  name: string;
  mimeType: string;
}

/** GET /files?folderId=... (Google Drive folder listing). */
export function useGetDriveFiles(folderId: string): UseQueryResult<DriveFile[], Error> {
  const api = useBackendApi();
  return useQuery<DriveFile[], Error>({
    queryKey: ["spl-drive-files", folderId],
    queryFn: () =>
      api.get<DriveFile[]>(`/files?folderId=${encodeURIComponent(folderId)}`).then((r) => r ?? []),
    enabled: Boolean(folderId),
  });
}
