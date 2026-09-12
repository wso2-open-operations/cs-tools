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

import type { BeCaseUpdateRequestStage } from "@api/backend/types";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

/** The two fixed reminder-message sets `case-update-request-templates` returns. */
export type CaseUpdateRequestCategory = "generic" | "migration";

/**
 * States in which "Request update" is offered — the case must actually be
 * waiting on the customer for the nudge to make sense. Mirrors the backend's
 * own gate in `RequestCaseUpdate` (`ErrMsgRequestUpdateNotAllowed` on any
 * other state), so the FE never offers an action the backend would 409 on.
 */
export function canRequestCaseUpdate(caseDetail: CsmCaseDetail): boolean {
  return (
    caseDetail.state === "awaiting_info" ||
    caseDetail.state === "solution_proposed"
  );
}

/**
 * Which fixed reminder-message set applies to this case. Replicates the
 * backend's own derivation in `RequestCaseUpdate` exactly — case-insensitive,
 * because `engagementType` is carried through unmodified from the data
 * source's raw display label (e.g. literally "Migration") rather than
 * normalized before it reaches either layer — so the dialog's preview always
 * matches what the backend will actually post.
 */
export function deriveCaseUpdateRequestCategory(
  caseDetail: Pick<CsmCaseDetail, "caseType" | "engagementType">,
): CaseUpdateRequestCategory {
  return caseDetail.caseType === "engagement" &&
    !!caseDetail.engagementType &&
    caseDetail.engagementType.toLowerCase() === "migration"
    ? "migration"
    : "generic";
}

/** Display labels for the fixed reminder stages, in menu/selector order. */
export const REQUEST_UPDATE_STAGE_LABEL: Record<
  Exclude<BeCaseUpdateRequestStage, "custom">,
  string
> = {
  first: "First reminder",
  second: "Second reminder",
  final: "Final notice",
};
