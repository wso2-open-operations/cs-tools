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
 * Validates a route-param identifier by format. Ids reaching the SPL pages are
 * UUIDs, 32-character hex ids, or short record numbers, so a conservative
 * charset and length is enough; anything else is treated as "no id" (the same
 * as a missing param). Request builders still `encodeURIComponent` the value,
 * which is what keeps it inert inside a URL — this only rejects malformed input
 * early.
 */
export function safeRouteId(raw: string | undefined): string {
  if (!raw) return "";
  return /^[A-Za-z0-9_-]{1,64}$/.test(raw) ? raw : "";
}
