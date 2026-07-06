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

import MobileAppPromptPage from "@components/mobile-app/MobileAppPromptPage";
import {
  getMobileAppConfig,
  getMobileAppStoreUrl,
} from "@config/mobileAppConfig";
import type { MobileDeviceInfo } from "@/types/mobileDevice";
import { detectMobileDevice } from "@utils/deviceDetection";
import { type JSX, type ReactNode, useMemo } from "react";

export interface MobileAppGateProps {
  children: ReactNode;
}

/**
 * Blocks the web portal on detected iOS/Android phones (and optional tablets),
 * showing the WSO2 Super App download prompt. There is no in-browser bypass.
 *
 * @param {MobileAppGateProps} props - Child tree to render when not on mobile.
 * @returns {JSX.Element} Mobile prompt or children.
 */
export default function MobileAppGate({ children }: MobileAppGateProps): JSX.Element {
  const mobileAppConfig = useMemo(() => getMobileAppConfig(), []);
  const device = useMemo<MobileDeviceInfo | null>(
    () =>
      detectMobileDevice({ includeTablets: mobileAppConfig.includeTablets }),
    [mobileAppConfig.includeTablets],
  );

  const storeUrl = device ? getMobileAppStoreUrl(device.os, mobileAppConfig) : undefined;

  const shouldShowPrompt =
    mobileAppConfig.enabled && device !== null && !!storeUrl;

  if (shouldShowPrompt && device) {
    return <MobileAppPromptPage device={device} />;
  }

  return <>{children}</>;
}
