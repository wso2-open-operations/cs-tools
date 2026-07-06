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
import {
  getAiAssistantPatchSuccessMessage,
  resolveRegistryTokenSubTabId,
  resolveSettingsPageTabId,
} from "@features/settings/utils/settingsPage";
import {
  AiAssistantPatchSuccessKind,
  RegistryTokenSubTabId,
  SettingsPageTabId,
} from "@features/settings/types/settings";

describe("settingsPage utils", () => {
  it("falls back to users tab for unknown settings tab id", () => {
    expect(resolveSettingsPageTabId("unknown")).toBe(SettingsPageTabId.USERS);
  });

  it("prevents service tab when admin tab is unavailable", () => {
    expect(resolveRegistryTokenSubTabId(RegistryTokenSubTabId.SERVICE, false)).toBe(
      RegistryTokenSubTabId.USER,
    );
  });

  it("returns success text for AI settings patch kind", () => {
    expect(getAiAssistantPatchSuccessMessage(AiAssistantPatchSuccessKind.KB)).toContain(
      "Knowledge",
    );
  });
});

