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

/**
 * Shape of `window.config` loaded from `public/config.js` at deploy/runtime.
 * Domain modules (auth, mobile app, banners, etc.) read subsets via dedicated config helpers.
 */
export interface CustomerPortalWindowConfig {
  CUSTOMER_PORTAL_AUTH_BASE_URL: string;
  CUSTOMER_PORTAL_AUTH_CLIENT_ID: string;
  CUSTOMER_PORTAL_AUTH_SIGN_IN_REDIRECT_URL: string;
  CUSTOMER_PORTAL_AUTH_SIGN_OUT_REDIRECT_URL: string;
  CUSTOMER_PORTAL_BACKEND_BASE_URL: string;
  CUSTOMER_PORTAL_THEME: string;
  CUSTOMER_PORTAL_LOG_LEVEL: string;
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_VISIBLE: boolean;
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_SEVERITY?: string;
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_TITLE?: string;
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_MESSAGE?: string;
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_LABEL?: string;
  CUSTOMER_PORTAL_MAINTENANCE_BANNER_ACTION_URL?: string;
  CUSTOMER_PORTAL_CHATBOT_WEBSOCKET_URL?: string;
  CUSTOMER_PORTAL_FLOATING_NOVERA_ENABLED?: boolean;
  CUSTOMER_PORTAL_TOP_BANNERS?: Array<{
    enabled: boolean;
    html: string;
    closeable: boolean;
    storageKey: string;
  }>;
  CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_VISIBLE?: boolean;
  CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_STORAGE_KEY?: string;
  CUSTOMER_PORTAL_ANNOUNCEMENT_BANNER_HTML?: string;
  CUSTOMER_PORTAL_UPDATES_USE_MOCK?: boolean;
  CUSTOMER_PORTAL_CONVERSATIONS_USE_MOCK?: boolean;
  CUSTOMER_PORTAL_CASES_USE_MOCK?: boolean;
  CUSTOMER_PORTAL_MOBILE_APP_PROMPT_ENABLED?: boolean;
  CUSTOMER_PORTAL_MOBILE_APP_IOS_STORE_URL?: string;
  CUSTOMER_PORTAL_MOBILE_APP_ANDROID_STORE_URL?: string;
  CUSTOMER_PORTAL_MOBILE_APP_INCLUDE_TABLETS?: boolean;
}

declare global {
  interface Window {
    config: CustomerPortalWindowConfig;
  }
}

export {};
