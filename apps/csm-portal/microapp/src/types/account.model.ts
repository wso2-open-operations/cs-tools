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

import type { AccountDto, AccountTier } from "./account.dto";

export interface Account {
  id: string;
  sfId: string;
  name: string;
  tier: AccountTier;
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

// Resolves the PG-native-vs-ServiceNow-sourced dual shape once here (see AccountDto's own
// comment), so nothing downstream has to know about ownerId-vs-owner / agentEnabled-vs-hasAgent —
// callers just get a plain Account.
export function toAccount(dto: AccountDto): Account {
  return {
    id: dto.id,
    sfId: dto.sfId,
    name: dto.name,
    tier: dto.tier,
    region: dto.region ?? null,
    activationDate: dto.activationDate,
    deactivationDate: dto.deactivationDate ?? null,
    ownerId: dto.owner?.id ?? dto.ownerId ?? null,
    ownerName: dto.owner?.name ?? null,
    technicalOwnerId: dto.technicalOwner?.id ?? dto.technicalOwnerId ?? null,
    technicalOwnerName: dto.technicalOwner?.name ?? null,
    agentEnabled: dto.hasAgent ?? dto.agentEnabled ?? false,
    kbReferencesEnabled: dto.hasKbReferences ?? dto.kbReferencesEnabled ?? false,
    createdOn: dto.createdOn,
    updatedOn: dto.updatedOn,
  };
}
