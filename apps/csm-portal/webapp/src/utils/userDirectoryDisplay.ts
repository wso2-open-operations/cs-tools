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

/** Normalise empty timezone values emitted by the supported user sources. */
export function displayUserTimezone(timezone: string | null | undefined): string {
  const value = timezone?.trim();
  return !value || /^-*none-*$/i.test(value) ? "—" : value;
}

/**
 * Human label for a user's `userType`. The two backing data sources disagree
 * on the external-user label (postgres emits `customer`, ServiceNow emits
 * `external` — see `csmUsers.ts`'s own `UserType` doc comment), so both map
 * to the same "External (customer)" label here rather than asking every
 * caller to know about the discrepancy.
 */
export function displayUserType(userType: string | null | undefined): string {
  switch (userType) {
    case "internal":
      return "Internal";
    case "customer":
    case "external":
      return "External (customer)";
    case "system":
      return "System";
    default:
      return "—";
  }
}
