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

/** Parses "36h 36m", "30m" or a plain number of hours into hours. */
export function parseHours(value: string): number {
  const trimmed = value.trim();
  const clock = trimmed.match(/(\d+)\s*h(?:\s*(\d+)\s*m)?/i);
  if (clock) {
    return parseInt(clock[1], 10) + parseInt(clock[2] || "0", 10) / 60;
  }
  const minutesOnly = trimmed.match(/^(\d+)\s*m$/i);
  if (minutesOnly) return parseInt(minutesOnly[1], 10) / 60;
  const numeric = Number(trimmed);
  return Number.isFinite(numeric) ? numeric : 0;
}
