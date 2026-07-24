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

/** A resolved `{id, name}` reference, as returned for ServiceNow-sourced accounts —
 * mirrors the webapp's own AccountOwnerRef (csm-accounts/types/csmAccounts.ts). */
export interface AccountOwnerRefDto {
  id: string;
  name?: string | null;
}

// Same shape for both a search-result row and GET /accounts/{id} — unlike
// Project, Account has no separate enriched detail shape.
//
// GET /accounts/{id} and POST /accounts/search return one of two shapes depending on the
// account's data source: PG-native accounts send bare `ownerId`/`technicalOwnerId` strings and
// `agentEnabled`/`kbReferencesEnabled` booleans, while ServiceNow-sourced accounts send resolved
// `owner`/`technicalOwner` {id, name} references and `hasAgent`/`hasKbReferences` instead — none
// of which openapi.yaml's Account schema documents. Confirmed live (a ServiceNow account's
// response carries `owner: {id, name}` with no bare `ownerId` at all) — same class of
// doc-vs-reality gap already caught for this endpoint on the webapp side.
export interface AccountDto {
  id: string;
  sfId: string;
  name: string;
  tier: AccountTier;
  region?: string | null;
  activationDate: string;
  deactivationDate?: string | null;
  ownerId?: string | null;
  owner?: AccountOwnerRefDto | null;
  technicalOwnerId?: string | null;
  technicalOwner?: AccountOwnerRefDto | null;
  agentEnabled?: boolean;
  hasAgent?: boolean;
  kbReferencesEnabled?: boolean;
  hasKbReferences?: boolean;
  createdOn: string;
  updatedOn: string;
}

export interface AccountSearchPayloadDto {
  pagination?: { offset?: number; limit?: number };
  searchQuery?: string;
}

export interface AccountSearchResponseDto {
  accounts: AccountDto[];
  total: number;
  limit: number;
  offset: number;
  hasMore?: boolean;
}
