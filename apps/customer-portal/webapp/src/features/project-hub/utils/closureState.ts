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

/**
 * A project's status as the portal shows it: "Open", "Read Only", "Pending Notified".
 * POST /search returns the raw lowercase value ("read_only"), so each word is capitalised.
 * Used by the status chips of the Projects tables and by the projects export, so the two
 * always say the same thing. A project with no status is "Active".
 *
 * @param value - The project's closure state as the API returns it.
 * @returns {string} The status to show.
 */
export function formatClosureState(value: string | null | undefined): string {
  if (!value) return "Active";
  return value
    .replace(/_/g, " ")
    .toLowerCase()
    .replace(/(^|[^a-z0-9])([a-z])/g, (_m, sep: string, ch: string) => sep + ch.toUpperCase());
}
