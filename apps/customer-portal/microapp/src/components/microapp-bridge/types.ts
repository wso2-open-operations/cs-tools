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

export const Topic = {
  token: "token",
  nativeLog: "native_log",
  navigateToMyApps: "close_webview",
  saveLocalData: "save_local_data",
  getLocalData: "get_local_data",
  deviceSafeAreaInsets: "device_safe_area_insets",
  deleteLocalData: "delete_local_data",
  openUrl: "open_url",
  scheduleLocalNotification: "scheduling_local_notification",
  cancelLocalNotification: "cancelling_local_notification",
  clearAllLocalNotifications: "clearing_all_local_notifications",
  qrRequest: "qr_request",
} as const;

export type TopicType = (typeof Topic)[keyof typeof Topic];

export type LogLevel = "error" | "warn" | "info" | "debug";
