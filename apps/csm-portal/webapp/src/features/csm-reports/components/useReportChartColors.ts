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

import { useTheme } from "@wso2/oxygen-ui";
import { useMemo } from "react";
import { paletteColor } from "@features/csm-dashboard/utils/paletteColor";

/**
 * Same order as the dashboard bar and pie widgets, without `secondary`.
 * In Acrylic Orange that role is #E8E8E8, so a slice drawn from it disappears.
 * Further slices use the same oxygen ramps as the dashboard composition donuts.
 */
const EXTRA_SERIES = [
  paletteColor("deepOrange", 500, "#f4511e"),
  paletteColor("blue", 500, "#3b82f6"),
  paletteColor("indigo", 400, "#6366f1"),
  paletteColor("cyan", 500, "#06b6d4"),
  paletteColor("teal", 500, "#14b8a6"),
  paletteColor("purple", 500, "#a855f7"),
  paletteColor("green", 500, "#22c55e"),
];

/** Severity hues from the dashboard composition charts, matched by priority name. */
const NAMED_PRIORITY: [RegExp, string][] = [
  [/\b(s0|catastrophic)\b/i, paletteColor("red", 600, "#dc2626")],
  [/\b(s1|critical|urgent|highest)\b/i, paletteColor("deepOrange", 500, "#f4511e")],
  [/\b(s2|high)\b/i, paletteColor("amber", 700, "#b45309")],
  [/\b(s3|medium|moderate)\b/i, paletteColor("blue", 500, "#3b82f6")],
  [/\b(s4|low|query)\b/i, paletteColor("green", 500, "#22c55e")],
];

export function useReportChartColors(): string[] {
  const theme = useTheme();
  return useMemo(
    () => [
      theme.palette.primary.main,
      theme.palette.warning.main,
      theme.palette.info.main,
      theme.palette.success.main,
      theme.palette.error.main,
      ...EXTRA_SERIES,
    ],
    [theme.palette.primary.main, theme.palette.warning.main, theme.palette.info.main, theme.palette.success.main, theme.palette.error.main],
  );
}

export function priorityChartColor(name: string, index: number, series: string[], unknown: string): string {
  const key = name.trim();
  if (key === "" || key.toLowerCase() === "unknown") return unknown;
  const named = NAMED_PRIORITY.find(([pattern]) => pattern.test(key));
  return named ? named[1] : series[index % series.length];
}
