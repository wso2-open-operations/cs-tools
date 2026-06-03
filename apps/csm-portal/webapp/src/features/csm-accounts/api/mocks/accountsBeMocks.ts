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

import { getMockCsmAccounts } from "@features/csm-accounts/api/mocks/accountsMocks";
import type {
  Account,
  SearchAccountsRequest,
  SearchAccountsResponse,
} from "@features/csm-accounts/types/csmAccounts";

const ACTIVATION_BASE = new Date("2022-01-01T00:00:00Z").getTime();
const REGIONS = ["us-east-1", "eu-west-1", "ap-southeast-1", "us-west-2"];

function regionFor(id: string): string {
  let h = 0;
  for (let i = 0; i < id.length; i += 1) h = (h * 31 + id.charCodeAt(i)) | 0;
  return REGIONS[Math.abs(h) % REGIONS.length];
}

function activationFor(id: string): string {
  let h = 0;
  for (let i = 0; i < id.length; i += 1) h = (h * 17 + id.charCodeAt(i)) | 0;
  const daysOffset = Math.abs(h) % 800;
  return new Date(ACTIVATION_BASE + daysOffset * 86_400_000).toISOString();
}

/**
 * Build the BE-shape account list from the existing CSM mocks. Stable across
 * calls; intentionally derived from `getMockCsmAccounts("all_customers")` so
 * the same accounts the CSM views know about appear here.
 */
function buildBeAccounts(): Account[] {
  const rows = getMockCsmAccounts("all_customers").accounts;
  return rows.map((r) => ({
    id: r.id,
    sfId: `SF-${r.id.replace(/^acc-/, "").padStart(5, "0")}`,
    name: r.name,
    tier: r.tier === "Platinum" || r.tier === "Gold" ? "enterprise" : "basic",
    region: regionFor(r.id),
    activationDate: activationFor(r.id),
    deactivationDate: r.status === "Suspended" ? activationFor(`${r.id}-deact`) : null,
    ownerId: r.accountManager ?? "—",
    technicalOwnerId: r.technicalOwner ?? null,
    agentEnabled: true,
    kbReferencesEnabled: true,
    createdAt: activationFor(r.id),
    updatedAt: r.lastActivityAt,
  }));
}

function matchesQuery(a: Account, q: string): boolean {
  const needle = q.toLowerCase();
  return (
    a.name.toLowerCase().includes(needle) ||
    a.sfId.toLowerCase().includes(needle)
  );
}

export function getMockBackendAccountsResponse(
  req: SearchAccountsRequest,
): SearchAccountsResponse {
  const all = buildBeAccounts();
  const q = (req.searchQuery ?? "").trim();
  const filtered = q ? all.filter((a) => matchesQuery(a, q)) : all;
  const offset = req.pagination?.offset ?? 0;
  const limit = req.pagination?.limit ?? 20;
  const page = filtered.slice(offset, offset + limit);
  return {
    accounts: page,
    total: filtered.length,
    limit,
    offset,
    hasMore: offset + page.length < filtered.length,
  };
}
