// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
//
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

import axios, { AxiosInstance } from 'axios';
import { attach, RaxConfig } from 'retry-axios';

/**
 * Singleton Axios instance with Asgardeo ID token on each request and 401 retries (`retry-axios`).
 */
export class APIService {
  private static _instance: AxiosInstance;
  private static _callback: () => Promise<{ idToken: string }>;
  private static _initialized = false;
  private static _retryAxiosAttached = false;
  private static _authInterceptorId: number | null = null;

  /**
   * @param callback - Returns a fresh Asgardeo ID token on each request (and on retry).
   */
  constructor(callback: () => Promise<{ idToken: string }>) {
    if (!APIService._instance) {
      APIService._instance = axios.create({
        withCredentials: true,
      });
    }

    if (!APIService._retryAxiosAttached) {
      attach(APIService._instance);
      APIService._retryAxiosAttached = true;
    }

    APIService._callback = callback;
    APIService._initialized = true;
    APIService.updateRequestInterceptor();

    (APIService._instance.defaults as unknown as RaxConfig).raxConfig = {
      retry: 3,
      instance: APIService._instance,
      httpMethodsToRetry: ['GET', 'POST', 'PATCH', 'PUT', 'DELETE'],
      statusCodesToRetry: [[401, 401]],
      retryDelay: 100,

      onRetryAttempt: async () => {
        await callback();
        if (APIService._authInterceptorId !== null) {
          APIService._instance.interceptors.request.eject(APIService._authInterceptorId);
        }
        APIService.updateRequestInterceptor();
      },
    };
  }

  /** Shared Axios instance; creates a bare instance if `APIService` was never constructed with auth. */
  public static getInstance(): AxiosInstance {
    if (!APIService._instance) {
      APIService._instance = axios.create({
        withCredentials: true,
      });
    }

    if (!APIService._retryAxiosAttached) {
      attach(APIService._instance);
      APIService._retryAxiosAttached = true;
    }

    return APIService._instance;
  }

  /** Re-registers the request interceptor after ejecting the previous one (used on 401 retry). */
  private static updateRequestInterceptor() {
    if (APIService._authInterceptorId !== null) {
      APIService._instance.interceptors.request.eject(APIService._authInterceptorId);
    }

    APIService._authInterceptorId = APIService._instance.interceptors.request.use(
      async (config) => {
        if (APIService._callback && APIService._initialized) {
          try {
            const res = await APIService._callback();
            if (res.idToken) {
              config.headers.set('Authorization', `Bearer ${res.idToken}`);
            }
          } catch (error) {
            console.error('Failed to get token:', error);
          }
        }

        return config;
      },
      (error) => Promise.reject(error)
    );
  }
}
