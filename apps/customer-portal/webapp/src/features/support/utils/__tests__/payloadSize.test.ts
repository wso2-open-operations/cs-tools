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
  checkPayloadSize,
  payloadSizeErrorMessage,
} from "@features/support/utils/payloadSize";
import { MAX_CASE_REQUEST_BODY_BYTES } from "@features/support/constants/supportConstants";

describe("checkPayloadSize", () => {
  it("accepts a body at or under the cap", () => {
    expect(checkPayloadSize("x".repeat(5), 5)).toEqual({ ok: true });
    expect(checkPayloadSize("d", MAX_CASE_REQUEST_BODY_BYTES)).toEqual({ ok: true });
  });

  it("rejects a body over the cap and reports its size", () => {
    expect(checkPayloadSize("x".repeat(10), 5)).toEqual({ ok: false, totalBytes: 10 });
  });

  it("counts bytes, not characters", () => {
    expect(checkPayloadSize("é".repeat(3), 5)).toEqual({ ok: false, totalBytes: 6 });
  });
});

describe("payloadSizeErrorMessage", () => {
  it("names the subject and the cap", () => {
    const result = checkPayloadSize(
      "x".repeat(MAX_CASE_REQUEST_BODY_BYTES + 1),
      MAX_CASE_REQUEST_BODY_BYTES,
    );
    expect(result.ok).toBe(false);
    if (!result.ok) {
      const message = payloadSizeErrorMessage(
        result,
        "case description",
        MAX_CASE_REQUEST_BODY_BYTES,
      );
      expect(message).toContain("The case description exceeds the");
      expect(message).toContain("inline images");
    }
  });
});
