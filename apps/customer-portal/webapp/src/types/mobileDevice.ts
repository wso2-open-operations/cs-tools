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

/** Mobile operating systems supported by the app-download prompt. */
export enum MobileOs {
  Ios = "ios",
  Android = "android",
}

/** Device form factor for mobile detection and copy. */
export enum DeviceType {
  Phone = "phone",
  Tablet = "tablet",
  Desktop = "desktop",
}

/** Navigator platform values used in device detection heuristics. */
export enum NavigatorPlatform {
  MacIntel = "MacIntel",
}

export interface MobileDeviceInfo {
  os: MobileOs;
  deviceType: DeviceType;
}

export interface DetectMobileDeviceOptions {
  /** When false (default), only phones trigger the mobile-app prompt. */
  includeTablets?: boolean;
}
