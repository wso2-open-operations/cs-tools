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
  DEFAULT_DOWNLOAD_PARALLELISM,
  downloadParallelism,
  downloadToDiskEnabled,
  entityAttachmentMaxBytes,
  formatLimit,
  formatLimitMb,
  inlineImageMaxBytes,
  inlineImagesTotalMaxBytes,
} from "@config/attachmentLimitsConfig";

const MIB = 1024 * 1024;
const setConfig = (config: unknown) => {
  (window as unknown as { config?: unknown }).config = config;
};

describe("attachment size limits", () => {
  afterEach(() => setConfig(undefined));

  it("default to entity-service's own defaults", () => {
    expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
    expect(inlineImagesTotalMaxBytes()).toBe(1 * MIB);
    expect(inlineImageMaxBytes()).toBe(1 * MIB);
    setConfig({});
    expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
    expect(inlineImagesTotalMaxBytes()).toBe(1 * MIB);
    expect(inlineImageMaxBytes()).toBe(1 * MIB);
  });

  it("read a number or a numeric string, at call time", () => {
    setConfig({ CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 25, CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: "5" });
    expect(entityAttachmentMaxBytes()).toBe(25 * MIB);
    expect(inlineImageMaxBytes()).toBe(5 * MIB);
    setConfig({ CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: " 2.5 " });
    expect(entityAttachmentMaxBytes()).toBe(2.5 * MIB);
  });

  it("prefers the total key over the older per-image key", () => {
    setConfig({
      CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: 5,
      CUSTOMER_PORTAL_INLINE_IMAGES_TOTAL_MAX_MB: "20",
    });
    expect(inlineImagesTotalMaxBytes()).toBe(20 * MIB);
    setConfig({ CUSTOMER_PORTAL_INLINE_IMAGES_TOTAL_MAX_MB: "bad", CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: 5 });
    expect(inlineImagesTotalMaxBytes()).toBe(5 * MIB);
  });

  it("gives the default, not the older key's value, for a new key over the ceiling", () => {
    setConfig({
      CUSTOMER_PORTAL_INLINE_IMAGES_TOTAL_MAX_MB: 26,
      CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: 5,
    });
    expect(inlineImagesTotalMaxBytes()).toBe(1 * MIB);
    expect(inlineImageMaxBytes()).toBe(1 * MIB);
  });

  it("ignores a value above the 100 MB / 25 MB ceilings and accepts the ceilings", () => {
    setConfig({
      CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 101,
      CUSTOMER_PORTAL_INLINE_IMAGES_TOTAL_MAX_MB: 26,
    });
    expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
    expect(inlineImagesTotalMaxBytes()).toBe(1 * MIB);
    setConfig({
      CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: 100,
      CUSTOMER_PORTAL_INLINE_IMAGES_TOTAL_MAX_MB: 25,
    });
    expect(entityAttachmentMaxBytes()).toBe(100 * MIB);
    expect(inlineImagesTotalMaxBytes()).toBe(25 * MIB);
  });

  it.each([0, -1, "0", "-3", "abc", "", "  ", null, true, NaN, Infinity])(
    "fall back to the default for %j",
    (bad) => {
      setConfig({ CUSTOMER_PORTAL_ENTITY_ATTACHMENT_MAX_MB: bad, CUSTOMER_PORTAL_INLINE_IMAGE_MAX_MB: bad });
      expect(entityAttachmentMaxBytes()).toBe(10 * MIB);
      expect(inlineImagesTotalMaxBytes()).toBe(1 * MIB);
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

describe("downloadParallelism", () => {
  afterEach(() => setConfig(undefined));

  it("defaults to 4 when unset or not a number", () => {
    expect(downloadParallelism()).toBe(DEFAULT_DOWNLOAD_PARALLELISM);
    for (const bad of ["", "  ", "abc", null, undefined, {}, Number.NaN]) {
      setConfig({ CUSTOMER_PORTAL_DOWNLOAD_PARALLELISM: bad });
      expect(downloadParallelism()).toBe(4);
    }
  });

  it("reads a number or numeric string, keeping a whole number between 1 and 8", () => {
    const cases: [unknown, number][] = [
      [1, 1],
      ["3", 3],
      [2.9, 2],
      [8, 8],
      [20, 8],
      [0, 1],
      [-5, 1],
    ];
    for (const [raw, expected] of cases) {
      setConfig({ CUSTOMER_PORTAL_DOWNLOAD_PARALLELISM: raw });
      expect(downloadParallelism()).toBe(expected);
    }
  });
});

describe("downloadToDiskEnabled", () => {
  afterEach(() => setConfig(undefined));

  it("is on when unset or not false", () => {
    expect(downloadToDiskEnabled()).toBe(true);
    for (const on of [true, "true", " TRUE ", "yes", "", null, undefined, 1]) {
      setConfig({ CUSTOMER_PORTAL_DOWNLOAD_TO_DISK: on });
      expect(downloadToDiskEnabled()).toBe(true);
    }
  });

  it("is off for false or the string false", () => {
    for (const off of [false, "false", " False "]) {
      setConfig({ CUSTOMER_PORTAL_DOWNLOAD_TO_DISK: off });
      expect(downloadToDiskEnabled()).toBe(false);
    }
  });
});
