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

/** Product class values. Must match backend entity:ProductClass. */
export const PRODUCT_CLASS = {
  PRODUCT_MODEL: "product_model",
} as const;

/**
 * Update-levels stream per deployment profile, keyed by the product key the
 * update-levels service uses (the product abbreviation) and then by the
 * lower-cased deployment profile. A profile missing here has no stream of its
 * own and uses the product's.
 *
 * Names come from GET /updates/product-update-levels: API Manager ships its
 * control plane, universal gateway and traffic manager with level numbering
 * of their own (wso2am-acp, wso2am-universal-gw, wso2am-tm).
 */
export const UPDATE_STREAM_BY_DEPLOYMENT_PROFILE: Readonly<
  Record<string, Readonly<Record<string, string>>>
> = {
  wso2am: {
    "api control plane": "wso2am-acp",
    "universal gateway": "wso2am-universal-gw",
    "traffic manager": "wso2am-tm",
  },
};

/**
 * Deployment profiles that describe the ordinary, undivided deployment of a
 * product (lower-cased). They are the profile of most version records, so they
 * are not shown: a version with one of them keeps the plain label it has always
 * had. Only the profiles that tell otherwise identical version records apart
 * (API Control Plane, Traffic Manager, Key Manager, ...) are.
 */
export const DEFAULT_DEPLOYMENT_PROFILES: ReadonlySet<string> = new Set([
  "all in one",
  "standard",
]);
