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

import { alpha, colors } from "@wso2/oxygen-ui";
import type {
  UsageEnvironmentPanelAccent,
  UsageOverviewEnvironmentAccent,
} from "@features/usage-metrics/types/usageMetrics";

const ACCENT_MAIN_PALETTE = [
  colors.orange?.[600] ?? "#EA580C",
  colors.blue?.[600] ?? "#2563EB",
  colors.green?.[600] ?? "#16A34A",
  colors.purple?.[600] ?? "#9333EA",
  colors.cyan?.[600] ?? "#0891B2",
  colors.red?.[600] ?? "#DC2626",
];

const ACCENT_TITLE_PALETTE = [
  colors.orange?.[800] ?? "#9A3412",
  colors.blue?.[800] ?? "#1E40AF",
  colors.green?.[800] ?? "#166534",
  colors.purple?.[800] ?? "#6B21A8",
  colors.cyan?.[800] ?? "#155E75",
  colors.red?.[800] ?? "#991B1B",
];

/**
 * Stable palette index from a type id string (e.g. deployment type).
 *
 * @param typeId - Type or deployment discriminator.
 * @returns Index into accent palettes.
 */
export function accentIndexForTypeId(typeId: string): number {
  let h = 0;
  for (let i = 0; i < typeId.length; i++) h += typeId.charCodeAt(i);
  return h % ACCENT_MAIN_PALETTE.length;
}

/**
 * Overview environment accordion colours (border, header, icon well).
 *
 * @param typeId - Deployment type id for palette selection.
 * @returns {UsageOverviewEnvironmentAccent} Resolved theme tokens.
 */
export function getUsageOverviewAccentForTypeId(
  typeId: string,
): UsageOverviewEnvironmentAccent {
  const idx = accentIndexForTypeId(typeId);
  const main = ACCENT_MAIN_PALETTE[idx];
  const title = ACCENT_TITLE_PALETTE[idx];
  return {
    main,
    title,
    border: alpha(main, 0.2),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
    statTileBg: alpha(main, 0.08),
  };
}

/**
 * Environment products tab card / accordion accents.
 *
 * @param typeId - Deployment type id for palette selection.
 * @returns {UsageEnvironmentPanelAccent} Resolved theme tokens.
 */
export function getUsageEnvironmentPanelAccent(
  typeId: string,
): UsageEnvironmentPanelAccent {
  const main = ACCENT_MAIN_PALETTE[accentIndexForTypeId(typeId)];
  return {
    main,
    stroke: main,
    borderDefault: alpha(main, 0.15),
    borderHover: alpha(main, 0.35),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
  };
}
