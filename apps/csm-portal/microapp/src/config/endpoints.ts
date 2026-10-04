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

export const BACKEND_URL = import.meta.env.VITE_BACKEND_URL;

if (!BACKEND_URL) {
  throw new Error("VITE_BACKEND_URL is not defined");
}

// Base URL for the case-activity SSE stream — csm-portal-backend's dedicated
// :9092 listener, exposed as its own Choreo REST endpoint (see that
// backend's .choreo/component.yaml). Optional, unlike BACKEND_URL: the
// backend only stands the stream listener up when Event Hub is configured,
// so a build without it just has no value here. useCaseActivityStream
// checks for that and no-ops rather than throwing — the Activities tab
// falls back to its existing manual-refresh behaviour.
export const STREAM_URL = import.meta.env.VITE_STREAM_URL;

export const USERS_ME_ENDPOINT = "/users/me";
export const USERS_SEARCH_ENDPOINT = "/users/search";

export const CASES_ENDPOINT = "/cases";
export const CASES_SEARCH_ENDPOINT = "/cases/search";
export const CASES_AGGREGATE_ENDPOINT = "/cases/aggregate";
export const CASE_DETAILS_ENDPOINT = (id: string) => `/cases/${id}`;
export const CASE_COMMENTS_SEARCH_ENDPOINT = (id: string) => `/cases/${id}/comments/search`;
export const CASE_COMMENTS_ENDPOINT = (id: string) => `/cases/${id}/comments`;
export const CASE_CALL_REQUESTS_ENDPOINT = (id: string) => `/cases/${id}/call-requests`;
export const CASE_CALL_REQUESTS_SEARCH_ENDPOINT = (id: string) => `/cases/${id}/call-requests/search`;
export const CASE_CALL_REQUEST_ENDPOINT = (caseId: string, callRequestId: string) =>
  `/cases/${caseId}/call-requests/${callRequestId}`;
export const CASE_ACTIVITIES_SEARCH_ENDPOINT = (id: string) => `/cases/${id}/activities/search`;

export const SLAS_SEARCH_ENDPOINT = "/slas/search";

export const CHANGE_REQUESTS_ENDPOINT = "/change-requests";
export const CHANGE_REQUESTS_SEARCH_ENDPOINT = "/change-requests/search";
export const CHANGE_REQUEST_ENDPOINT = (id: string) => `/change-requests/${id}`;

export const GROUPS_SEARCH_ENDPOINT = "/groups/search";
export const IT_SERVICES_SEARCH_ENDPOINT = "/services/search";
export const SERVICE_OFFERINGS_SEARCH_ENDPOINT = "/service-offerings/search";
export const CONFIGURATION_ITEMS_SEARCH_ENDPOINT = "/configuration-items/search";

export const INCIDENTS_ENDPOINT = "/incidents";
export const INCIDENTS_SEARCH_ENDPOINT = "/incidents/search";
export const INCIDENT_ENDPOINT = (id: string) => `/incidents/${id}`;
export const INCIDENT_COMMENTS_SEARCH_ENDPOINT = (id: string) => `/incidents/${id}/comments/search`;

export const ACCOUNTS_SEARCH_ENDPOINT = "/accounts/search";
export const ACCOUNT_ENDPOINT = (id: string) => `/accounts/${id}`;

export const PROJECTS_SEARCH_ENDPOINT = "/projects/search";
export const PROJECT_ENDPOINT = (id: string) => `/projects/${id}`;
export const PRODUCTS_SEARCH_ENDPOINT = "/products/search";
export const PRODUCT_VULNERABILITIES_SEARCH_ENDPOINT = "/products/vulnerabilities/search";
export const PRODUCT_VULNERABILITY_ENDPOINT = (id: string) => `/products/vulnerabilities/${id}`;
export const DEPLOYMENTS_SEARCH_ENDPOINT = "/deployments/search";
export const DEPLOYMENT_PRODUCTS_SEARCH_ENDPOINT = (deploymentId: string) =>
  `/deployments/${deploymentId}/products/search`;

export const CATALOGS_SEARCH_ENDPOINT = "/catalogs/search";
export const CATALOG_ITEM_VARIABLES_ENDPOINT = (catalogId: string, catalogItemId: string) =>
  `/catalogs/${catalogId}/items/${catalogItemId}/variables`;

export const ATTACHMENTS_ENDPOINT = "/attachments";
export const ATTACHMENTS_SEARCH_ENDPOINT = "/attachments/search";
export const ATTACHMENT_CONTENT_ENDPOINT = (id: string) => `/attachments/${id}/content`;

export const TIME_CARDS_ENDPOINT = "/time-cards";
export const TIME_CARDS_SEARCH_ENDPOINT = "/time-cards/search";
export const TIME_CARD_ENDPOINT = (id: string) => `/time-cards/${id}`;

export const PRODUCT_UPDATE_LEVELS_ENDPOINT = "/updates/product-update-levels";
export const UPDATE_LEVELS_SEARCH_ENDPOINT = "/updates/levels/search";
