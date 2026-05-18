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

import { MobileOs } from "@/types/mobileDevice";

export interface MobileAppConfig {
  enabled: boolean;
  iosStoreUrl?: string;
  androidStoreUrl?: string;
  includeTablets: boolean;
}

/**
 * Reads mobile-app prompt settings from window.config.
 *
 * @returns {MobileAppConfig} Resolved mobile app configuration.
 */
export function getMobileAppConfig(): MobileAppConfig {
  const config = window.config;

  return {
    enabled: config?.CUSTOMER_PORTAL_MOBILE_APP_PROMPT_ENABLED ?? false,
    iosStoreUrl: config?.CUSTOMER_PORTAL_MOBILE_APP_IOS_STORE_URL,
    androidStoreUrl: config?.CUSTOMER_PORTAL_MOBILE_APP_ANDROID_STORE_URL,
    includeTablets:
      config?.CUSTOMER_PORTAL_MOBILE_APP_INCLUDE_TABLETS ?? false,
  };
}

/**
 * Resolves the app-store URL for the given mobile OS.
 *
 * @param {MobileOs} os - Target mobile operating system.
 * @param {MobileAppConfig} mobileAppConfig - Mobile app configuration.
 * @returns {string | undefined} Store URL when configured.
 */
export function getMobileAppStoreUrl(
  os: MobileOs,
  mobileAppConfig: MobileAppConfig = getMobileAppConfig(),
): string | undefined {
  return os === MobileOs.Ios
    ? mobileAppConfig.iosStoreUrl
    : mobileAppConfig.androidStoreUrl;
}
