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
import { isEntityAttachmentsEnabled } from "@config/entityAttachmentsConfig";

afterEach(() => {
  window.config = {} as typeof window.config;
});

describe("isEntityAttachmentsEnabled", () => {
  it("is off when the key is absent", () => {
    window.config = {} as typeof window.config;
    expect(isEntityAttachmentsEnabled()).toBe(false);
  });

  it("is on only for true or the string \"true\"", () => {
    for (const raw of [true, "true", " TRUE "]) {
      window.config = { CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED: raw } as typeof window.config;
      expect(isEntityAttachmentsEnabled()).toBe(true);
    }
  });

  it("is off for anything else", () => {
    for (const raw of [false, "false", "1", "yes", "", 1]) {
      window.config = { CSM_PORTAL_ENTITY_ATTACHMENTS_ENABLED: raw } as unknown as typeof window.config;
      expect(isEntityAttachmentsEnabled()).toBe(false);
    }
  });
});
