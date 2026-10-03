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

// React Query hooks for SPL's case domain. Search/get/comments/worknote
// used to call this backend's own /spl/cases* routes (a ServiceNow-shaped
// translation of entity-service data); all four now call CS Portal's own
// /cases routes directly, since SPL's data source for them is the exact
// same entity-service data those routes already serve raw -- see cs-tools'
// csm-portal-backend main.go SPL route registration comment. Attachments
// keep calling /cases/*: no entity-service storage/backfill path exists
// yet, so nothing to merge onto.
import { useMutation, useQuery, useQueryClient, type UseMutationResult, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi, BackendApiError } from "@api/backend/client";
import type { CaseCommentDetails, CaseDetails, CaseDetailsWithCount } from "./caseTypes";

// caseStateFromDisplay/caseStateToDisplay translate between the six display
// labels SPL's UI has always used (ServiceNow's own state labels, e.g. "Work
// In Progress" -- see CaseStateCard.tsx/CasesPage.tsx, which are NOT
// changing as part of this) and entity-service's domain.CaseState wire
// values (lowercase snake_case, e.g. "work_in_progress"). "closed" has no
// SPL summary card and is deliberately not in caseStateFromDisplay -- SPL's
// UI never asks to filter by a label it doesn't offer.
const caseStateFromDisplay: Record<string, string> = {
  Open: "open",
  "Work In Progress": "work_in_progress",
  "Awaiting Info": "awaiting_info",
  "Solution Proposed": "solution_proposed",
  "Waiting on WSO2": "waiting_on_wso2",
  Reopened: "reopened",
};
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
interface EntityUserRef {
  id: string | null;
  email: string;
  name: string;
}
interface EntitySearchCaseView {
  id: string;
  internalId: string;
  number: string;
  createdOn: string;
  subject: string | null;
  description: string | null;
  state: string | null;
  // Free-form display string from the backend, e.g. "Critical (P1)", "Low
  // (P4)" -- already in the exact label shape the UI's own PRIORITY_COLOR
  // lookup expects (CaseDetailPage.tsx), on both the search and GET views
  // (BeCaseSearchView/BeCaseView in src/api/backend/types.ts), so this needs
  // no reformatting.
  severity: string | null;
  product: EntityRef | null;
  project: EntityRef | null;
  projectKey: string | null;
  assignedEngineer: EntityUserRef | null;
  // The case creator. `id` is always null here by design (the data source
  // doesn't resolve the reporter to a user record on either view) -- `name`/
  // `email` are always populated.
  createdBy: EntityUserRef | null;
  account: EntityRef | null;
  // Nullable: ServiceNow-sourced cases may have no deployment. Only
  // {id, name} are ever populated on this join -- the deployment's own
  // `type` lives on a separate entity, reachable only via POST
  // /deployments/search (no ids filter) rather than this response, so it
  // isn't mapped below.
  deployment: EntityRef | null;
}
interface EntitySearchCasesResponse {
  cases: EntitySearchCaseView[];
  total: number;
}

function toCaseDetails(v: EntitySearchCaseView): CaseDetails {
  const state = v.state ?? "";
  return {
    id: v.id,
    caseId: v.internalId,
    number: v.number,
    caseType: "case",
    priority: v.severity ?? "",
    shortDescription: v.subject ?? "",
    description: v.description ?? "",
    state: caseStateToDisplay[state] ?? state,
    openedAt: v.createdOn,
    openedBy: v.createdBy?.name ?? v.createdBy?.email ?? "",
    assignedTo: v.assignedEngineer?.name ?? "",
    accountNumber: "",
    accountName: v.account?.name ?? "",
    accountId: v.account?.id,
    projectNumber: v.projectKey ?? "",
    projectKey: v.projectKey ?? "",
    projectId: v.project?.id,
    productName: v.product?.name ?? "",
    // Neither field is available: there is no case-level "last WSO2/customer
    // comment" timestamp, and it can't be derived from the comments list
    // either -- a case comment's author carries no customer-vs-engineer
    // signal today (CS-Portal's own native mapper has the identical gap and
    // hardcodes every comment author to 'wso2_engineer' rather than guess --
    // see mappers.ts's uiCommentFromBe, whose own doc comment says so
    // explicitly). Needs new backend work (attaching user type to a
    // comment's createdBy, or an equivalent lookup), not a frontend
    // wire-through.
    lastWSO2CommentTime: "",
    lastCustomerCommentTime: "",
    projectDeploymentName: v.deployment?.name ?? "",
    // Not available on this response -- the deployment's own `type` lives on
    // a separate entity, reachable only via POST /deployments/search (no ids
    // filter) rather than this one, so showing it here would mean an extra
    // network round trip per case. Deliberately left out of this fix.
    projectDeploymentType: "",
  };
}

