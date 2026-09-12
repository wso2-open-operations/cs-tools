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

export type AccountTier = "basic" | "enterprise";

// A support-tier reference as returned on the account detail view for the
// backing-data-source-backed shape ({id, label}), vs. a plain label string on
// the list view. See `tier`/`supportTier` below.
export interface SupportTierRef {
  id: string;
  label: string;
}

// A named person reference as returned on the alternate response shape's
// account manager / technical owner / renewal account manager fields. `id`
// is null when the backing data source hasn't resolved this person to a
// canonical user record; `email` may be null when genuinely unset.
export interface PersonRef {
  id: string | null;
  name: string;
  email?: string | null;
}

export interface Account {
  id: string;
  sfId: string;
  name: string;
  // Postgres-backed accounts carry `tier`. Accounts sourced from the
  // alternate (ServiceNow-shaped) response instead carry `supportTier`,
  // either as a plain label string (list view) or an `{id, label}` ref
  // (detail view) — there is only one FE `Account` type today, so both are
  // modeled here as optional and resolved at the read site.
  tier?: AccountTier;
  supportTier?: string | SupportTierRef | null;
  region?: string | null;
  activationDate: string;
  deactivationDate?: string | null;
  // Postgres-backed accounts carry `ownerId`/`technicalOwnerId` as bare id
  // strings. Accounts sourced from the alternate response shape instead
  // carry the named people below — same dual-shape situation as `tier` above.
  ownerId: string;
  technicalOwnerId?: string | null;
  accountManager?: PersonRef | null;
  renewalAccountManager?: PersonRef | null;
  technicalOwner?: PersonRef | null;
  // The account's assigned CRE/SRE team, when the backing data source has
  // one on record — surfaced as a clickable link to the team directory page
  // on the account detail view, same shape/precedent as the case detail
  // page's `CustomerContextWidget` (see `CaseCustomerContext` in
  // `csmCases.ts`).
  creTeam?: { id: string; name: string } | null;
  sreTeam?: { id: string; name: string } | null;
  // The unified AccountView emits `hasAgent`/`hasKbReferences`; the older
  // `agentEnabled`/`kbReferencesEnabled` names are no longer sent for an
  // account by either data source. (`ProjectAccountRef` — the account
  // summary embedded in a project — still uses the old names; see
  // `csmProjects.ts`, which is a different shape and stays as it is.)
  hasAgent: boolean;
  hasKbReferences: boolean;
  createdOn: string;
  updatedOn: string;
}

/**
 * Resolves the Tier value regardless of which response shape the account
 * came from: Postgres-backed `tier`, or the alternate-shape `supportTier`
 * (plain string on the list view, `{id, label}` ref on the detail view).
 */
export function resolveAccountTier(
  account: Pick<Account, "tier" | "supportTier">,
): string | undefined {
  if (account.tier) return account.tier;
  if (typeof account.supportTier === "string") return account.supportTier;
  return account.supportTier?.label ?? undefined;
}

export type DeactivationState = "none" | "past" | "future";

/**
 * Classifies an account's deactivation date relative to `now` (defaults to
 * the real clock, injectable for tests). Used to keep the header chip and
 * the Overview card's label/tense in agreement: an unparseable date cannot
 * be asserted as past or future, so it is treated the same as "none".
 */
export function getDeactivationState(
  deactivationDate: string | null | undefined,
  now: Date = new Date(),
): DeactivationState {
  if (!deactivationDate) return "none";
  const d = new Date(deactivationDate);
  if (Number.isNaN(d.getTime())) return "none";
  return d.getTime() < now.getTime() ? "past" : "future";
}

export interface SearchAccountsRequest {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  filters?: {
    searchQuery?: string;
  };
}

export interface SearchAccountsResponse {
  accounts: Account[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}
