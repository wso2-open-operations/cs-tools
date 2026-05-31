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

import { UPDATES_NULL_PLACEHOLDER } from "@features/updates/constants/updatesConstants";
import type { StatCardColor } from "@/features/dashboard/constants/dashboard";
import {
  UpdatePendingLevelType,
  UpdateProductCardHeaderStatus,
  UpdatesStatKey,
  type PendingUpdateLevelRow,
  type ProductUpdateLevelsItem,
  type RecommendedUpdateLevelItem,
  type UpdatesStatConfigItem,
  type UpdatesStats,
} from "@features/updates/types/updates";

/** @deprecated Prefer importing {@link UPDATES_NULL_PLACEHOLDER} from constants. */
export const NULL_PLACEHOLDER = UPDATES_NULL_PLACEHOLDER;

/**
 * Returns the StatCard-compatible chip colour for an update type.
 *
 * @param updateType - The update type string from the API.
 * @returns {StatCardColor} The chip colour token.
 */
export const getUpdateTypeChipColor = (updateType: string): StatCardColor => {
  switch (updateType) {
    case "security":
      return "error";
    default:
      return "success";
  }
};

/**
 * Resolves header / progress colour from “healthy” heuristic.
 *
 * @param isHealthy - Whether the product is at least halfway to recommended.
 * @returns {UpdateProductCardHeaderStatus} MUI colour token.
 */
export function resolveUpdateCardHeaderStatusColor(
  isHealthy: boolean,
): UpdateProductCardHeaderStatus {
  switch (isHealthy) {
    case true:
      return UpdateProductCardHeaderStatus.Info;
    case false:
      return UpdateProductCardHeaderStatus.Warning;
  }
}

/**
 * Label for the product card CTA to open pending updates.
 *
 * @param pendingLevels - Count of pending levels.
 * @returns {string} Button label text.
 */
export function formatViewPendingUpdatesButtonLabel(
  pendingLevels: number,
): string {
  return `View ${pendingLevels} Pending Updates`;
}

/**
 * Aggregates recommended update level data into summary statistics.
 *
 * @param data - The raw update level data.
 * @returns {UpdatesStats | undefined} The aggregated statistics.
 */
export const aggregateUpdateStats = (
  data: RecommendedUpdateLevelItem[] | undefined,
): UpdatesStats | undefined => {
  if (!data) {
    return undefined;
  }

  return data.reduce<UpdatesStats>(
    (acc, item) => {
      const installedRegular = item.installedUpdatesCount;
      const installedSecurity = item.installedSecurityUpdatesCount;
      const pendingRegular = item.availableUpdatesCount;
      const pendingSecurity = item.availableSecurityUpdatesCount;

      acc.totalUpdatesInstalled =
        (acc.totalUpdatesInstalled || 0) + installedRegular + installedSecurity;
      if (acc.totalUpdatesInstalledBreakdown) {
        acc.totalUpdatesInstalledBreakdown.regular += installedRegular;
        acc.totalUpdatesInstalledBreakdown.security += installedSecurity;
      }

      acc.totalUpdatesPending =
        (acc.totalUpdatesPending || 0) + pendingRegular + pendingSecurity;
      if (acc.totalUpdatesPendingBreakdown) {
        acc.totalUpdatesPendingBreakdown.regular += pendingRegular;
        acc.totalUpdatesPendingBreakdown.security += pendingSecurity;
      }

      acc.securityUpdatesPending =
        (acc.securityUpdatesPending || 0) + pendingSecurity;

      return acc;
    },
    {
      productsTracked: data.length,
      totalUpdatesInstalled: 0,
      totalUpdatesInstalledBreakdown: { regular: 0, security: 0 },
      totalUpdatesPending: 0,
      totalUpdatesPendingBreakdown: { regular: 0, security: 0 },
      securityUpdatesPending: 0,
    },
  );
};

/**
 * Gets the display value for a specific statistic ID from aggregated data.
 *
 * @param aggregatedData - The aggregated statistics.
 * @param id - The ID of the statistic to retrieve.
 * @returns {string | number} The display value.
 */
export const getStatValue = (
  aggregatedData: UpdatesStats | undefined,
  id: keyof UpdatesStats,
): string | number => {
  if (!aggregatedData) {
    return UPDATES_NULL_PLACEHOLDER;
  }

  const val = aggregatedData[id];
  if (val === null || val === undefined) {
    return UPDATES_NULL_PLACEHOLDER;
  }

  if (typeof val === "object") {
    return UPDATES_NULL_PLACEHOLDER;
  }

  return val as string | number;
};

/**
 * Gets the tooltip text for a stat card, including breakdown info if applicable.
 *
 * @param stat - The stat card definition.
 * @param aggregatedData - The aggregated statistics.
 * @returns {string} The tooltip text.
 */
export const getStatTooltipText = (
  stat: UpdatesStatConfigItem,
  aggregatedData: UpdatesStats | undefined,
): string => {
  if (!aggregatedData) {
    return stat.tooltipText;
  }

  switch (stat.id) {
    case UpdatesStatKey.TotalUpdatesInstalled:
      if (aggregatedData.totalUpdatesInstalledBreakdown) {
        const { regular, security } =
          aggregatedData.totalUpdatesInstalledBreakdown;
        return `${stat.tooltipText} (${regular} Regular • ${security} Security)`;
      }
      return stat.tooltipText;
    case UpdatesStatKey.TotalUpdatesPending:
      if (aggregatedData.totalUpdatesPendingBreakdown) {
        const { regular, security } =
          aggregatedData.totalUpdatesPendingBreakdown;
        return `${stat.tooltipText} (${regular} Regular • ${security} Security)`;
      }
      return stat.tooltipText;
    default:
      return stat.tooltipText;
  }
};

/**
 * Computes pending update levels by matching productName and productBaseVersion
 * between recommended and product-update-levels responses.
 *
 * @param recommended - The recommended item.
 * @param productLevels - The product update levels response.
 * @returns {PendingUpdateLevelRow[]} Pending levels with type (security/regular).
 */
export const getPendingUpdateLevels = (
  recommended: RecommendedUpdateLevelItem,
  productLevels: ProductUpdateLevelsItem[] | undefined,
): PendingUpdateLevelRow[] => {
  if (!productLevels) {
    return [];
  }

  const product = productLevels.find(
    (p) => p.productName === recommended.productName,
  );
  if (!product) {
    return [];
  }

  const versionEntry = product.productUpdateLevels.find(
    (v) => v.productBaseVersion === recommended.productBaseVersion,
  );
  if (!versionEntry) {
    return [];
  }

  const { endingUpdateLevel, recommendedUpdateLevel } = recommended;
  const updateLevelSet = new Set(versionEntry.updateLevels);

  const pendingLevels: number[] = [];
  for (
    let level = endingUpdateLevel + 1;
    level <= recommendedUpdateLevel;
    level++
  ) {
    if (updateLevelSet.has(level)) {
      pendingLevels.push(level);
    }
  }

  const securityCount = recommended.availableSecurityUpdatesCount;

  return pendingLevels.map((level, index) => ({
    updateLevel: level,
    updateType:
      index < securityCount
        ? UpdatePendingLevelType.Security
        : UpdatePendingLevelType.Regular,
  }));
};
