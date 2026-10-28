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
import { isEntityAttachmentsEnabled } from "@/config/entityAttachmentsConfig";

describe("isEntityAttachmentsEnabled", () => {
  afterEach(() => {
    (window as unknown as { config?: unknown }).config = undefined;
  });

  it("is on unless the key is exactly false", () => {
    const set = (value: unknown) => {
      (window as unknown as { config?: unknown }).config = {
        CUSTOMER_PORTAL_ENTITY_ATTACHMENTS_ENABLED: value,
      };
    };
    expect(isEntityAttachmentsEnabled()).toBe(true);
    (window as unknown as { config?: unknown }).config = {};
    expect(isEntityAttachmentsEnabled()).toBe(true);
    set("false");
    expect(isEntityAttachmentsEnabled()).toBe(true);
    set(true);
    expect(isEntityAttachmentsEnabled()).toBe(true);
    set(false);
    expect(isEntityAttachmentsEnabled()).toBe(false);
  });
});
