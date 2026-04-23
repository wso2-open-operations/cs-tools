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

declare global {
  interface Window {
    config: {
      CLIENT_ID: string;
      SIGN_IN_REDIRECT_URL: string;
      SIGN_OUT_REDIRECT_URL: string;
      ASGARDEO_BASE_URL: string;
      IS_MICROAPP: boolean;
      BACKEND_BASE_URL: string;
    };
  }
}

export const CLIENT_ID = window.config.CLIENT_ID;
export const SIGN_IN_REDIRECT_URL = window.config.SIGN_IN_REDIRECT_URL;
export const SIGN_OUT_REDIRECT_URL = window.config.SIGN_OUT_REDIRECT_URL;
export const ASGARDEO_BASE_URL = window.config.ASGARDEO_BASE_URL;
export const IS_MICROAPP = window.config.IS_MICROAPP;

// TODO: Uncomment and update the `baseUrl` variable when the backend URL configuration is available and in use.
// const baseUrl = window.config.BACKEND_BASE_URL;

export const serviceUrls = {
  serviceUrls: {
    // TODO: Add service URLs here as needed for future implementation.
  },
};
