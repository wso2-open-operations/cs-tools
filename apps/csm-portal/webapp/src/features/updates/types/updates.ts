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

// Mirrors csm-portal-backend/internal/updates/types.go (portal camelCase).

export interface UpdateLevel {
  productBaseVersion: string;
  channel: string;
  updateLevels: number[];
}

export interface ProductUpdateLevel {
  productName: string;
  productUpdateLevels: UpdateLevel[];
}

export interface SearchUpdatesPayload {
  productName: string;
  productVersion: string;
  startingUpdateLevel: number;
  endingUpdateLevel: number;
}

export interface SecurityAdvisory {
  id: string;
  overview: string;
  severity: string;
  description: string;
  impact: string;
  solution: string;
  notes: string;
  credits: string;
}

export interface DependantRelease {
  repository: string;
  releaseVersion: string;
}

export interface UpdateDescription {
  updateLevel: number;
  productName: string;
  productVersion: string;
  channel: string;
  updateType: string;
  updateNumber: number;
  description?: string;
  instructions?: string;
  bugFixes?: string;
  filesAdded?: string;
  filesModified?: string;
  filesRemoved?: string;
  bundlesInfoChanges?: string;
  dependantReleases?: DependantRelease[];
  timestamp: number;
  securityAdvisories: SecurityAdvisory[];
}

export interface UpdateLevelGroup {
  updateType: string;
  updateDescriptionLevels: UpdateDescription[];
}

// POST /updates/levels/search response: map of level key -> group
export type UpdateLevelsSearchResponse = Record<string, UpdateLevelGroup>;
