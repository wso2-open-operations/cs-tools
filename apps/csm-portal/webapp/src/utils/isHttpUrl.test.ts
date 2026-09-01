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
import { isHttpUrl } from "./isHttpUrl";

describe("isHttpUrl", () => {
  it("accepts absolute http(s) URLs", () => {
    expect(isHttpUrl("https://github.com/wso2-enterprise/choreo/issues/1")).toBe(true);
    expect(isHttpUrl("http://example.com")).toBe(true);
  });

  it("rejects javascript: and other dangerous schemes", () => {
    expect(isHttpUrl("javascript:alert(1)")).toBe(false);
    expect(isHttpUrl("data:text/html,<script>alert(1)</script>")).toBe(false);
    expect(isHttpUrl("vbscript:msgbox(1)")).toBe(false);
  });

  it("rejects relative and malformed values", () => {
    expect(isHttpUrl("/cases/123")).toBe(false);
    expect(isHttpUrl("")).toBe(false);
    expect(isHttpUrl("not a url")).toBe(false);
  });
});
