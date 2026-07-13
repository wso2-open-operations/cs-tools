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

export const USERS_ME_ENDPOINT = "/users/me";

export const CASES_ENDPOINT = "/cases";
export const CASES_SEARCH_ENDPOINT = "/cases/search";
export const CASE_DETAILS_ENDPOINT = (id: string) => `/cases/${id}`;
export const CASE_COMMENTS_SEARCH_ENDPOINT = (id: string) => `/cases/${id}/comments/search`;

export const PROJECTS_SEARCH_ENDPOINT = "/projects/search";
export const DEPLOYMENTS_SEARCH_ENDPOINT = "/deployments/search";
export const DEPLOYMENT_PRODUCTS_SEARCH_ENDPOINT = (deploymentId: string) =>
  `/deployments/${deploymentId}/products/search`;

export const ATTACHMENTS_ENDPOINT = "/attachments";

export const TIME_CARDS_ENDPOINT = "/time-cards";
export const TIME_CARDS_SEARCH_ENDPOINT = "/time-cards/search";
export const TIME_CARD_ENDPOINT = (id: string) => `/time-cards/${id}`;
