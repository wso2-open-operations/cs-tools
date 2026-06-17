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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { isMockMode, useBackendApi } from "@api/backend/client";
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import type { BeCaseView } from "@api/backend/types";
import { getMockCsmCaseDetailById } from "@features/csm-cases/api/mocks/casesMocks";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 150;

/**
 * Build a CsmCaseDetail from the rich `BeCaseView`. The view embeds the
 * account / project / deployment / deployed-product / reporter as objects, so
 * their names (and the account tier) come straight off the response with no
 * extra lookups. Side widgets the backend doesn't return yet (SLA clocks,
 * watchers, tags, time logs, attachments, linked items) default to empty /
 * placeholder values.
 */
function detailFromBeCase(c: BeCaseView): CsmCaseDetail {
  const account = c.account;
  const customer = account?.name ?? "—";
  // createdBy.name can be empty for unhydrated users, so fall back to the email.
  const reporter = c.createdBy?.name?.trim() || c.createdBy?.email;
  const assignee = c.assignedEngineer?.name?.trim() || "Unassigned";
  const product = c.deployedProduct?.displayName ?? "—";
  return {
    id: c.id,
    caseNumber: c.number,
    wso2CaseId: c.internalId,
    subject: c.subject ?? "(no subject)",
    customer,
    accountId: account?.id ?? "",
    projectId: c.project?.id ?? "",
    projectName: c.project?.name ?? "—",
    product,
    severity: severityFromPriority(c.priority),
    state: uiStateFromBe(c.state),
    nextStates: (c.nextStates ?? []).map(uiStateFromBe),
    assignee,
    // "Is me" needs the signed-in user's entity id, which this mapper doesn't
    // have; left false until the assignee can be compared to the current user.
    assigneeIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 0,
    createdAt: c.createdOn ?? "",
    updatedAt: c.updatedOn ?? c.createdOn ?? "",
    description: c.description ?? "",
    assignmentGroup: "grp.cre_team",
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
 * Calls `GET /cases/{id}` in LIVE mode and falls back to the seeded mock
 * detail when the mock toggle is on.
 */
export function useGetCsmCaseDetail(
  caseId: string | undefined,
): UseQueryResult<CsmCaseDetail | null, Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmCaseDetail | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseDetail | null> => {
      if (!caseId) return null;

      if (isMockMode()) {
        logger.debug(`[useGetCsmCaseDetail] Returning mock case ${caseId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCaseDetailById(caseId) ?? null;
      }

      const beCase = await api.get<BeCaseView>(
        `/cases/${encodeURIComponent(caseId)}`,
      );
      if (!beCase) return null;

      // The CaseView embeds account / project / deployment / deployed-product /
      // reporter, so the whole detail builds from this one response — no
      // follow-up lookups.
      return detailFromBeCase(beCase);
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
