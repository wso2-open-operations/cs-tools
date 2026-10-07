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

// Ported 1:1 from the source app's data/utils/usageMetricsAccent.ts.

import { alpha } from "@mui/material/styles";

/** Resolved accent tokens for a deployment's cards/accordions. */
export interface UsageAccent {
  main: string;
  title: string;
  stroke: string;
  border: string;
  headerBg: string;
  headerHoverBg: string;
  iconWellBg: string;
  iconColor: string;
  statTileBg: string;
}

// Deterministic 6-colour palette — mirrors customer-portal's usage-metrics accents.
const ACCENT_MAIN_PALETTE = ["#EA580C", "#2563EB", "#16A34A", "#9333EA", "#0891B2", "#DC2626"];
const ACCENT_TITLE_PALETTE = ["#9A3412", "#1E40AF", "#166534", "#6B21A8", "#155E75", "#991B1B"];

/** Stable palette index from an id/name string. */
export function accentIndexForKey(key: string): number {
  let h = 0;
  for (let i = 0; i < key.length; i++) h += key.charCodeAt(i);
  return h % ACCENT_MAIN_PALETTE.length;
}

function buildAccent(main: string, title: string): UsageAccent {
  return {
    main,
    title,
    stroke: main,
    border: alpha(main, 0.2),
    headerBg: alpha(main, 0.08),
    headerHoverBg: alpha(main, 0.12),
    iconWellBg: alpha(main, 0.15),
    iconColor: main,
    statTileBg: alpha(main, 0.08),
  };
}

/** Resolve the accent tokens for a deployment, keyed by a stable id or name. */
export function getUsageAccent(key: string): UsageAccent {
  const idx = accentIndexForKey(key || "");
  return buildAccent(ACCENT_MAIN_PALETTE[idx], ACCENT_TITLE_PALETTE[idx]);
}

/** Single fixed accent (blue) used everywhere instead of the per-deployment hash color. */
export const USAGE_ACCENT_FIXED: UsageAccent = buildAccent(ACCENT_MAIN_PALETTE[1], ACCENT_TITLE_PALETTE[1]);
