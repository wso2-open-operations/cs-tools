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
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import type {
  BeChangeRequestSearchPayload,
  BeChangeRequestSearchResponse,
  BeChangeRequestSearchView,
  BeCreateStandardChangeProposalPayload,
  BeDecideStandardChangeProposalPayload,
  BeEntityRef,
  BeProjectSearchPayload,
  BeProjectSearchResponse,
  BeSearchStandardChangeProposalsPayload,
  BeSearchStandardChangeProposalsResponse,
  BeSearchStandardChangeTemplatesPayload,
  BeSearchStandardChangeTemplatesResponse,
  BeStandardChangeProposal,
  BeStandardChangeTemplate,
  BeStandardChangeTemplateVersion,
  BeSuspendStandardChangeTemplatePayload,
  BeUpdateStandardChangeProposalPayload,
} from "@api/backend/types";

// Operations > Change requests > Standard changes: the CAB-approved template
// catalogue and the proposals that create, revise or retire its templates.
// Every route is a thin BFF proxy of entity-service, which decides who may act
// (the proposer edits and submits, a CAB member decides, the owner or a CAB
// member suspends): a refusal keeps its message and `errorCode`, so callers
// show `err.message`.

/** Every standard change query lives under this key, so one write refreshes them all. */
export const STANDARD_CHANGES_KEY = ["operations", "standard-changes"] as const;
const TEMPLATES_KEY = [...STANDARD_CHANGES_KEY, "templates"] as const;
const PROPOSALS_KEY = [...STANDARD_CHANGES_KEY, "proposals"] as const;

const TEMPLATES = "/standard-change-templates";
const PROPOSALS = "/standard-change-proposals";

const enc = encodeURIComponent;

/** One page of the catalogue (`POST /standard-change-templates/search`). */
export function useSearchStandardChangeTemplates(
  payload: BeSearchStandardChangeTemplatesPayload,
): UseQueryResult<BeSearchStandardChangeTemplatesResponse, Error> {
  const api = useBackendApi();
  return useQuery<BeSearchStandardChangeTemplatesResponse, Error>({
    queryKey: [...TEMPLATES_KEY, "search", payload],
    queryFn: () =>
      api.post<BeSearchStandardChangeTemplatesPayload, BeSearchStandardChangeTemplatesResponse>(
        `${TEMPLATES}/search`,
        payload,
      ),
    placeholderData: keepPreviousData,
  });
}

/**
 * Type-ahead over one customer's ACTIVE templates (the only ones a change
 * request for that customer's project can be raised from), for
 * `AsyncEntitySelect` (`searchExtra` = the customer's account id). A template is
 * one customer's, so with no customer there is nothing to offer and nothing is
 * searched.
 */
export function useSearchActiveStandardChangeTemplatesForSelect(
  query: string,
  enabled: boolean,
  accountId?: string,
): UseQueryResult<BeStandardChangeTemplate[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  return useQuery<BeStandardChangeTemplate[], Error>({
    queryKey: [...TEMPLATES_KEY, "select", accountId ?? "", q],
    queryFn: async () => {
      const body: BeSearchStandardChangeTemplatesPayload = {
        states: ["active"],
        accountId,
        pagination: { offset: 0, limit: 20 },
      };
      if (q) body.query = q;
      const res = await api.post<BeSearchStandardChangeTemplatesPayload, BeSearchStandardChangeTemplatesResponse>(
        `${TEMPLATES}/search`,
        body,
      );
      return res.templates ?? [];
    },
    enabled: enabled && !!accountId,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  });
}

/** A template with its current version and version history; `null` when not found. */
export function useGetStandardChangeTemplate(
  id: string | undefined,
): UseQueryResult<BeStandardChangeTemplate | null, Error> {
  const api = useBackendApi();
  return useQuery<BeStandardChangeTemplate | null, Error>({
    queryKey: [...TEMPLATES_KEY, "detail", id],
    queryFn: () => api.get<BeStandardChangeTemplate>(`${TEMPLATES}/${enc(id!)}`),
    enabled: !!id,
  });
}

