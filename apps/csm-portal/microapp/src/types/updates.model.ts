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

import type { UpdateDescriptionDto, UpdateLevelsSearchResponseDto } from "./updates.dto";

export interface UpdateLevel {
  productBaseVersion: string;
  channel: string;
  updateLevels: number[];
}

export interface ProductUpdateLevel {
  productName: string;
  productUpdateLevels: UpdateLevel[];
}

export interface SearchUpdatesInput {
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
  bugFixes: string[];
  filesAdded: string[];
  filesModified: string[];
  filesRemoved: string[];
  dependantReleases: DependantRelease[];
  releasedOn: Date | null;
  securityAdvisories: SecurityAdvisory[];
}

export interface UpdateLevelGroup {
  levelKey: string;
  updateType: string;
  updateDescriptionLevels: UpdateDescription[];
}

// The upstream sometimes JSON-encodes these list fields as a string
// (e.g. '["Foo.java","Bar.java"]') rather than sending a real array —
// normalized here once so components never see the raw encoded form.
function parseJsonStringArray(raw: string | undefined | null): string[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as string[]) : [];
  } catch {
    return [];
  }
}

export function toUpdateDescription(dto: UpdateDescriptionDto): UpdateDescription {
  return {
    updateLevel: dto.updateLevel,
    productName: dto.productName,
    productVersion: dto.productVersion,
    channel: dto.channel,
    updateType: dto.updateType,
    updateNumber: dto.updateNumber,
    description: dto.description,
    instructions: dto.instructions,
    bugFixes: parseJsonStringArray(dto.bugFixes),
    filesAdded: parseJsonStringArray(dto.filesAdded),
    filesModified: parseJsonStringArray(dto.filesModified),
    filesRemoved: parseJsonStringArray(dto.filesRemoved),
    dependantReleases: dto.dependantReleases ?? [],
    releasedOn: Number.isFinite(dto.timestamp) && dto.timestamp > 0 ? new Date(dto.timestamp) : null,
    securityAdvisories: dto.securityAdvisories ?? [],
  };
}

/** Level-key-sorted groups, ready to render — components never see the raw response map. */
export function toUpdateLevelGroups(dto: UpdateLevelsSearchResponseDto): UpdateLevelGroup[] {
  return Object.entries(dto)
    .map(([levelKey, group]) => ({
      levelKey,
      updateType: group.updateType,
      updateDescriptionLevels: group.updateDescriptionLevels.map(toUpdateDescription),
    }))
    .sort((a, b) => Number(a.levelKey) - Number(b.levelKey));
}
