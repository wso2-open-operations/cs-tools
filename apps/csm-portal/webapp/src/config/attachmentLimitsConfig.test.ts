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
  attachmentLimitMessage,
  downloadToDiskEnabled,
  formatMegabytes,
  formatSizeLimit,
} from "@config/attachmentLimitsConfig";

const MB = 1024 * 1024;

function setConfig(config: Record<string, unknown>): void {
  window.config = config as unknown as typeof window.config;
}

afterEach(() => {
  window.config = {} as typeof window.config;
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
});

describe("downloadParallelism", () => {
  it("defaults to 4 when unset or not a number", () => {
    expect(downloadParallelism()).toBe(DEFAULT_DOWNLOAD_PARALLELISM);
    for (const bad of ["", "  ", "abc", null, undefined, {}, Number.NaN]) {
      setConfig({ CSM_PORTAL_DOWNLOAD_PARALLELISM: bad });
      expect(downloadParallelism()).toBe(4);
    }
  });

  it("reads a number or numeric string, keeping a whole number between 1 and 8", () => {
    const cases: [unknown, number][] = [[1, 1], ["3", 3], [2.9, 2], [8, 8], [20, 8], [0, 1], [-5, 1]];
    for (const [raw, expected] of cases) {
      setConfig({ CSM_PORTAL_DOWNLOAD_PARALLELISM: raw });
      expect(downloadParallelism()).toBe(expected);
    }
  });
});

describe("downloadToDiskEnabled", () => {
  afterEach(() => setConfig({}));

  it("is on when unset or not false", () => {
    expect(downloadToDiskEnabled()).toBe(true);
    for (const on of [true, "true", " TRUE ", "yes", "", null, undefined, 1]) {
      setConfig({ CSM_PORTAL_DOWNLOAD_TO_DISK: on });
      expect(downloadToDiskEnabled()).toBe(true);
    }
  });

  it("is off for false or the string false", () => {
    for (const off of [false, "false", " False "]) {
      setConfig({ CSM_PORTAL_DOWNLOAD_TO_DISK: off });
      expect(downloadToDiskEnabled()).toBe(false);
    }
  });
});

describe("attachmentLimitMessage", () => {
  it("says only one file is allowed when the limit is one", () => {
    expect(attachmentLimitMessage(1)).toBe("Only one file allowed.");
  });

  it("uses the plural above one", () => {
    expect(attachmentLimitMessage(3)).toBe("Only 3 files allowed.");
  });
});