/** One version as the CAB approved it (immutable, so cached for good). */
export function useGetStandardChangeTemplateVersion(
  templateId: string | undefined,
  versionId: string | undefined,
): UseQueryResult<BeStandardChangeTemplateVersion | null, Error> {
  const api = useBackendApi();
  return useQuery<BeStandardChangeTemplateVersion | null, Error>({
    queryKey: [...TEMPLATES_KEY, "version", templateId, versionId],
    queryFn: () =>
      api.get<BeStandardChangeTemplateVersion>(`${TEMPLATES}/${enc(templateId!)}/versions/${enc(versionId!)}`),
    enabled: !!templateId && !!versionId,
    staleTime: Infinity,
  });
}

/** One page of proposals (`POST /standard-change-proposals/search`). */
export function useSearchStandardChangeProposals(
  payload: BeSearchStandardChangeProposalsPayload,
): UseQueryResult<BeSearchStandardChangeProposalsResponse, Error> {
  const api = useBackendApi();
  return useQuery<BeSearchStandardChangeProposalsResponse, Error>({
    queryKey: [...PROPOSALS_KEY, "search", payload],
    queryFn: () =>
      api.post<BeSearchStandardChangeProposalsPayload, BeSearchStandardChangeProposalsResponse>(
        `${PROPOSALS}/search`,
        payload,
      ),
    placeholderData: keepPreviousData,
  });
}

/**
 * How many proposals match `filter` (the search's `total`, asking for one row),
 * for the "needs your attention" line above the catalogue. It is a hint, not
 * the list: a failure is not retried and the caller says nothing about it.
 */
export function useCountStandardChangeProposals(
  filter: Omit<BeSearchStandardChangeProposalsPayload, "pagination">,
): UseQueryResult<number, Error> {
  const api = useBackendApi();
  return useQuery<number, Error>({
    queryKey: [...PROPOSALS_KEY, "count", filter],
    queryFn: async () => {
      const res = await api.post<BeSearchStandardChangeProposalsPayload, BeSearchStandardChangeProposalsResponse>(
        `${PROPOSALS}/search`,
        { ...filter, pagination: { offset: 0, limit: 1 } },
      );
      return res.total ?? 0;
    },
    retry: false,
    staleTime: 30_000,
  });
}

/** A proposal with its content, evidence and CAB approvers; `null` when not found. */
export function useGetStandardChangeProposal(
  id: string | undefined,
): UseQueryResult<BeStandardChangeProposal | null, Error> {
  const api = useBackendApi();
  return useQuery<BeStandardChangeProposal | null, Error>({
    queryKey: [...PROPOSALS_KEY, "detail", id],
    queryFn: () => api.get<BeStandardChangeProposal>(`${PROPOSALS}/${enc(id!)}`),
    enabled: !!id,
  });
}

/**
 * Every write reads the standard change queries again when it settles, refused
 * or not: a decision writes a template, a suspension changes what the catalogue
 * and the create form offer, and a refusal (409) usually means the page is stale.
 */
function useStandardChangeMutation<TVars, TResult>(
  fn: (vars: TVars) => Promise<TResult>,
): UseMutationResult<TResult, Error, TVars> {
  const qc = useQueryClient();
  return useMutation<TResult, Error, TVars>({
    mutationFn: fn,
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: STANDARD_CHANGES_KEY });
    },
  });
}

/** The longest suspension reason the server takes. */
export const MAX_SUSPEND_REASON_LENGTH = 1000;

/**
 * `POST /standard-change-templates/{id}/suspend` (the owner or a CAB member), with
 * the optional reason trimmed: a blank one is left out of the body.
 */
export function useSuspendStandardChangeTemplate(): UseMutationResult<
  BeStandardChangeTemplate,
  Error,
  { id: string; reason?: string }
> {
  const api = useBackendApi();
  return useStandardChangeMutation(({ id, reason }: { id: string; reason?: string }) => {
    const trimmed = reason?.trim();
    return api.post<BeSuspendStandardChangeTemplatePayload, BeStandardChangeTemplate>(
      `${TEMPLATES}/${enc(id)}/suspend`,
      trimmed ? { reason: trimmed } : {},
    );
  });
}

