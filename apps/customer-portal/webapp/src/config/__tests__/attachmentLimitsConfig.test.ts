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
  entityAttachmentMaxBytes,
  formatLimit,
  formatLimitMb,
  inlineImageMaxBytes,
} from "@config/attachmentLimitsConfig";

const MIB = 1024 * 1024;
const setConfig = (config: unknown) => {
  (window as unknown as { config?: unknown }).config = config;
};

describe("attachment size limits", () => {
  afterEach(() => setConfig(undefined));

  it("default to entity-service's own defaults", () => {
    expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
    expect(inlineImageMaxBytes()).toBe(1 * MIB);
    setConfig({});
    expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
    expect(inlineImageMaxBytes()).toBe(1 * MIB);
  });

  it("read a number or a numeric string, at call time", () => {
    setConfig({ CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25, CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: "5" });
    expect(entityAttachmentMaxBytes()).toBe(25 * MIB);
    expect(inlineImageMaxBytes()).toBe(5 * MIB);
    setConfig({ CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: " 2.5 " });
    expect(entityAttachmentMaxBytes()).toBe(2.5 * MIB);
  });

  it.each([0, -1, "0", "-3", "abc", "", "  ", null, true, NaN, Infinity])(
    "fall back to the default for %j",
    (bad) => {
      setConfig({ CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: bad, CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: bad });
      expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
      expect(inlineImageMaxBytes()).toBe(1 * MIB);
    },
  );

  it("formats a limit as MB for messages", () => {
    expect(formatLimitMb(25 * MIB)).toBe("25");
    expect(formatLimitMb(2.5 * MIB)).toBe("2.5");
  });

  it("formats a limit as GB from 1024 MB, else MB", () => {
    expect(formatLimit(10 * MIB)).toBe("10 MB");
    expect(formatLimit(1023 * MIB)).toBe("1023 MB");
    expect(formatLimit(1024 * MIB)).toBe("1 GB");
    expect(formatLimit(10 * 1024 * MIB)).toBe("10 GB");
    expect(formatLimit(1.5 * 1024 * MIB)).toBe("1.5 GB");
  });
});
