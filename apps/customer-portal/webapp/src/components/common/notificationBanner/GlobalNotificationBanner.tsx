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

import { NotificationBanner } from "@wso2/oxygen-ui";
import { useState, useEffect, type JSX } from "react";
import { notificationBannerConfig } from "@/config/notificationBannerConfig";

// Props for the GlobalNotificationBanner component.
interface GlobalNotificationBannerProps {
  visible: boolean;
}

/**
 * GlobalNotificationBanner component.
 *
 * @param {GlobalNotificationBannerProps} props - Component props.
 * @returns {JSX.Element | null} The GlobalNotificationBanner component.
 */
export default function GlobalNotificationBanner({
  visible,
}: GlobalNotificationBannerProps): JSX.Element | null {
  // State for the notification banner dismissal.
  const [dismissed, setDismissed] = useState<boolean>(false);

  // Reset the dismissed state when the visibility configuration changes to true.
  useEffect(() => {
    if (visible) {
      setDismissed(false);
    }
  }, [visible]);

  if (!visible || dismissed) {
    return null;
  }

  return (
    <NotificationBanner
      visible={true}
      severity={notificationBannerConfig.severity}
      title={notificationBannerConfig.title}
      message={notificationBannerConfig.message}
      actionLabel={notificationBannerConfig.actionLabel}
      onAction={() => {
        if (notificationBannerConfig.actionUrl) {
          window.open(
            notificationBannerConfig.actionUrl,
            "_blank",
            "noopener,noreferrer",
          );
        }
      }}
      onDismiss={() => setDismissed(true)}
    />
  );
}
