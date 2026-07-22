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
  ownerId: string;
  technicalOwnerId: string | null;
  agentEnabled: boolean;
  kbReferencesEnabled: boolean;
  createdOn: string;
  updatedOn: string;
}

export function toAccount(dto: AccountDto): Account {
  return {
    id: dto.id,
    sfId: dto.sfId,
    name: dto.name,
    tier: dto.tier,
    region: dto.region ?? null,
    activationDate: dto.activationDate,
    deactivationDate: dto.deactivationDate ?? null,
    ownerId: dto.ownerId,
    technicalOwnerId: dto.technicalOwnerId ?? null,
    agentEnabled: dto.agentEnabled,
    kbReferencesEnabled: dto.kbReferencesEnabled,
    createdOn: dto.createdOn,
    updatedOn: dto.updatedOn,
  };
}
