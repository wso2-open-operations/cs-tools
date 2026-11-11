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
  DEFAULT_ENTITY_ATTACHMENT_MAX_MB,
  DEFAULT_INLINE_IMAGES_TOTAL_MAX_MB,
  formatLimit,
  formatLimitMb,
} from "@config/attachmentLimitsConfig";

const MIB = 1024 * 1024;
const setConfig = (config: unknown) => {
  (window as unknown as { config?: unknown }).config = config;
};

describe("attachment size limits", () => {
  it("default to entity-service's own defaults, used until the server answers", () => {
    expect(DEFAULT_ENTITY_ATTACHMENT_MAX_MB).toBe(10);
    expect(DEFAULT_INLINE_IMAGES_TOTAL_MAX_MB).toBe(1);
  });

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
