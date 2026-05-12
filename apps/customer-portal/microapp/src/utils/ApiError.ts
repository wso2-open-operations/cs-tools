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

import { isAxiosError } from "axios";

interface ApiErrorMessageBody {
  message?: string;
}

export class ApiError extends Error {
  public readonly status: number;
  public readonly statusText: string;

  constructor(status: number, statusText: string, message?: string) {
    super(message ?? `${status} ${statusText}`);
    this.name = "ApiError";
    this.status = status;
    this.statusText = statusText;
  }
}

export function toApiError(error: unknown, fallbackMessage: string): Error {
  if (error instanceof ApiError) {
    return error;
  }

  if (isAxiosError<ApiErrorMessageBody>(error)) {
    const status = error.response?.status ?? 500;
    const statusText = error.response?.statusText ?? "Internal Server Error";
    const apiMessage = error.response?.data?.message;
    return new ApiError(status, statusText, apiMessage ?? fallbackMessage);
  }

  return error instanceof Error ? error : new Error(fallbackMessage);
}

export function getApiErrorMessage(error: unknown): string | undefined {
  if (!(error instanceof ApiError)) {
    return undefined;
  }

  const defaultMessage = `${error.status} ${error.statusText}`;
  return error.message !== defaultMessage ? error.message : undefined;
}
