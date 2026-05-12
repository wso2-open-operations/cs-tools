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

export interface AnnouncementBannerConfig {
  /** Set to true to show the banner. When false the banner is never rendered. */
  visible: boolean;
  /** Unique key used in localStorage to remember whether the user dismissed this banner.
   *  Change the key whenever you update the banner content so it shows again. */
  storageKey: string;
  /** Complete self-contained HTML rendered as the banner.
   *  Include all wrapper styles (background, padding, colors) and a close button
   *  with data-close-banner attribute to trigger dismissal. */
  html: string;
}

export const announcementBannerConfig: AnnouncementBannerConfig = {
  visible: window.config?.CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE ?? false,
  storageKey:
    window.config?.CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY ??
    "announcement_banner_v1",
  html: window.config?.CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_HTML ?? "",
};
