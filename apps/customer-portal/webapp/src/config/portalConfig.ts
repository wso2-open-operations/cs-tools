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
  /** Enables the Novera chat "request a token limit increase" flow. */
  CUSTOMER_PORTAL_NOVERA_TOKEN_REQUEST_ENABLED?: boolean;
  /** Enables 👍/👎 answer feedback, with its reason tags, on Novera answers. */
  CUSTOMER_PORTAL_NOVERA_FEEDBACK_ENABLED?: boolean;
  CUSTOMER_PORTAL_TOP_BANNERS?: Array<{
    enabled: boolean;
    html: string;
    closeable: boolean;
    storageKey: string;
    startsAt?: string;
    expiresAt?: string;
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
  /**
   * Enables the case-creation "add deployment/product inline" empty-state
   * (alert + in-menu add-new row + wizard + singleton auto-select). Default
   * ON when unset - this is a kill switch, not an opt-in.
   */
  CUSTOMER_PORTAL_ALLOW_DEPLOYMENT_SETUP_DURING_CASE_CREATION?: boolean;
  /**
   * Kill switch for the Pending Verification settings tab, hidden until the
   * auto-closure scheduler integration is deployed. Default ON when unset —
   * flip to false to hide the tab.
   */
  CUSTOMER_PORTAL_ALLOW_PENDING_VERIFICATION_SETTINGS?: boolean;
  /**
   * Base URL of customer-portal-activity-stream-service, the case-activity
   * SSE stream. A separate Choreo component from the main backend, so this
   * is its own key rather than being derived from
   * CUSTOMER_PORTAL_BACKEND_BASE_URL. Optional: that service only stands the
   * stream listener up when Event Hub is configured, so a deployment without
   * it simply leaves this unset and useCaseActivityStream no-ops.
   */
  CUSTOMER_PORTAL_STREAM_BASE_URL?: string;
  /**
   * Master on/off switch for the case-activity SSE stream, independent of
   * whether CUSTOMER_PORTAL_STREAM_BASE_URL is set. Defaults to false, so a
   * config predating this key never turns the stream on.
   */
  CUSTOMER_PORTAL_STREAM_ENABLED?: boolean;
  /**
   * Uses entity-service's stored-attachment routes (files in SFTPGo) for
   * downloading, uploading and deleting case and deployment attachments, and for
   * showing comment images. Defaults to false, so a config predating this key
   * keeps the existing flow.
   */
  CUSTOMER_PORTAL_ENTITY_ATTACHMENTS_ENABLED?: boolean;
  /**
   * Largest work item or deployment file offered for upload, in MB (a number or
   * numeric string). Defaults to 100, entity-service's own default; set it to
   * match the service's `ATTACHMENT_MAX_BYTES`. The service stays authoritative.
   */
  CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB?: number | string;
  /**
   * Largest pasted or inserted comment image, in MB (a number or numeric
   * string). Defaults to 10, entity-service's `INLINE_IMAGE_MAX_BYTES` default.
   */
  CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB?: number | string;
}

declare global {
  interface Window {
    config: CustomerPortalWindowConfig;
  }
}

export {};
