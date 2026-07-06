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

import type { SubscriptionType } from "@features/csm-projects/types/csmProjects";

/**
 * Cloud-support subscription types. For these, a case is filed against the
 * project's single primary-production deployment, so the case-creation form
 * hides the deployment picker and auto-selects it (mirrors the customer
 * portal's `shouldRestrictToPrimaryProductionDeployments`). Managed-cloud and
 * other subscriptions keep the normal deployment → product cascade.
 */
export const CLOUD_SUPPORT_SUBSCRIPTION_TYPES: readonly SubscriptionType[] = [
  "cloud_support",
  "cloud_evaluation_support",
];

/** True when the subscription type files cases directly against the primary
 *  production deployment (no deployment selection step). */
export function isCloudSupportSubscription(
  type: SubscriptionType | null | undefined,
): boolean {
  return !!type && CLOUD_SUPPORT_SUBSCRIPTION_TYPES.includes(type);
}
