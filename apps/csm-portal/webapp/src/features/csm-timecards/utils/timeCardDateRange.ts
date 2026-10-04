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

import { parseDateOnly } from "@utils/dateTime";

export interface DateRange {
  from: string;
  to: string;
}

/**
 * Next work-date range after the "From" field changes. Returns `null` when the
 * value is a future date (rejected). An empty/unparseable value only clears
 * "From"; "To" is moved only when both ends are valid dates and the new
 * "From" lands after it. Dates are compared parsed, not as strings.
 */
export function rangeAfterFromChange(
  value: string,
  current: DateRange,
  today: Date,
): DateRange | null {
  const next = parseDateOnly(value);
  if (next && next > today) return null;
  const to = parseDateOnly(current.to);
  if (next && to && next > to) return { from: value, to: value };
  return { from: next ? value : "", to: current.to };
}

/** Mirror of {@link rangeAfterFromChange} for the "To" field. */
export function rangeAfterToChange(
  value: string,
  current: DateRange,
  today: Date,
): DateRange | null {
  const next = parseDateOnly(value);
  if (next && next > today) return null;
  const from = parseDateOnly(current.from);
  if (next && from && next < from) return { from: value, to: value };
  return { from: current.from, to: next ? value : "" };
}
