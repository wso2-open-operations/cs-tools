// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import axios, { AxiosInstance, CancelTokenSource } from 'axios';
import { attach, RaxConfig } from 'retry-axios';

export class APIService {
  private static _instance: AxiosInstance;
  private static _idToken: string;
  private static _cancelTokenSource = axios.CancelToken.source();
  private static _callback: () => Promise<{ idToken: string }>;
  private static _initialized = false;
  private static _authInterceptorId: number | null = null;

  constructor(idToken: string, callback: () => Promise<{ idToken: string }>) {
    // Only create a new instance if one doesn't exist
    if (!APIService._instance) {
      APIService._instance = axios.create({
        withCredentials: true,
      });
    }
    
    // Attach retry-axios to the instance
    attach(APIService._instance);

    APIService._idToken = idToken;
    APIService._callback = callback;
    APIService._initialized = true;
    APIService.updateRequestInterceptor();

    // Configure retry behavior with auth token refresh
    (APIService._instance.defaults as unknown as RaxConfig).raxConfig = {
      retry: 3,
      instance: APIService._instance,
      httpMethodsToRetry: ['GET', 'POST', 'PATCH', 'PUT', 'DELETE'],
      statusCodesToRetry: [[401, 401]],
      retryDelay: 100,

      onRetryAttempt: async (err) => {
        const res = await callback();
        APIService.updateTokens(res.idToken);
        // Eject only the auth interceptor, not all interceptors
        if (APIService._authInterceptorId !== null) {
          APIService._instance.interceptors.request.eject(APIService._authInterceptorId);
        }
        APIService.updateRequestInterceptor();
      },
    };
  }

  public static getInstance(): AxiosInstance {
    // Create instance if it doesn't exist
    if (!APIService._instance) {
      APIService._instance = axios.create({
        withCredentials: true,
      });
    }
    
    // Ensure interceptors are attached if not already initialized
    if (!APIService._initialized) {
      // Attach retry-axios without auth (for unauthenticated requests)
      attach(APIService._instance);
      
      // Mark as initialized to prevent duplicate attachment
      APIService._initialized = true;
    }
    
    return APIService._instance;
  }

  public static getCancelToken() {
    return APIService._cancelTokenSource;
  }

  public static updateCancelToken(): CancelTokenSource {
    APIService._cancelTokenSource = axios.CancelToken.source();
    return APIService._cancelTokenSource;
  }

  private static updateTokens(idToken: string) {
    APIService._idToken = idToken;
  }

  private static updateRequestInterceptor() {
    // Eject any previously registered auth interceptor to avoid duplicates
    if (APIService._authInterceptorId !== null) {
      APIService._instance.interceptors.request.eject(APIService._authInterceptorId);
    }
    
    // Register new auth interceptor and store its ID
    APIService._authInterceptorId = APIService._instance.interceptors.request.use(
      async (config) => {
        // Only add auth header if we have a callback (i.e., user is authenticated)
        if (APIService._callback && APIService._initialized) {
          try {
            let res = await APIService._callback();
            config.headers.set('Authorization', 'Bearer ' + res.idToken);
          } catch (error) {
            console.error('Failed to get token:', error);
          }
        }

        return config;
      },
      (error) => {
        return Promise.reject(error);
      }
    );
  }
}
