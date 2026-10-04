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

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { openExternalUrl } from "./openExternalUrl";

describe("openExternalUrl", () => {
  const openSpy = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("open", openSpy);
  });
  afterEach(() => {
    openSpy.mockReset();
    vi.unstubAllGlobals();
  });

  it("opens an https URL in a new tab without opener or referrer", () => {
    expect(openExternalUrl("https://drive.example/folder/1")).toBe(true);
    expect(openSpy).toHaveBeenCalledWith(
      "https://drive.example/folder/1",
      "_blank",
      "noopener,noreferrer",
    );
  });

  it.each([
    "javascript:alert(1)",
    "data:text/html,x",
    "http://insecure.example/",
    "/relative/path",
    "not a url",
    "",
  ])("rejects %j", (url) => {
    expect(openExternalUrl(url)).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
  });

  it("rejects null and undefined", () => {
    expect(openExternalUrl(null)).toBe(false);
    expect(openExternalUrl(undefined)).toBe(false);
    expect(openSpy).not.toHaveBeenCalled();
  });
});
