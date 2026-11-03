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


import { afterEach, describe, expect, it } from "vitest";
import {
  LEGACY_ATTACHMENT_MAX_BYTES,
  entityAttachmentMaxBytes,
  formatMegabytes,
  formatSizeLimit,
  getMaxAttachmentSizeBytes,
  inlineImageMaxBytes,
} from "@config/attachmentLimitsConfig";

const MB = 1024 * 1024;

function setConfig(config: Record<string, unknown>): void {
  window.config = config as unknown as typeof window.config;
}

afterEach(() => {
  window.config = {} as typeof window.config;
});

describe("entityAttachmentMaxBytes", () => {
  it("defaults to 100 MB", () => {
    setConfig({});
    expect(entityAttachmentMaxBytes()).toBe(100 * MB);
  });

  it("reads a number or a numeric string of megabytes", () => {
    setConfig({ CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25 });
    expect(entityAttachmentMaxBytes()).toBe(25 * MB);
    setConfig({ CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: " 25 " });
    expect(entityAttachmentMaxBytes()).toBe(25 * MB);
    setConfig({ CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: "0.5" });
    expect(entityAttachmentMaxBytes()).toBe(0.5 * MB);
  });

  it("falls back to the default for anything unusable", () => {
    for (const raw of ["abc", "", "  ", 0, -5, "-5", "0", NaN, Infinity, null, true, {}]) {
      setConfig({ CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: raw });
      expect(entityAttachmentMaxBytes()).toBe(100 * MB);
    }
  });
});

describe("inlineImageMaxBytes", () => {
  it("defaults to 10 MB and reads its own key", () => {
    setConfig({});
    expect(inlineImageMaxBytes()).toBe(10 * MB);
    setConfig({ CSM_PORTAL_INLINE_IMAGE_MAX_MB: "5", CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25 });
    expect(inlineImageMaxBytes()).toBe(5 * MB);
  });

  it("falls back to the default for anything unusable", () => {
    for (const raw of ["x", 0, -1]) {
      setConfig({ CSM_PORTAL_INLINE_IMAGE_MAX_MB: raw });
      expect(inlineImageMaxBytes()).toBe(10 * MB);
    }
  });
});

describe("getMaxAttachmentSizeBytes", () => {
  it("uses the configured limit through the entity flow", () => {
    setConfig({ CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25 });
    expect(getMaxAttachmentSizeBytes()).toBe(25 * MB);
  });

  it("keeps the older limit whatever the key says when entity attachments are off", () => {
    setConfig({ CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED: false, CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25 });
    expect(getMaxAttachmentSizeBytes()).toBe(LEGACY_ATTACHMENT_MAX_BYTES);
    expect(LEGACY_ATTACHMENT_MAX_BYTES).toBe(10 * MB);
  });
});

describe("formatMegabytes", () => {
  it("prints whole and fractional megabytes", () => {
    expect(formatMegabytes(10 * MB)).toBe("10 MB");
    expect(formatMegabytes(2.5 * MB)).toBe("2.5 MB");
  });
});

describe("formatSizeLimit", () => {
  it("uses MB below 1024 MB and GB from there", () => {
    expect(formatSizeLimit(10 * MB)).toBe("10 MB");
    expect(formatSizeLimit(1023 * MB)).toBe("1023 MB");
    expect(formatSizeLimit(1024 * MB)).toBe("1 GB");
    expect(formatSizeLimit(10 * 1024 * MB)).toBe("10 GB");
    expect(formatSizeLimit(1536 * MB)).toBe("1.5 GB");
  });

  it("formats the configured limit of 10240 MB as 10 GB", () => {
    setConfig({ CSM_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 10240 });
    expect(formatSizeLimit(entityAttachmentMaxBytes())).toBe("10 GB");
  });
});
