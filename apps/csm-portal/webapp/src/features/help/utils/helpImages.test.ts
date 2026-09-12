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
import { resolveHelpImages } from "@features/help/utils/helpImages";

describe("resolveHelpImages", () => {
  // No topic currently embeds an image (the registry is empty — see
  // helpImages.ts), so there's no "known" name to assert resolves today; this
  // covers the unresolved/no-op paths that hold regardless.

  it("leaves an unknown image reference untouched", () => {
    const html = '<img src="images/does-not-exist.png" alt="Missing">';
    expect(resolveHelpImages(html)).toBe(html);
  });

  it("leaves HTML with no image references untouched", () => {
    const html = "<p>No images here.</p>";
    expect(resolveHelpImages(html)).toBe(html);
  });
});
