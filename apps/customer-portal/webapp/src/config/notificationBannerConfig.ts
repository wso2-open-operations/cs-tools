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

// Interface for the notification banner configuration.
export interface NotificationBannerConfig {
  visible: boolean;
  severity: "info" | "warning" | "error" | "success";
  title: string;
  message: string;
  actionLabel?: string;
  actionUrl?: string;
}

export const notificationBannerConfig: NotificationBannerConfig = {
  actionLabel: window.config?.CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL,
  actionUrl: window.config?.CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_URL,
  message: window.config?.CUSTOMER_PORTAL_MAINTENANCE_BANNER_MESSAGE || "",
  severity:
    (window.config?.CUSTOMER_PORTAL_MAINTENANCE_BANNER_SEVERITY as
      | "info"
      | "warning"
      | "error"
      | "success") || "info",
  title: window.config?.CUSTOMER_PORTAL_MAINTENANCE_BANNER_TITLE || "",
  visible: window.config?.CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE || false,
};
