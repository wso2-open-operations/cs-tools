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

import type { AccountDto, AccountOwnerRefDto } from "./account.dto";

export interface Account {
  id: string;
  sfId: string;
  name: string;
  /** Display label for the tier, or null when the data source has none.
   *  Resolved from `tier` (PG-native lowercase enum) or `supportTier`
   *  (ServiceNow label / ref) — callers get one field either way. */
  tier: string | null;
  region: string | null;
  activationDate: string;
  deactivationDate: string | null;
  ownerId: string | null;
  /** Resolved display name for the owner, when the source is ServiceNow. Falls back to
   * `ownerId` in the UI when this is null (a PG-native account). */
  ownerName: string | null;
  technicalOwnerId: string | null;
  technicalOwnerName: string | null;
  agentEnabled: boolean;
  kbReferencesEnabled: boolean;
  createdOn: string;
  updatedOn: string;
}

// `tier` (PG-native) and `supportTier` (unified AccountView) are the same concept
// under two names and two shapes; resolve to one display label here so no page
// has to know which data source an account came from.
// The unified AccountView renamed `owner` -> `accountManager`. `owner` is
// preferred when present so a backend still emitting it is unaffected, and
// `accountManager` is used otherwise — without this, ownerId/ownerName are null
// for every account and the detail page renders "—". Same shape of fix as
// resolveTier below.
function resolveOwner(dto: AccountDto): AccountOwnerRefDto | null {
  return dto.owner ?? dto.accountManager ?? null;
}

function resolveTier(dto: AccountDto): string | null {
  if (dto.tier) return dto.tier;
  if (typeof dto.supportTier === "string") return dto.supportTier || null;
  return dto.supportTier?.label ?? null;
}

// Resolves the PG-native-vs-ServiceNow-sourced dual shape once here (see AccountDto's own
// comment), so nothing downstream has to know about ownerId-vs-owner / agentEnabled-vs-hasAgent —
// callers just get a plain Account.
export function toAccount(dto: AccountDto): Account {
  return {
    id: dto.id,
    sfId: dto.sfId,
    name: dto.name,
    tier: resolveTier(dto),
    region: dto.region ?? null,
    activationDate: dto.activationDate,
    deactivationDate: dto.deactivationDate ?? null,
    ownerId: resolveOwner(dto)?.id ?? dto.ownerId ?? null,
    ownerName: resolveOwner(dto)?.name ?? null,
    technicalOwnerId: dto.technicalOwner?.id ?? dto.technicalOwnerId ?? null,
    technicalOwnerName: dto.technicalOwner?.name ?? null,
    agentEnabled: dto.hasAgent ?? dto.agentEnabled ?? false,
    kbReferencesEnabled: dto.hasKbReferences ?? dto.kbReferencesEnabled ?? false,
    createdOn: dto.createdOn,
    updatedOn: dto.updatedOn,
  };
}
