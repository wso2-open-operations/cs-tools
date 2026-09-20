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
import { safeHttpUrl } from "./url";

describe("safeHttpUrl", () => {
  it("passes through a valid https URL unchanged", () => {
    expect(safeHttpUrl("https://github.com/acme/widgets/issues/1")).toBe(
      "https://github.com/acme/widgets/issues/1",
    );
  });

  it("rejects a javascript: URL", () => {
    expect(safeHttpUrl("javascript:alert(1)")).toBe("#");
  });

  it("rejects a plain http URL", () => {
    expect(safeHttpUrl("http://github.com/acme/widgets/issues/1")).toBe("#");
  });

  it("rejects a malformed URL", () => {
    expect(safeHttpUrl("not a url")).toBe("#");
  });

  it("falls back to # for null/undefined/empty", () => {
    expect(safeHttpUrl(null)).toBe("#");
    expect(safeHttpUrl(undefined)).toBe("#");
    expect(safeHttpUrl("")).toBe("#");
  });
});
