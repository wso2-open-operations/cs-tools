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

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** "2024-12-07" to "7 Dec 2024"; parsed by hand so the viewer's time zone cannot shift the day. */
export function formatPeriodDate(value?: string): string {
  const match = value?.trim().match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) return "";
  const month = MONTHS[parseInt(match[2], 10) - 1];
  return month ? `${parseInt(match[3], 10)} ${month} ${match[1]}` : "";
}

/** The header text for the period the response covers. */
export function formatPeriod(start?: string, end?: string): string {
  const from = formatPeriodDate(start);
  const to = formatPeriodDate(end);
  if (from && to) return `Period: ${from} to ${to}`;
  if (from) return `Period: from ${from}`;
  if (to) return `Period: up to ${to}`;
  return "Period: all time";
}
