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

import { describe, expect, it } from "vitest";
import { ApiError } from "@utils/ApiError";
import { USERS_ME_MAX_RETRIES, shouldRetryUsersMe } from "./usersMeRetry";

describe("shouldRetryUsersMe", () => {
  it("retries 5xx responses", () => {
    expect(shouldRetryUsersMe(0, new ApiError(500, "Internal Server Error"))).toBe(true);
    expect(shouldRetryUsersMe(1, new ApiError(503, "Service Unavailable"))).toBe(true);
  });

  it("retries a network error that carries no status", () => {
    expect(shouldRetryUsersMe(0, new TypeError("Failed to fetch"))).toBe(true);
  });

  it("never retries 4xx responses, including 401 and 403", () => {
    for (const status of [400, 401, 403, 404]) {
      expect(shouldRetryUsersMe(0, new ApiError(status, "x"))).toBe(false);
    }
  });

  it("stops after the bounded number of retries", () => {
    const error = new ApiError(500, "Internal Server Error");
    expect(shouldRetryUsersMe(USERS_ME_MAX_RETRIES - 1, error)).toBe(true);
    expect(shouldRetryUsersMe(USERS_ME_MAX_RETRIES, error)).toBe(false);
  });
});
