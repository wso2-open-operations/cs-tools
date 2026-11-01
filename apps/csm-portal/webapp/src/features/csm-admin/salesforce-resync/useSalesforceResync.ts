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

import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import { BackendApiError, useBackendApi } from "@api/backend/client";
import { ApiQueryKeys } from "@constants/apiConstants";

/** What a comparison or overwrite is run against. */
export type SalesforceResyncScope = "account" | "project";

/** Component keys the backend understands (see POST /salesforce/resync/preview). */
export type SalesforceResyncComponentKey =
  | "account"
  | "partners"
  | "projects"
  | "project"
  | "opportunities"
  | "linkedOpportunities"
  | "invoices"
  | "contacts"
  | "memberships";

export interface SalesforceResyncTarget {
  scope: SalesforceResyncScope;
  /** The CSM id of the account or project. */
  id: string;
}

export interface SalesforceResyncFieldDiff {
  field: string;
  csmValue: string | null;
  sfValue: string | null;
  wouldTruncate: boolean;
}

export type SalesforceResyncRecordState = "differs" | "missing_in_csm" | "only_in_csm";

export interface SalesforceResyncRecord {
  sfId: string;
  label: string;
  state: SalesforceResyncRecordState;
  fields: SalesforceResyncFieldDiff[];
}

export interface SalesforceResyncCounts {
  inSync: number;
  differs: number;
  missingInCsm: number;
  onlyInCsm: number;
}

export type SalesforceResyncComponentStatus = "in_sync" | "differs" | "skipped";

export interface SalesforceResyncComponent {
  key: SalesforceResyncComponentKey;
  label: string;
  status: SalesforceResyncComponentStatus;
  skippedReason?: string | null;
  counts: SalesforceResyncCounts;
  records: SalesforceResyncRecord[];
}

export interface SalesforceResyncPreview {
  target: { scope: SalesforceResyncScope; id: string; sfId: string; name?: string | null };
  capped: boolean;
  cappedReason?: string | null;
  components: SalesforceResyncComponent[];
}

export interface SalesforceResyncApplyRequest extends SalesforceResyncTarget {
  components: SalesforceResyncComponentKey[];
}

export type SalesforceResyncApplyStatus = "applied" | "failed" | "skipped";

export interface SalesforceResyncApplyComponent {
  key: SalesforceResyncComponentKey;
  status: SalesforceResyncApplyStatus;
  succeeded: number;
  failed: number;
  skipped?: number;
  skippedStale: number;
  skippedStaleSfIds?: string[];
  skippedReason?: string | null;
  errors?: { sfId: string; message: string }[];
}

export interface SalesforceResyncApplyResponse {
  components: SalesforceResyncApplyComponent[];
}

/**
 * Both calls are synchronous and can run close to a minute on the backend.
 * `useBackendApi` sets no timeout of its own, so without this the browser
 * would wait on whatever the network stack decides; 120s leaves headroom over
 * the backend's own request time limit and still ends a hung call.
 */
export const SALESFORCE_RESYNC_TIMEOUT_MS = 120_000;

/** Components that start unticked even when they differ: they change who can access what. */
export const DELIBERATE_COMPONENT_KEYS: readonly SalesforceResyncComponentKey[] = ["contacts", "memberships"];

export const SALESFORCE_RESYNC_TIMEOUT_MESSAGE =
  "This took too long, try again. If you were overwriting, run the comparison again first to see what has changed.";

function isTimeoutError(error: unknown): boolean {
  return (
    error instanceof Error && (error.name === "TimeoutError" || error.name === "AbortError")
  );
}

/**
 * A person-readable message for a failed preview or apply call: 409 (another
 * run in progress), 403 (not an admin) and timeouts get fixed wording, every
 * other failure shows the backend's own message.
 */
export function salesforceResyncErrorMessage(error: unknown): string {
  if (isTimeoutError(error)) return SALESFORCE_RESYNC_TIMEOUT_MESSAGE;
  if (error instanceof BackendApiError) {
    if (error.status === 409) {
      return "Another comparison or overwrite is already running for this record, try again shortly.";
    }
    if (error.status === 403) {
      return "Only administrators can compare or overwrite data from Salesforce. If you need this, ask an administrator.";
    }
    if (error.status === 504) return SALESFORCE_RESYNC_TIMEOUT_MESSAGE;
    return error.message || "The request failed.";
  }
  if (error instanceof Error && error.message) return error.message;
  return "The request failed.";
}

function timeoutSignal(): AbortSignal | undefined {
  return typeof AbortSignal !== "undefined" && "timeout" in AbortSignal
    ? AbortSignal.timeout(SALESFORCE_RESYNC_TIMEOUT_MS)
    : undefined;
}

/**
 * The read-only comparison (POST /salesforce/resync/preview). A mutation, not
 * a query: it is slow, one-per-click and must never refire on focus or remount.
 */
export function useSalesforceResyncPreview(): UseMutationResult<
  SalesforceResyncPreview,
  Error,
  SalesforceResyncTarget
> {
  const api = useBackendApi();
  return useMutation<SalesforceResyncPreview, Error, SalesforceResyncTarget>({
    mutationFn: (target) =>
      api.post<SalesforceResyncTarget, SalesforceResyncPreview>("/salesforce/resync/preview", target, {
        signal: timeoutSignal(),
      }),
  });
}

/** Cache roots that show account or project data; refetched after an overwrite. */
const REFRESHED_QUERY_ROOTS: readonly string[] = [
  ApiQueryKeys.CSM_ACCOUNT_DETAIL,
  ApiQueryKeys.CSM_ACCOUNT_PROJECTS,
  ApiQueryKeys.CSM_ACCOUNTS,
  ApiQueryKeys.CSM_PROJECT_DETAIL,
  ApiQueryKeys.CSM_PROJECTS,
  ApiQueryKeys.PROJECT_CONTACTS,
  "csm-accounts-search",
  "csm-accounts-search-paged",
  "csm-projects-search",
];

/**
 * The overwrite (POST /salesforce/resync/apply) for the listed components
 * only. A 200 carries a per-component outcome (some can have failed), so the
 * account and project caches are refreshed on any 200, not only when every
 * component applied.
 */
export function useSalesforceResyncApply(): UseMutationResult<
  SalesforceResyncApplyResponse,
  Error,
  SalesforceResyncApplyRequest
> {
  const api = useBackendApi();
  const queryClient = useQueryClient();
  return useMutation<SalesforceResyncApplyResponse, Error, SalesforceResyncApplyRequest>({
    mutationFn: (request) =>
      api.post<SalesforceResyncApplyRequest, SalesforceResyncApplyResponse>("/salesforce/resync/apply", request, {
        signal: timeoutSignal(),
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        predicate: (q) => typeof q.queryKey[0] === "string" && REFRESHED_QUERY_ROOTS.includes(q.queryKey[0]),
      });
    },
  });
}
