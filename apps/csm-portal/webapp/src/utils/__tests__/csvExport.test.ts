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

import { describe, expect, it, vi } from "vitest";
import { csvField, fetchAllPages, rowsToCsvText } from "@utils/csvExport";

describe("csvField", () => {
  it("leaves a plain value untouched", () => {
    expect(csvField("hello")).toBe("hello");
  });

  it("quotes a value containing a comma", () => {
    expect(csvField("a, b")).toBe('"a, b"');
  });

  it("doubles internal quotes", () => {
    expect(csvField('say "hi"')).toBe('"say ""hi"""');
  });

  it("quotes a value with a bare carriage return", () => {
    expect(csvField("a\rb")).toBe('"a\rb"');
  });

  it.each(["=SUM(A1:A2)", "+1", "-1", "@cmd", "\tx", "\rx"])(
    "prefixes a quote when the value starts with %j so it is not read as a formula",
    (value) => {
      expect(csvField(value).replace(/^"/, "").startsWith("'")).toBe(true);
    },
  );

  it("neutralises a formula and still quotes it when it contains a comma", () => {
    expect(csvField("=A1,B1")).toBe('"\'=A1,B1"');
  });

  it("does not touch a value with a formula character that is not leading", () => {
    expect(csvField("a=b")).toBe("a=b");
  });
});

describe("rowsToCsvText", () => {
  it("joins header and rows with CRLF", () => {
    const text = rowsToCsvText(["A", "B"], [["1", "2"], ["3", "4"]]);
    expect(text).toBe("A,B\r\n1,2\r\n3,4");
  });
});

describe("fetchAllPages", () => {
  function makeFetcher(items: number[], pageErrorAt?: number) {
    return vi.fn(async (offset: number, limit: number) => {
      if (pageErrorAt !== undefined && offset === pageErrorAt) {
        throw new Error("network blip");
      }
      return {
        items: items.slice(offset, offset + limit),
        total: items.length,
      };
    });
  }

  it("pages until the result set is exhausted", async () => {
    const items = Array.from({ length: 125 }, (_, i) => i);
    const fetchPage = makeFetcher(items);
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });

    expect(result.items).toEqual(items);
    expect(result.truncated).toBe(false);
    expect(result.total).toBe(125);
    // 125 rows at 50/page -> 3 requests (50 + 50 + 25).
    expect(fetchPage).toHaveBeenCalledTimes(3);
  });

  it("stops after a single page when total fits in it", async () => {
    const items = [1, 2, 3];
    const fetchPage = makeFetcher(items);
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });
    expect(result.items).toEqual(items);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });

  it("returns an empty result without looping when there's nothing to fetch", async () => {
    const fetchPage = makeFetcher([]);
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });
    expect(result.items).toEqual([]);
    expect(result.total).toBe(0);
    expect(result.truncated).toBe(false);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });

  it("truncates at the row cap instead of looping forever, and says so", async () => {
    const items = Array.from({ length: 500 }, (_, i) => i);
    const fetchPage = makeFetcher(items);
    const result = await fetchAllPages(fetchPage, { pageSize: 50, maxRows: 120 });

    // Cap of 120 rows at 50/page -> stops after the 3rd page (150 fetched,
    // still >= cap), never fetching all 500.
    expect(result.truncated).toBe(true);
    expect(result.items.length).toBeLessThan(500);
    expect(result.items.length).toBeGreaterThanOrEqual(120);
    expect(result.total).toBe(500);
  });

  it("reports progress after every page via onProgress", async () => {
    const items = Array.from({ length: 30 }, (_, i) => i);
    const fetchPage = makeFetcher(items);
    const onProgress = vi.fn();
    await fetchAllPages(fetchPage, { pageSize: 10, onProgress });

    expect(onProgress).toHaveBeenNthCalledWith(1, 10, 30);
    expect(onProgress).toHaveBeenNthCalledWith(2, 20, 30);
    expect(onProgress).toHaveBeenNthCalledWith(3, 30, 30);
  });

  it("propagates a mid-export page failure instead of returning a partial result", async () => {
    const items = Array.from({ length: 100 }, (_, i) => i);
    const fetchPage = makeFetcher(items, 50);
    await expect(fetchAllPages(fetchPage, { pageSize: 50 })).rejects.toThrow(
      "network blip",
    );
  });

  it("never loops forever if a page comes back empty despite a nonzero total", async () => {
    const fetchPage = vi.fn(async () => ({ items: [] as number[], total: 999 }));
    const result = await fetchAllPages(fetchPage, { pageSize: 50 });
    expect(result.items).toEqual([]);
    expect(fetchPage).toHaveBeenCalledTimes(1);
  });
});
