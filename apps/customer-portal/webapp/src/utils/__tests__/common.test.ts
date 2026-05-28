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
import { paginatedSelectMenuListProps, stripLightModeInlineStyles } from "@utils/common";
import { PAGINATED_SELECT_MENU_MAX_HEIGHT_PX } from "@constants/common";

describe("common utils", () => {
  it("paginatedSelectMenuListProps sets max height", () => {
    const props = paginatedSelectMenuListProps();
    expect(props.sx.maxHeight).toBe(PAGINATED_SELECT_MENU_MAX_HEIGHT_PX);
    expect(props.onScroll).toBeUndefined();
  });

  it("stripLightModeInlineStyles removes white backgrounds and dark text", () => {
    const html =
      '<p style="background-color: white; color: #000000">Hi</p>';
    const cleaned = stripLightModeInlineStyles(html);
    expect(cleaned).not.toContain("background-color: white");
    expect(cleaned).not.toContain("color: #000000");
  });

});
