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
  parseApiResponseErrorCode,
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

describe("ApiError.code", () => {
  it("carries the machine-readable code when it is given, and none when it is not", () => {
    expect(new ApiError(409, "Conflict", "m", undefined, "change_request_on_hold").code).toBe("change_request_on_hold");
    expect(new ApiError(409, "Conflict", "m").code).toBeUndefined();
    expect(new ApiError(409, "Conflict", "m", "corr-1").correlationId).toBe("corr-1");
  });
});

describe("parseApiResponseErrorCode", () => {
  it("reads errorCode from a JSON error body", () => {
    expect(
      parseApiResponseErrorCode('{"message":"on hold","errorCode":"change_request_on_hold"}'),
    ).toBe("change_request_on_hold");
  });

  it("is undefined when the body names no code", () => {
    expect(parseApiResponseErrorCode('{"message":"on hold"}')).toBeUndefined();
    expect(parseApiResponseErrorCode("")).toBeUndefined();
    expect(parseApiResponseErrorCode("<html>502 Bad Gateway</html>")).toBeUndefined();
    expect(parseApiResponseErrorCode("null")).toBeUndefined();
    expect(parseApiResponseErrorCode("[]")).toBeUndefined();
  });

  it("is undefined for a value that is not a plain lower-case name", () => {
    for (const code of ["Change_Request", "on hold", "<b>x</b>", "-x", "a__b", "", "a".repeat(65), 7, null, {}, ["a"]]) {
      expect(parseApiResponseErrorCode(JSON.stringify({ message: "m", errorCode: code })), JSON.stringify(code)).toBeUndefined();
    }
  });
});
