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

// Base URL for the API service.
export const BACKEND_BASE_URL = import.meta.env
  .CUSTOMER_PORTAL_BACKEND_BASE_URL;

if (!BACKEND_BASE_URL) {
  throw new Error("Missing required env variables: baseUrl");
}

// Interface for the API configuration.
interface ApiConfig {
  backendUrl: string;
}

// Configuration for the API service.
export const apiConfig: ApiConfig = {
  backendUrl: BACKEND_BASE_URL,
};