export function useGetCases(
  stateFilter: string,
  offset: number,
  limit: number,
): UseQueryResult<CaseDetailsWithCount, Error> {
  const api = useBackendApi();
  return useQuery<CaseDetailsWithCount, Error>({
    queryKey: ["cases-search", stateFilter, offset, limit],
    queryFn: async () => {
      const entityState = caseStateFromDisplay[stateFilter];
      const body = {
        filters: {
          filters: entityState ? [{ field: "state", op: "in", values: [entityState] }] : [],
        },
        sortBy: { field: "createdOn", order: "desc" },
        pagination: { offset, limit },
      };
      const data = await api.post<typeof body, EntitySearchCasesResponse>("/cases/search", body);
      return { count: data.total, cases: (data.cases ?? []).map(toCaseDetails) };
    },
  });
}

export function useGetCase(caseId: string): UseQueryResult<CaseDetails, Error> {
  const api = useBackendApi();
  return useQuery<CaseDetails, Error>({
    queryKey: ["case", caseId],
    enabled: Boolean(caseId),
    queryFn: async () => {
      const data = await api.get<EntitySearchCaseView>(`/cases/${encodeURIComponent(caseId)}`);
      if (!data) throw new BackendApiError(404, "Case not found");
      return toCaseDetails(data);
    },
  });
}

interface EntityCaseComment {
  id: string;
  type: "work_note" | "comment" | "activity";
  content: string;
  createdBy: EntityUserRef | null;
  createdOn: string;
}
interface EntityCaseCommentsResponse {
  total: number;
  comments: EntityCaseComment[];
}
interface CaseCommentsResponse {
  total: number;
  comments: CaseCommentDetails[];
}

// commentTypeToDisplay maps entity-service's CommentType wire values to the
// two labels SPL's UI has always used (see CaseCommentDetails.type in
// caseTypes.ts) -- "activity" (system-generated field-change entries)
// has no SPL display bucket and is filtered out below, same as before.
function commentTypeToDisplay(t: EntityCaseComment["type"]): "comments" | "work_notes" | null {
  if (t === "comment") return "comments";
  if (t === "work_note") return "work_notes";
  return null;
}

export function useGetCaseComments(
  caseId: string,
  offset: number,
  limit: number,
): UseQueryResult<CaseCommentsResponse, Error> {
  const api = useBackendApi();
  return useQuery<CaseCommentsResponse, Error>({
    queryKey: ["case-comments", caseId, offset, limit],
    enabled: Boolean(caseId),
    queryFn: async () => {
      const body = { pagination: { offset, limit } };
      const data = await api.post<typeof body, EntityCaseCommentsResponse>(
        `/cases/${encodeURIComponent(caseId)}/comments/search`,
        body,
      );
      const comments: CaseCommentDetails[] = [];
      for (const c of data.comments ?? []) {
        const type = commentTypeToDisplay(c.type);
        if (!type) continue;
        comments.push({
          id: c.id,
          createdOn: c.createdOn,
          caseType: "case",
          type,
          value: c.content,
          createdBy: c.createdBy?.name ?? c.createdBy?.email ?? "",
        });
      }
      return { total: data.total, comments };
    },
  });
}

export function useGetCaseAttachments(caseId: string) {
  const api = useBackendApi();
  return useQuery({
    queryKey: ["spl-case-attachments", caseId],
    enabled: Boolean(caseId),
    queryFn: async () => {
      const data = await api.get(`/cases/${encodeURIComponent(caseId)}/attachments-info?offset=0&limit=10`);
      return data ?? [];
    },
  });
}

interface CreateCaseCommentPayload {
  type: "work_note";
  content: string;
}

/**
 * Post a work_note-type comment on a case via `POST /cases/{id}/comments` --
 * the only comment type a `worknote_creator`-only caller may ever send (see
 * PermCreateWorkNote's own doc comment on the backend; a full-PermWrite
 * caller could post any type, but this hook is only ever used for work
 * notes). On success, invalidates this case's comments list so the new note
 * shows without a manual refetch -- the mutation's own response isn't typed
 * further than that, since CaseBox's own query is the source of truth for
 * display.
 */
export function usePostWorkNote(caseId: string): UseMutationResult<unknown, Error, string> {
  const api = useBackendApi();
  const queryClient = useQueryClient();

  return useMutation<unknown, Error, string>({
    mutationFn: async (content: string) => {
      const payload: CreateCaseCommentPayload = { type: "work_note", content };
      return api.post<CreateCaseCommentPayload, unknown>(
        `/cases/${encodeURIComponent(caseId)}/comments`,
        payload,
      );
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["case-comments", caseId] });
    },
  });
}
