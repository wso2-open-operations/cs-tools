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

import { parseHours } from "@features/csm-reports/components/parseHours";

export type QueryHoursSlices = {
  consumed: number;
  remaining: number;
  consumedPct: number;
  remainingPct: number;
};

/**
 * Splits total query hours into consumed and remaining. Remaining is clamped
 * to [0, total], so an over-consumed project shows 0 remaining and a fully
 * consumed donut. A non-positive or unparsable total yields all zeros.
 */
export function computeQueryHoursSlices(remainingQueryHours: string, totalQueryHours: string): QueryHoursSlices {
  const total = parseHours(totalQueryHours);
  if (!Number.isFinite(total) || total <= 0) {
    return { consumed: 0, remaining: 0, consumedPct: 0, remainingPct: 0 };
  }
  const remaining = Math.min(Math.max(parseHours(remainingQueryHours), 0), total);
  const consumed = Math.max(total - remaining, 0);
  return {
    consumed,
    remaining,
    consumedPct: parseFloat(((consumed * 100) / total).toFixed(1)),
    remainingPct: parseFloat(((remaining * 100) / total).toFixed(1)),
  };
}
