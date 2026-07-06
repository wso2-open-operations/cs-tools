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
import {
  ApiError,
  getApiErrorMessage,
  isBadRequestError,
  isForbiddenError,
  isNotFoundError,
  isUnauthorizedError,
  parseApiResponseMessage,
} from "@utils/ApiError";

describe("ApiError", () => {
  it("carries status and custom message", () => {
    const err = new ApiError(403, "Forbidden", "No access");
    expect(err.name).toBe("ApiError");
    expect(err.status).toBe(403);
    expect(err.message).toBe("No access");
  });

  it("detects HTTP status helpers", () => {
    expect(isUnauthorizedError(new ApiError(401, "Unauthorized"))).toBe(true);
    expect(isForbiddenError(new ApiError(403, "Forbidden"))).toBe(true);
    expect(isNotFoundError(new ApiError(404, "Not Found"))).toBe(true);
    expect(isBadRequestError(new ApiError(400, "Bad Request"))).toBe(true);
    expect(isForbiddenError(new Error("x"))).toBe(false);
  });

  it("extracts API message when different from default", () => {
    const err = new ApiError(400, "Bad Request", "Invalid id");
    expect(getApiErrorMessage(err)).toBe("Invalid id");
    expect(getApiErrorMessage(new Error("x"))).toBeUndefined();
  });

  it("parseApiResponseMessage prefers JSON message", () => {
    expect(
      parseApiResponseMessage('{"message":"Case not found"}', 404, "Not Found"),
    ).toBe("Case not found");
    expect(parseApiResponseMessage("", 500, "")).toBe("HTTP 500");
  });
});
