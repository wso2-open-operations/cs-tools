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

import {
  DeviceType,
  MobileOs,
  NavigatorPlatform,
  type DetectMobileDeviceOptions,
  type MobileDeviceInfo,
} from "@/types/mobileDevice";

/**
 * Returns true when the browser reports an iPad (including iPadOS desktop UA).
 *
 * @param {string} userAgent - Navigator user agent string.
 * @returns {boolean} Whether the device is treated as an iPad.
 */
function isIPadUserAgent(userAgent: string): boolean {
  if (/iPad/i.test(userAgent)) {
    return true;
  }

  if (typeof navigator === "undefined") {
    return false;
  }

  return (
    navigator.platform === NavigatorPlatform.MacIntel &&
    navigator.maxTouchPoints > 1 &&
    !/iPhone|iPod/i.test(userAgent)
  );
}

/**
 * Classifies Android as phone or tablet using the Mobile token in the user agent.
 *
 * @param {string} userAgent - Navigator user agent string.
 * @returns {DeviceType | null} phone, tablet, or null when not Android.
 */
function getAndroidDeviceType(userAgent: string): DeviceType | null {
  if (!/Android/i.test(userAgent)) {
    return null;
  }

  return /Mobile/i.test(userAgent) ? DeviceType.Phone : DeviceType.Tablet;
}

/**
 * Detects iOS or Android mobile devices and distinguishes phones from tablets.
 *
 * @param {DetectMobileDeviceOptions} [options] - Detection options.
 * @returns {MobileDeviceInfo | null} Device info for supported mobile OSes, else null.
 */
export function detectMobileDevice(
  options: DetectMobileDeviceOptions = {},
): MobileDeviceInfo | null {
  if (typeof navigator === "undefined") {
    return null;
  }

  const { includeTablets = false } = options;
  const userAgent = navigator.userAgent;

  if (/iPhone|iPod/i.test(userAgent)) {
    return { os: MobileOs.Ios, deviceType: DeviceType.Phone };
  }

  if (isIPadUserAgent(userAgent)) {
    if (!includeTablets) {
      return null;
    }
    return { os: MobileOs.Ios, deviceType: DeviceType.Tablet };
  }

  const androidType = getAndroidDeviceType(userAgent);
  if (androidType) {
    if (androidType === DeviceType.Tablet && !includeTablets) {
      return null;
    }
    return { os: MobileOs.Android, deviceType: androidType };
  }

  return null;
}

/**
 * Returns true when the current device should see the mobile-app download prompt.
 *
 * @param {DetectMobileDeviceOptions} [options] - Forwarded to {@link detectMobileDevice}.
 * @returns {boolean} Whether the prompt should be shown.
 */
export function shouldPromptForMobileApp(
  options?: DetectMobileDeviceOptions,
): boolean {
  return detectMobileDevice(options) !== null;
}
