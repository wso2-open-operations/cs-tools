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
  base64DecodedBytes,
  checkPayloadSize,
  payloadSizeErrorMessage,
} from "@features/support/utils/payloadSize";
import {
  MAX_ATTACHMENT_SIZE_BYTES,
  MAX_CASE_REQUEST_BODY_BYTES,
} from "@features/support/constants/supportConstants";

const MB = 1024 * 1024;
// Base64 string that decodes to `rawBytes` bytes (rawBytes must be a multiple of 3).
const b64 = (rawBytes: number) => "A".repeat((rawBytes / 3) * 4);
const body = (description: string, attachments: { name: string; file: string }[]) =>
  JSON.stringify({ description, attachments });

describe("base64DecodedBytes", () => {
  it("accounts for padding", () => {
    expect(base64DecodedBytes("QUJD")).toBe(3);
    expect(base64DecodedBytes("QUI=")).toBe(2);
    expect(base64DecodedBytes("QQ==")).toBe(1);
  });
});

describe("checkPayloadSize", () => {
  it("accepts an 8 MB file", () => {
    const attachments = [{ name: "a.zip", file: b64(8 * MB) }];
    expect(
      checkPayloadSize(body("d", attachments), MAX_CASE_REQUEST_BODY_BYTES, attachments),
    ).toEqual({ ok: true });
  });

  it("accepts a file of exactly the per-file limit", () => {
    const attachments = [{ name: "a.zip", file: b64(MAX_ATTACHMENT_SIZE_BYTES) }];
    expect(
      checkPayloadSize(body("d", attachments), MAX_CASE_REQUEST_BODY_BYTES, attachments),
    ).toEqual({ ok: true });
  });

  it("rejects a file just over the raw per-file limit and names it", () => {
    const attachments = [{ name: "big.zip", file: b64(MAX_ATTACHMENT_SIZE_BYTES + 3) }];
    const result = checkPayloadSize(
      body("d", attachments),
      MAX_CASE_REQUEST_BODY_BYTES,
      attachments,
    );
    expect(result).toEqual({
      ok: false,
      reason: "attachment_too_large",
      fileName: "big.zip",
    });
    if (!result.ok) {
      expect(
        payloadSizeErrorMessage(result, "case description", MAX_CASE_REQUEST_BODY_BYTES),
      ).toContain('"big.zip" exceeds the 10.00 MB per-file limit');
    }
  });

  it("blames attachments when they dominate an oversize body", () => {
    const attachments = [
      { name: "a.bin", file: b64(9 * MB) },
      { name: "b.bin", file: b64(9 * MB) },
    ];
    const result = checkPayloadSize(
      body("small", attachments),
      MAX_CASE_REQUEST_BODY_BYTES,
      attachments,
    );
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe("attachments_dominant");
      const msg = payloadSizeErrorMessage(result, "case description", MAX_CASE_REQUEST_BODY_BYTES);
      expect(msg).toContain("attachments are too large");
      expect(msg).toContain("10.00 MB");
      expect(msg).not.toContain("inline images");
    }
  });

  it("blames the description when it dominates an oversize body", () => {
    const attachments = [{ name: "a.bin", file: b64(MB) }];
    const result = checkPayloadSize(
      body("x".repeat(MAX_CASE_REQUEST_BODY_BYTES), attachments),
      MAX_CASE_REQUEST_BODY_BYTES,
      attachments,
    );
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toBe("body_too_large");
      expect(
        payloadSizeErrorMessage(result, "case description", MAX_CASE_REQUEST_BODY_BYTES),
      ).toContain("The case description exceeds the");
    }
  });

  it("uses the supplied cap for bodies without attachments", () => {
    expect(checkPayloadSize("x".repeat(10), 5).ok).toBe(false);
    expect(checkPayloadSize("x".repeat(5), 5).ok).toBe(true);
  });
});
