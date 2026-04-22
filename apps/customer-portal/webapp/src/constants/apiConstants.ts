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

// Constants for API-related query keys.
export const ApiQueryKeys = {
  PROJECTS: "projects",
  PROJECT_DETAILS: "project-details",
  PROJECT_FEATURES: "project-features",
  SUPPORT_STATS: "support-stats",
  CASE_CREATION_METADATA: "case-creation-metadata",
  CASES_STATS: "cases-stats",
  DASHBOARD_STATS: "dashboard-stats",
  PROJECT_STATS: "project-stats",
  PROJECT_CASES: "project-cases",
  CASE_DETAILS: "case-details",
  CASE_COMMENTS: "case-comments",
  CASE_ATTACHMENTS: "case-attachments",
  CHAT_HISTORY: "chat-history",
  CONVERSATIONS_SEARCH: "conversations-search",
  CONVERSATION_MESSAGES: "conversation-messages",
  CONVERSATION_STATS: "conversation-stats",
  CONVERSATION_SUMMARY: "conversation-summary",
  CONVERSATION_RECOMMENDATIONS_SEARCH: "conversation-recommendations-search",
  DEPLOYMENTS: "deployments",
  DEPLOYMENT_ATTACHMENTS: "deployment-attachments",
  PRODUCTS: "products",
  PRODUCT_VERSIONS_SEARCH: "product-versions-search",
  DEPLOYMENT_PRODUCTS: "deployment-products",
  TIME_TRACKING_STATS: "time-tracking-stats",
  RECOMMENDED_UPDATE_LEVELS: "recommended-update-levels",
  PRODUCT_UPDATE_LEVELS: "product-update-levels",
  PRODUCT_VULNERABILITY: "product-vulnerability",
  PRODUCT_VULNERABILITIES_SEARCH: "product-vulnerabilities-search",
  PRODUCT_VULNERABILITIES_META: "product-vulnerabilities-meta",
  CASE_CALL_REQUESTS: "case-call-requests",
  PROJECT_CONTACTS: "project-contacts",
  TIME_TRACKING_DETAILS: "time-tracking-details",
  TIME_CARDS_SEARCH: "time-cards-search",
  PROJECT_DEPLOYMENT_DETAILS: "project-deployment-details",
  UPDATE_LEVELS_SEARCH: "update-levels-search",
  CHANGE_REQUESTS: "change-requests",
  CHANGE_REQUEST_DETAILS: "change-request-details",
  CHANGE_REQUEST_COMMENTS: "change-request-comments",
  CHANGE_REQUEST_STATS: "change-request-stats",
  CATALOGS_SEARCH: "catalogs-search",
  CATALOG_ITEM_VARIABLES: "catalog-item-variables",
  REGISTRY_TOKENS_SEARCH: "registry-tokens-search",
  INTEGRATION_USERS: "integration-users",
  METADATA: "metadata",
  PROJECT_USAGE_STATS: "project-usage-stats",
  DEPLOYMENT_INSTANCES_SEARCH: "deployment-instances-search",
  DEPLOYED_PRODUCT_INSTANCES_SEARCH: "deployed-product-instances-search",
  PROJECT_INSTANCES_SEARCH: "project-instances-search",
  PROJECT_INSTANCE_USAGES: "project-instance-usages",
  DEPLOYED_PRODUCT_INSTANCE_USAGES: "deployed-product-instance-usages",
  DEPLOYMENT_INSTANCE_USAGES: "deployment-instance-usages",
  PROJECT_INSTANCE_METRICS: "project-instance-metrics",
  DEPLOYMENT_INSTANCE_METRICS: "deployment-instance-metrics",
  DEPLOYED_PRODUCT_INSTANCE_METRICS: "deployed-product-instance-metrics",
} as const;

// Constants for API-related mutation keys.
export const ApiMutationKeys = {
  POST_COMMENT: ["postComment"],
  POST_CHANGE_REQUEST_COMMENT: ["postChangeRequestComment"],
} as const;

// Constants for WebSocket communication.
export const WS_CHOREO_OAUTH2_TOKEN = "choreo-oauth2-token";
export const WS_CUSTOMER_PORTAL = "cs-customer-portal";
export const CONNECT_HANDSHAKE_TIMEOUT_MS = 25_000;

/**
 * Backoff delays (ms) between ID-token retrieval attempts inside
 * `useAuthApiClient` (`resolveIdTokenWithRetry`). Array length equals the
 * maximum number of token retrieval attempts before surfacing failure.
 */
export const TOKEN_RETRY_DELAYS_MS = [150, 300, 600, 1000] as const;

/**
 * Asgardeo SPA SDK internal error code when `getIdToken()` runs before the auth
 * client is ready (observed with `@asgardeo/auth-react`; value may change on SDK
 * upgrades — re-verify against SDK release notes if token retrieval misbehaves).
 */
export const ASGARDEO_UNAUTHENTICATED_CODE = "SPA-AUTH_CLIENT-VM-IV02";

/**
 * Synthetic error message thrown when ID token cannot be obtained after retries
 * because auth is not ready (`getIdToken` kept returning unauthenticated).
 * Used by `useGetUserDetails` and other callers to retry without treating as a
 * hard failure.
 */
export const AUTH_NOT_READY_ERROR_MESSAGE = "Authentication is not ready yet";
