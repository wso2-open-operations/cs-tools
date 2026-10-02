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

// Ported 1:1 from the source app's data/utils/formatters.ts.

export interface StatSummary {
  curr: number;
  avg: number;
  min: number;
  max: number;
}

export function fmtNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return n.toLocaleString();
}

export function fmtDate(iso: string): string {
  try {
    return new Date(iso + "T00:00:00").toLocaleDateString("en-US", { month: "short", day: "numeric" });
  } catch {
    return iso;
  }
}

export function extractVersion(updateLevel?: string, productName?: string): string {
  if (updateLevel && updateLevel !== "2147483647") {
    const m = updateLevel.match(/(\d+\.\d+\.\d+)/);
    if (m) return m[1];
  }
  // Fallback: extract from product name
  if (productName) {
    const m = productName.match(/(\d+\.\d+\.\d+)/);
    if (m) return m[1];
  }
  return "";
}

/** Strip escaped quotes from jdkVersion strings */
export function cleanJdkVersion(raw?: string): string {
  return (raw ?? "").replace(/"/g, "").trim();
}

export function computeStats(values: number[]): StatSummary {
  if (values.length === 0) return { curr: 0, avg: 0, min: 0, max: 0 };
  return {
    curr: values[values.length - 1],
    avg: Math.round(values.reduce((a, b) => a + b, 0) / values.length),
    min: Math.min(...values),
    max: Math.max(...values),
  };
}