/** `POST /standard-change-templates/{id}/resume` (the owner or a CAB member). */
export function useResumeStandardChangeTemplate(): UseMutationResult<BeStandardChangeTemplate, Error, string> {
  const api = useBackendApi();
  return useStandardChangeMutation((id: string) =>
    api.postEmpty<BeStandardChangeTemplate>(`${TEMPLATES}/${enc(id)}/resume`),
  );
}

/** `POST /standard-change-templates/{id}/clear-review` (the owner or a CAB member). */
export function useClearStandardChangeTemplateReview(): UseMutationResult<BeStandardChangeTemplate, Error, string> {
  const api = useBackendApi();
  return useStandardChangeMutation((id: string) =>
    api.postEmpty<BeStandardChangeTemplate>(`${TEMPLATES}/${enc(id)}/clear-review`),
  );
}

/** `POST /standard-change-proposals`: a new draft (new, modify or retire). */
export function useCreateStandardChangeProposal(): UseMutationResult<
  BeStandardChangeProposal,
  Error,
  BeCreateStandardChangeProposalPayload
> {
  const api = useBackendApi();
  return useStandardChangeMutation((body: BeCreateStandardChangeProposalPayload) =>
    api.post<BeCreateStandardChangeProposalPayload, BeStandardChangeProposal>(PROPOSALS, body),
  );
}

/** `PATCH /standard-change-proposals/{id}`: a draft, by its proposer. */
export function useUpdateStandardChangeProposal(): UseMutationResult<
  BeStandardChangeProposal,
  Error,
  { id: string; patch: BeUpdateStandardChangeProposalPayload }
> {
  const api = useBackendApi();
  return useStandardChangeMutation(({ id, patch }: { id: string; patch: BeUpdateStandardChangeProposalPayload }) =>
    api.patch<BeUpdateStandardChangeProposalPayload, BeStandardChangeProposal>(`${PROPOSALS}/${enc(id)}`, patch),
  );
}

/** `POST /standard-change-proposals/{id}/submit`: sends a complete draft to the CAB. */
export function useSubmitStandardChangeProposal(): UseMutationResult<BeStandardChangeProposal, Error, string> {
  const api = useBackendApi();
  return useStandardChangeMutation((id: string) =>
    api.postEmpty<BeStandardChangeProposal>(`${PROPOSALS}/${enc(id)}/submit`),
  );
}

/** `POST /standard-change-proposals/{id}/withdraw`: the proposer, while a draft or in CAB review. */
export function useWithdrawStandardChangeProposal(): UseMutationResult<BeStandardChangeProposal, Error, string> {
  const api = useBackendApi();
  return useStandardChangeMutation((id: string) =>
    api.postEmpty<BeStandardChangeProposal>(`${PROPOSALS}/${enc(id)}/withdraw`),
  );
}

/** `POST /standard-change-proposals/{id}/decision`: a CAB member on their own row. */
export function useDecideStandardChangeProposal(): UseMutationResult<
  BeStandardChangeProposal,
  Error,
  { id: string } & BeDecideStandardChangeProposalPayload
> {
  const api = useBackendApi();
  return useStandardChangeMutation(({ id, ...body }: { id: string } & BeDecideStandardChangeProposalPayload) =>
    api.post<BeDecideStandardChangeProposalPayload, BeStandardChangeProposal>(`${PROPOSALS}/${enc(id)}/decision`, body),
  );
}

/** A safety bound on paging through one customer's projects. */
const MAX_CUSTOMER_PROJECT_PAGES = 40;

/**
 * The ids of every project of one customer (account), paged through
 * `POST /projects/search` with `accountId`. The change request search has no
 * account filter, so a customer's change requests are found by its projects.
 */
