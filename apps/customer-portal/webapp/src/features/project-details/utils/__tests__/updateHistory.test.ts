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

import { createTheme } from "@wso2/oxygen-ui";
import { describe, expect, it } from "vitest";
import {
  getUpdateHistoryEntryBackground,
  getUpdateHistoryErrorMessage,
  getUpdateHistoryOutlineColor,
} from "@features/project-details/utils/updateHistory";

describe("getUpdateHistoryErrorMessage", () => {
  it("returns Error message when thrown value is Error", () => {
    expect(getUpdateHistoryErrorMessage(new Error("save failed"))).toBe("save failed");
  });

  it("returns default message for non-Error values", () => {
    expect(getUpdateHistoryErrorMessage("x")).toBe("Failed to save update history.");
  });
});

describe("update history theme helpers", () => {
  const lightTheme = createTheme({ palette: { mode: "light" } });
  const darkTheme = createTheme({ palette: { mode: "dark" } });

  it("returns background colors for light and dark mode", () => {
    expect(getUpdateHistoryEntryBackground(lightTheme)).toBeTruthy();
    expect(getUpdateHistoryEntryBackground(darkTheme)).toBeTruthy();
  });

  it("returns outline colors for light and dark mode", () => {
    expect(getUpdateHistoryOutlineColor(lightTheme)).toBeTruthy();
    expect(getUpdateHistoryOutlineColor(darkTheme)).toBeTruthy();
  });
});
