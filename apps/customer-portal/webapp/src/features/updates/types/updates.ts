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

// Model type for update level breakdown.
export type UpdateLevelBreakdown = { regular: number; security: number };

// Response type for updates statistics.
export type UpdatesStats = {
  productsTracked: number | null;
  totalUpdatesInstalled: number | null;
  totalUpdatesInstalledBreakdown?: UpdateLevelBreakdown;
  totalUpdatesPending: number | null;
  totalUpdatesPendingBreakdown?: UpdateLevelBreakdown;
  securityUpdatesPending: number | null;
}

// Item type for a single product recommended update level item.
export type RecommendedUpdateLevelItem = {
  productName: string;
  productBaseVersion: string;
  channel: string;
  startingUpdateLevel: number;
  endingUpdateLevel: number;
  installedUpdatesCount: number;
  installedSecurityUpdatesCount: number;
  timestamp: number;
  recommendedUpdateLevel: number;
  availableUpdatesCount: number;
  availableSecurityUpdatesCount: number;
}

// Item type for product update levels.
export type ProductUpdateLevelEntry = {
  productBaseVersion: string;
  channel: string;
  updateLevels: number[];
}

// Item type for one product's update levels.
export type ProductUpdateLevelsItem = {
  productName: string;
  productUpdateLevels: ProductUpdateLevelEntry[];
}

// Item type for a security advisory item inside an update description level.
export type SecurityAdvisory = {
  id: string;
  overview: string;
  severity: string;
  description: string;
  impact: string;
  solution: string;
  notes: string;
  credits: string;
}

// Item type for a single update description entry within an update level.
export type UpdateDescriptionLevel = {
  updateLevel: number;
  productName: string;
  productVersion: string;
  channel: string;
  updateType: string;
  updateNumber: number;
  description: string;
  instructions: string;
  bugFixes: string;
  filesAdded: string;
  filesModified: string;
  filesRemoved: string;
  bundlesInfoChanges: string | null;
  dependantReleases: string | null;
  timestamp: number;
  securityAdvisories: SecurityAdvisory[];
}

// Item type for a single update level key from POST /updates/levels/search.
export type UpdateLevelEntry = {
  updateType: string;
  updateDescriptionLevels: UpdateDescriptionLevel[];
}

// Response type for POST /updates/levels/search.
export type UpdateLevelsSearchResponse = Record<string, UpdateLevelEntry>;

// Request type for searching update levels.
export type UpdateLevelsSearchRequest = {
  startingUpdateLevel: number;
  endingUpdateLevel: number;
  productName: string;
  productVersion: string;
}
