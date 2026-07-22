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

import { formatDate, parseOptionalBackendTimestamp } from "./dateTime";

/** Shared across Accounts/Projects/Deployments: a raw backend timestamp
 * (or null/undefined) to a display string, "—" when absent/unparseable. */
export function formatDateOnly(raw: string | null | undefined): string {
  const parsed = parseOptionalBackendTimestamp(raw);
  return parsed ? formatDate(parsed) : "—";
}

/** "cloud_support" -> "cloud support" — matches the webapp's formatSubscriptionType/
 * deploymentTypeLabel treatment of these underscore-separated enum values. */
export function formatEnumLabel(value: string): string {
  return value.replace(/_/g, " ");
}
