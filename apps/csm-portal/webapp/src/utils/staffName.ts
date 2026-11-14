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

// The data appends a circled W (U+24E6, or the capital U+24CC) to the name of
// many users. It says nothing reliable about who they are, so it is never
// shown; a support engineer is marked by an icon, from the user's type.
const STAFF_NAME_MARK = /\s*[\u24CC\u24E6]\s*$/u;

/**
 * Removes the circled W from the end of a name.
 *
 * @param name - A person's name as the API returns it.
 * @returns {string} The name without the mark.
 */
export function stripStaffMark(name: string | null | undefined): string {
  return (name ?? "").replace(STAFF_NAME_MARK, "");
}
