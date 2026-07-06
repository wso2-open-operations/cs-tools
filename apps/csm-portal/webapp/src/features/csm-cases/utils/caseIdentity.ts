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

import type { CsmCaseRow } from "@features/csm-cases/types/csmCases";

/**
 * Human-readable case identity: the project-scoped WSO2 case id and the
 * CS case number joined by a slash, dropping whichever is absent. Deliberately
 * never includes the UUID `id` (that is for API paths/links only). Returns ""
 * when the case carries neither human id.
 */
export function caseIdLabel(
  c: Pick<CsmCaseRow, "caseNumber" | "wso2CaseId">,
): string {
  return [c.wso2CaseId, c.caseNumber].filter(Boolean).join(" / ");
}
