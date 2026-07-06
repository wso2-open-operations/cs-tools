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

export interface Account {
  id: string;
  sfId: string;
  name: string;
  tier: AccountTier;
  region?: string | null;
  activationDate: string;
  deactivationDate?: string | null;
  ownerId: string;
  technicalOwnerId?: string | null;
  agentEnabled: boolean;
  kbReferencesEnabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface SearchAccountsRequest {
  pagination?: {
    limit?: number;
    offset?: number;
  };
  searchQuery?: string;
}

export interface SearchAccountsResponse {
  accounts: Account[];
  total: number;
  limit: number;
  offset: number;
  hasMore: boolean;
}
