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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import type { BeCaseView } from "@api/backend/types";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

/**
 * Build a CsmCaseDetail from the rich `BeCaseView`. The view embeds the
 * account / project / deployment / deployed-product / reporter as objects, so
 * their names (and the account tier) come straight off the response with no
 * extra lookups. Side widgets the backend doesn't return yet (SLA clocks,
 * watchers, tags, time logs, attachments, linked items) default to empty /
 * placeholder values.
 */
function detailFromBeCase(
  c: BeCaseView,
  currentUserEmail?: string,
): CsmCaseDetail {
  const account = c.account;
  const customer = account?.name ?? "—";
  // createdBy.name can be empty for unhydrated users, so fall back to the email.
  const reporter = c.createdBy?.name?.trim() || c.createdBy?.email;
  const assignee = c.assignedEngineer?.name?.trim() || "Unassigned";
  // "Is me" by comparing the assignee's email (the only stable identity the FE
  // shares with the JWT) to the signed-in user's, case-insensitively. Falls back
  // to false when either is absent — e.g. the data source doesn't return the
  // assignee email — so the gate stays closed rather than guessing.
  const assigneeEmail = c.assignedEngineer?.email ?? undefined;
  const assigneeIsMe =
    !!assigneeEmail &&
    !!currentUserEmail &&
    assigneeEmail.toLowerCase() === currentUserEmail.toLowerCase();
  // Prefer the deployed-product label (carries the version); fall back to the
  // plain product name, which the CaseView populates even when no specific
  // deployed product is linked. `||` so an empty displayName also falls through.
  const product = c.deployedProduct?.displayName || c.product?.name || "—";
  return {
    id: c.id,
    caseNumber: c.number,
    wso2CaseId: c.internalId,
    subject: c.subject ?? "(no subject)",
    caseType: c.type ?? undefined,
    customer,
    accountId: account?.id ?? "",
    projectId: c.project?.id ?? "",
    projectName: c.project?.name ?? "—",
    product,
    severity: severityFromPriority(c.severity),
    state: uiStateFromBe(c.state),
    workState: c.workState ?? null,
    nextStates: (c.nextStates ?? []).map(uiStateFromBe),
    assignee,
    assigneeIsMe,
    slaClockType: "ack",
    minutesToBreach: 0,
    createdAt: c.createdOn ?? "",
    updatedAt: c.updatedOn ?? c.createdOn ?? "",
    description: c.description ?? "",
    assignmentGroup: "grp.cre_team",
    conversationId: c.conversation?.id,
    createdBy: reporter,
    createdByEmail: c.createdBy?.email,
    customerContext: {
      accountName: customer,
      // Account tier from the embedded account's `type` (e.g. "Enterprise");
      // free-form, so tolerated downstream by tierLabel/tierColor.
      tier: account?.type ?? "",
      // The CaseView account ref carries only id/name/type, so region and the
      // account manager aren't available here.
      region: "—",
      // The reporter (createdBy) is the customer-side person who opened it.
      primaryContact: reporter ?? "—",
      primaryContactEmail: c.createdBy?.email ?? "—",
      accountManager: "—",
      openCases: 0,
    },
    productContext: {
      product,
      version: "—",
      deployment: c.deployment?.name ?? "—",
      deploymentId: c.deployment?.id,
      environment: "prod",
    },
    slaClocks: [],
    watchers: [],
    linkedItems: [],
    tags: [],
    timeLogs: [],
    audit: [],
    attachments: [],
    isWatching: false,
  };
}

/**
 * Look up a single CSM case by id. Returns `null` (not an error) when the
 * id is unknown so the page can render a not-found state for that case.
 *
 * Calls `GET /cases/{id}`.
 */
export function useGetCsmCaseDetail(
  caseId: string | undefined,
): UseQueryResult<CsmCaseDetail | null, Error> {
  const api = useBackendApi();
  // Signed-in user's email, used to resolve `assigneeIsMe` against the case's
  // assigned-engineer email. In the query key so a late-arriving claim recomputes.
  const currentUserEmail = useIdTokenClaims()?.email;

  return useQuery<CsmCaseDetail | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId ?? "", currentUserEmail ?? ""],
    queryFn: async (): Promise<CsmCaseDetail | null> => {
      if (!caseId) return null;

      const beCase = await api.get<BeCaseView>(
        `/cases/${encodeURIComponent(caseId)}`,
      );
      if (!beCase) return null;

      // The CaseView embeds account / project / deployment / deployed-product /
      // reporter, so the whole detail builds from this one response — no
      // follow-up lookups.
      return detailFromBeCase(beCase, currentUserEmail);
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
