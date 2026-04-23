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

export const PROJECTS_ENDPOINT = "/projects/search";
export const PROJECT_DETAILS_ENDPOINT = (id: string) => `/projects/${id}`;
export const PROJECT_FEATURES_ENDPOINT = (id: string) => `/projects/${id}/features`;
export const PROJECT_STATS_ENDPOINT = (id: string) => `/projects/${id}/stats`;
export const PROJECT_CASES_ENDPOINT = (id: string) => `/projects/${id}/cases/search`;
export const PROJECT_CHATS_ENDPOINT = (id: string) => `/projects/${id}/conversations/search`;
export const PROJECT_CHANGE_REQUESTS_ENDPOINT = (id: string) => `/projects/${id}/change-requests/search`;
export const PROJECT_SERVICE_REQUESTS_ENDPOINT = (id: string) => `/projects/${id}/service-requests/search`;
export const PROJECT_CASES_FILTERS_ENDPOINT = (id: string) => `/projects/${id}/filters`;
export const PROJECT_DEPLOYMENTS_ENDPOINT = (id: string) => `/projects/${id}/deployments/search`;
export const PROJECT_DEPLOYMENT_PRODUCTS_ENDPOINT = (deploymentId: string) =>
  `/deployments/${deploymentId}/products/search`;
export const PROJECT_USERS_ENDPOINT = (id: string) => `/projects/${id}/contacts`;
export const PROJECT_USERS_VALIDATION_ENDPOINT = (id: string) => `/projects/${id}/contacts/validate`;
export const CREATE_CASE_ENDPOINT = "/cases";
export const CASE_CLASSIFICATION_ENDPOINT = "/cases/classify";
export const CASE_STATS_ENDPOINT = (id: string) => `/projects/${id}/stats/cases`;
export const CASE_DETAILS_ENDPOINT = (id: string) => `/cases/${id}`;
export const CASE_COMMENTS_ENDPOINT = (id: string) => `/cases/${id}/comments`;
export const CASE_CALL_REQUESTS_ENDPOINT = (id: string) => `/cases/${id}/call-requests/search`;
export const USERS_ME_ENDPOINT = "/users/me";
export const CHAT_INITIATE_ENDPOINT = (id: string) => `/projects/${id}/conversations`;
export const CHAT_ADD_MESSAGE_ENDPOINT = (id: string, conversationId: string) =>
  `/projects/${id}/conversations/${conversationId}/messages`;
export const CHAT_COMMENTS_ENDPOINT = (id: string) => `/conversations/${id}/messages`;
export const CHAT_DETAILS_ENDPOINT = (id: string) => `/conversations/${id}`;
export const CHANGE_REQUEST_DETAILS_ENDPOINT = (id: string) => `/change-requests/${id}`;
export const CHANGE_REQUEST_STATS_ENDPOINT = (id: string) => `/projects/${id}/stats/change-requests`;
export const USER_ACTIONS_ENDPOINT = (id: string, email: string) => `/projects/${id}/contacts/${email}`;
export const METADATA_ENDPOINT = "/metadata";
export const NOVERA_WEBSOCKET_INITIALIZATION_ENDPOINT = (sessionId: string) =>
  import.meta.env.VITE_BACKEND_URL.replace("https://", "wss://").replace(
    "/v1.0",
    `/websocket/v1.0/ws?sessionId=${sessionId}`,
  );

export const CHANGE_PASSWORD_URL = "https://wso2.com/user/password";
