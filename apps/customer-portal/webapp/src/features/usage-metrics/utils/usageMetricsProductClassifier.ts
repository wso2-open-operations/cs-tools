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

/** WSO2 product families that have known metric mappings. */
export enum WsoProductType {
  APIM = "APIM",
  IS = "IS",
  MI = "MI",
  UNKNOWN = "UNKNOWN",
}

/** Chart appearance config for a single usage metric key. */
export type MetricChartConfig = {
  title: string;
  caption: string;
  stroke: string;
};

/** Appearance config for each known metric key. */
export const METRIC_CHART_CONFIG: Record<string, MetricChartConfig> = {
  TRANSACTION_COUNT: {
    title: "Transactions",
    caption: "Periodic transaction count",
    stroke: "#3B82F6",
  },
  API_COUNT: {
    title: "API Count",
    caption: "Unique APIs accessed per period",
    stroke: "#8B5CF6",
  },
  MCP_API_COUNT: {
    title: "MCP API Count",
    caption: "MCP API calls per period",
    stroke: "#14B8A6",
  },
  TOTAL_USERS: {
    title: "Total Users",
    caption: "Active users over time",
    stroke: "#22C55E",
  },
  TOTAL_ROOT_ORGS: {
    title: "Root Organizations",
    caption: "Root organizations over time",
    stroke: "#F59E0B",
  },
  TOTAL_B2B_ORGS: {
    title: "B2B Organizations",
    caption: "B2B organizations over time",
    stroke: "#F97316",
  },
};

export const METRIC_CHART_CONFIG_FALLBACK: MetricChartConfig = {
  title: "Metric",
  caption: "Metric over time",
  stroke: "#6B7280",
};

export const CORE_CHART_CONFIG: MetricChartConfig = {
  title: "Core Usage",
  caption: "Cores over time",
  stroke: "#F97316",
};

function parseMajorVersion(label: string): number {
  const m = label.match(/(\d+)\.\d+/);
  return m ? parseInt(m[1], 10) : 0;
}

const PRODUCT_TYPE_PATTERNS: [WsoProductType, RegExp][] = [
  [WsoProductType.APIM, /api[\s-]?manager|\bapim\b/i],
  [WsoProductType.IS, /identity[\s-]server/i],
  [WsoProductType.MI, /micro[\s-]integrator|\bintegrator\b/i],
];

/**
 * Classifies a product label into a WSO2 product type and extracts major version.
 *
 * @param label - Product display label (e.g. "WSO2 API Manager 4.4.0 All In One").
 * @returns Product type and major version number.
 */
export function classifyProductLabel(label: string): {
  type: WsoProductType;
  majorVersion: number;
} {
  const majorVersion = parseMajorVersion(label);
  for (const [type, pattern] of PRODUCT_TYPE_PATTERNS) {
    if (pattern.test(label)) {
      return { type, majorVersion };
    }
  }
  return { type: WsoProductType.UNKNOWN, majorVersion };
}

/**
 * Returns the ordered metric keys relevant for a given product label.
 * UNKNOWN products fall back to showing all common metric keys.
 *
 * @param label - Product display label used for type detection.
 * @param version - Explicit product version string (e.g. "7.0.0") as the primary
 *   source for major version selection. Falls back to parsing the label when omitted.
 * @returns Ordered list of metric keys to display.
 */
export function getProductMetricKeys(label: string, version?: string): string[] {
  const { type, majorVersion: labelMajorVersion } = classifyProductLabel(label);
  const majorVersion = version != null ? parseMajorVersion(version) : labelMajorVersion;
  switch (type) {
    case WsoProductType.APIM:
      return majorVersion >= 7
        ? ["TRANSACTION_COUNT", "API_COUNT", "MCP_API_COUNT"]
        : ["TRANSACTION_COUNT", "API_COUNT"];
    case WsoProductType.IS:
      return majorVersion >= 7
        ? ["TOTAL_B2B_ORGS", "TOTAL_ROOT_ORGS", "TOTAL_USERS"]
        : ["TOTAL_ROOT_ORGS", "TOTAL_USERS"];
    case WsoProductType.MI:
      return ["TRANSACTION_COUNT"];
    default:
      return ["TRANSACTION_COUNT", "API_COUNT", "TOTAL_USERS", "TOTAL_ROOT_ORGS"];
  }
}
