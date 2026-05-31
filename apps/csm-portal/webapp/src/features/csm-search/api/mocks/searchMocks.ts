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

import { getMockCsmCases } from "@features/csm-cases/api/mocks/casesMocks";
import { getMockCsmProjects } from "@features/csm-projects/api/mocks/projectsMocks";
import { getMockCsmAccounts } from "@features/csm-accounts/api/mocks/accountsMocks";
import type {
  CsmSearchAccountHit,
  CsmSearchCaseHit,
  CsmSearchProjectHit,
  CsmSearchResults,
} from "@features/csm-search/types/csmSearch";

const MAX_PER_GROUP = 6;

function normalise(value: string): string {
  return value.trim().toLowerCase();
}

/**
 * Search across cases, projects and accounts in the CSM mocks.
 *
 * Matchable fields, per the current product spec:
 *   - cases: internal case id (`case-1001`), WSO2 case number (`CS-1001`),
 *     case subject
 *   - projects: project id (treated as the project "key"), project name
 *   - accounts: account name
 *
 * Always searches across `all_customers` so the global search returns hits
 * regardless of the engineer's ABT scope.
 */
export function searchMockCsm(rawQuery: string): CsmSearchResults {
  const q = normalise(rawQuery);
  if (q.length === 0) {
    return { query: rawQuery, cases: [], projects: [], accounts: [] };
  }

  const allCases = getMockCsmCases("all_customers").cases;
  const allProjects = getMockCsmProjects("all_customers").projects;
  const allAccounts = getMockCsmAccounts("all_customers").accounts;

  const cases: CsmSearchCaseHit[] = allCases
    .filter((c) => {
      return (
        normalise(c.id).includes(q) ||
        normalise(c.caseNumber).includes(q) ||
        normalise(c.subject).includes(q)
      );
    })
    .slice(0, MAX_PER_GROUP)
    .map((c) => ({
      kind: "case",
      id: c.id,
      caseNumber: c.caseNumber,
      severity: c.severity,
      title: `${c.caseNumber} · ${c.subject}`,
      subtitle: `${c.customer} · ${c.projectName}`,
      badge: c.severity,
      href: `/cases/${c.id}`,
    }));

  const projects: CsmSearchProjectHit[] = allProjects
    .filter((p) => {
      return (
        normalise(p.id).includes(q) ||
        normalise(p.name).includes(q)
      );
    })
    .slice(0, MAX_PER_GROUP)
    .map((p) => ({
      kind: "project",
      id: p.id,
      title: p.name,
      subtitle: `${p.customer} · ${p.productType}`,
      badge: p.tier,
      href: `/projects/${p.id}`,
    }));

  const accounts: CsmSearchAccountHit[] = allAccounts
    .filter((a) => normalise(a.name).includes(q))
    .slice(0, MAX_PER_GROUP)
    .map((a) => ({
      kind: "account",
      id: a.id,
      title: a.name,
      subtitle: `${a.projectCount} project${a.projectCount === 1 ? "" : "s"} · ${a.openCaseCount} open cases`,
      badge: a.tier,
      href: `/accounts/${a.id}`,
    }));

  return { query: rawQuery, cases, projects, accounts };
}