export function useCustomerProjectIds(accountId: string | undefined): UseQueryResult<string[], Error> {
  const api = useBackendApi();
  return useQuery<string[], Error>({
    queryKey: [...STANDARD_CHANGES_KEY, "customer-projects", accountId],
    queryFn: async () => {
      const ids: string[] = [];
      for (let page = 0; page < MAX_CUSTOMER_PROJECT_PAGES; page++) {
        const res = await api.post<BeProjectSearchPayload, BeProjectSearchResponse>("/projects/search", {
          accountId,
          pagination: { offset: ids.length, limit: BE_MAX_PAGE_LIMIT },
        });
        const rows = res.projects ?? [];
        ids.push(...rows.map((p) => p.id));
        if (rows.length < BE_MAX_PAGE_LIMIT || !res.hasMore) break;
      }
      return ids;
    },
    enabled: !!accountId,
    staleTime: 60_000,
  });
}

/**
 * Type-ahead over the CLOSED change requests of one customer, for a proposal's
 * evidence (a proposal cites completed changes of its own customer to show the
 * change is repeatable; the server refuses another customer's). `accountId` is
 * the proposal's customer (`AsyncEntitySelect`'s `searchExtra`): with none
 * nothing is searched, and a customer with no projects has no change requests.
 */
export function useSearchClosedChangeRequestsForSelect(
  query: string,
  enabled: boolean,
  accountId?: string,
): { data: BeChangeRequestSearchView[] | undefined; isFetching: boolean; isError: boolean } {
  const api = useBackendApi();
  const q = query.trim();
  const projects = useCustomerProjectIds(enabled ? accountId : undefined);
  const projectIds = projects.data;
  const search = useQuery<BeChangeRequestSearchView[], Error>({
    queryKey: [...STANDARD_CHANGES_KEY, "closed-change-requests", accountId ?? "", q, projectIds],
    queryFn: async () => {
      const res = await api.post<BeChangeRequestSearchPayload, BeChangeRequestSearchResponse>("/change-requests/search", {
        filters: { searchQuery: q, states: ["closed"], projectIds },
        pagination: { offset: 0, limit: 20 },
      });
      return res.changeRequests ?? [];
    },
    enabled: enabled && !!accountId && !!projectIds && projectIds.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
  // A customer with no projects has no change requests to cite: an empty list, not a search.
  const noProjects = !!projectIds && projectIds.length === 0;
  return {
    data: noProjects ? [] : search.data,
    isFetching: projects.isFetching || search.isFetching,
    isError: projects.isError || search.isError,
  };
}

/**
 * Type-ahead over the customers (accounts, `POST /accounts/search`), for naming
 * the customer a standard change is for and for the catalogue's customer filter.
 */
export function useSearchStandardChangeCustomersForSelect(
  query: string,
  enabled: boolean,
): UseQueryResult<BeEntityRef[], Error> {
  const api = useBackendApi();
  const q = query.trim();
  return useQuery<BeEntityRef[], Error>({
    queryKey: [...STANDARD_CHANGES_KEY, "customers", q],
    queryFn: async () => {
      const res = await api.post<
        { pagination: { offset: number; limit: number }; filters?: { searchQuery: string } },
        { accounts?: Array<{ id: string; name?: string | null }> | null }
      >("/accounts/search", {
        pagination: { offset: 0, limit: 20 },
        ...(q ? { filters: { searchQuery: q } } : {}),
      });
      return (res.accounts ?? []).map((a) => ({ id: a.id, name: a.name || a.id }));
    },
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  });
}

/**
 * The customer (account) of a project (`GET /projects/{id}`): the customer a
 * standard change raised for it must belong to. `null` when the project has no
 * customer recorded or is not found.
 */
export function useProjectCustomer(projectId: string | undefined): UseQueryResult<BeEntityRef | null, Error> {
  const api = useBackendApi();
  return useQuery<BeEntityRef | null, Error>({
    queryKey: [...STANDARD_CHANGES_KEY, "project-customer", projectId],
    queryFn: async () => {
      const project = await api.get<{ account?: { id?: string | null; name?: string | null } | null } | null>(
        `/projects/${enc(projectId!)}`,
      );
      const account = project?.account;
      return account?.id ? { id: account.id, name: account.name || account.id } : null;
    },
    enabled: !!projectId,
    staleTime: 60_000,
  });
}
