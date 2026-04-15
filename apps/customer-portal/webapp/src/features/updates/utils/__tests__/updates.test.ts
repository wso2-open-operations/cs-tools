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

import { describe, it, expect } from "vitest";
import {
  aggregateUpdateStats,
  getStatValue,
  getStatTooltipText,
  getPendingUpdateLevels,
  NULL_PLACEHOLDER,
} from "@features/updates/utils/updates";
import { UPDATES_STATS } from "@features/updates/constants/updatesConstants";
import type { RecommendedUpdateLevelItem } from "@features/updates/types/updates";

const createUpdateLevelItem = (
  overrides: Partial<RecommendedUpdateLevelItem> = {},
): RecommendedUpdateLevelItem => ({
  productName: "product",
  productBaseVersion: "1.0",
  channel: "full",
  startingUpdateLevel: 0,
  endingUpdateLevel: 10,
  installedUpdatesCount: 100,
  installedSecurityUpdatesCount: 20,
  timestamp: 0,
  recommendedUpdateLevel: 10,
  availableUpdatesCount: 50,
  availableSecurityUpdatesCount: 10,
  ...overrides,
});

describe("updates utilities", () => {
  describe("aggregateUpdateStats", () => {
    it("should return undefined if data is undefined", () => {
      expect(aggregateUpdateStats(undefined)).toBeUndefined();
    });

    it("should correctly aggregate stats for a multi-product dataset", () => {
      const data: RecommendedUpdateLevelItem[] = [
        createUpdateLevelItem({
          installedUpdatesCount: 200,
          installedSecurityUpdatesCount: 40,
          availableUpdatesCount: 60,
          availableSecurityUpdatesCount: 30,
        }),
        ...Array(19).fill(null).map(() =>
          createUpdateLevelItem({
            installedUpdatesCount: 183,
            installedSecurityUpdatesCount: 39,
            availableUpdatesCount: 55,
            availableSecurityUpdatesCount: 21,
          }),
        ),
      ];
      const stats = aggregateUpdateStats(data);
      expect(stats).toBeDefined();
      if (!stats) return;

      expect(stats.productsTracked).toBe(20);
      expect(stats.totalUpdatesInstalled).toBe(4458);
      expect(stats.totalUpdatesPending).toBe(1534);
      expect(stats.securityUpdatesPending).toBe(429);
      expect(stats.totalUpdatesInstalledBreakdown!.regular).toBe(3667);
      expect(stats.totalUpdatesInstalledBreakdown!.security).toBe(791);
      expect(stats.totalUpdatesPendingBreakdown!.regular).toBe(1105);
      expect(stats.totalUpdatesPendingBreakdown!.security).toBe(429);
    });
  });

  describe("getStatValue", () => {
    const mockStats = {
      productsTracked: 10,
      totalUpdatesInstalled: 100,
      totalUpdatesInstalledBreakdown: { regular: 80, security: 20 },
      totalUpdatesPending: 50,
      totalUpdatesPendingBreakdown: { regular: 30, security: 20 },
      securityUpdatesPending: 20,
    };

    it("should return NULL_PLACEHOLDER if aggregatedData is undefined", () => {
      expect(getStatValue(undefined, "productsTracked")).toBe(NULL_PLACEHOLDER);
    });

    it("should return the correct value for a given key", () => {
      expect(getStatValue(mockStats, "productsTracked")).toBe(10);
      expect(getStatValue(mockStats, "totalUpdatesInstalled")).toBe(100);
    });

    it("should return NULL_PLACEHOLDER if value is an object", () => {
      expect(
        getStatValue(mockStats, "totalUpdatesInstalledBreakdown" as any),
      ).toBe(NULL_PLACEHOLDER);
    });
  });

  describe("getStatTooltipText", () => {
    const mockStats = {
      productsTracked: 10,
      totalUpdatesInstalled: 100,
      totalUpdatesInstalledBreakdown: { regular: 80, security: 20 },
      totalUpdatesPending: 50,
      totalUpdatesPendingBreakdown: { regular: 30, security: 20 },
      securityUpdatesPending: 20,
    };

    const installedStat = UPDATES_STATS.find(
      (s) => s.id === "totalUpdatesInstalled",
    )!;
    const productStat = UPDATES_STATS.find((s) => s.id === "productsTracked")!;

    it("should return default tooltip if aggregatedData is undefined", () => {
      expect(getStatTooltipText(installedStat, undefined)).toBe(
        installedStat.tooltipText,
      );
    });

    it("should include breakdown for installed updates", () => {
      const tooltip = getStatTooltipText(installedStat, mockStats);
      expect(tooltip).toContain(installedStat.tooltipText);
      expect(tooltip).toContain("80 Regular • 20 Security");
    });

    it("should return default tooltip for non-breakdown stats", () => {
      expect(getStatTooltipText(productStat, mockStats)).toBe(
        productStat.tooltipText,
      );
    });
  });

  describe("getPendingUpdateLevels", () => {
    const recommended: RecommendedUpdateLevelItem = createUpdateLevelItem({
      productName: "wso2am",
      productBaseVersion: "4.2.0",
      endingUpdateLevel: 11,
      recommendedUpdateLevel: 22,
      availableSecurityUpdatesCount: 9,
    });

    const productLevels = [
      {
        productName: "wso2am",
        productUpdateLevels: [
          {
            productBaseVersion: "4.2.0",
            channel: "full",
            updateLevels: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22],
          },
        ],
      },
    ];

    it("returns empty array when productLevels is undefined", () => {
      expect(getPendingUpdateLevels(recommended, undefined)).toEqual([]);
    });

    it("returns empty array when productName does not match", () => {
      const levels = [{ ...productLevels[0], productName: "other" }];
      expect(getPendingUpdateLevels(recommended, levels)).toEqual([]);
    });

    it("returns empty array when productBaseVersion does not match", () => {
      const levels = [
        {
          ...productLevels[0],
          productUpdateLevels: [{ ...productLevels[0].productUpdateLevels[0], productBaseVersion: "5.0.0" }],
        },
      ];
      expect(getPendingUpdateLevels(recommended, levels)).toEqual([]);
    });

    it("returns pending levels between endingUpdateLevel+1 and recommendedUpdateLevel", () => {
      const rows = getPendingUpdateLevels(recommended, productLevels);
      expect(rows.map((r) => r.updateLevel)).toEqual([
        12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22,
      ]);
    });

    it("assigns first availableSecurityUpdatesCount as security, rest as regular", () => {
      const rows = getPendingUpdateLevels(recommended, productLevels);
      const security = rows.filter((r) => r.updateType === "security");
      const regular = rows.filter((r) => r.updateType === "regular");
      expect(security.length).toBe(9);
      expect(regular.length).toBe(2);
    });
  });
});
