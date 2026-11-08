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
import { canDeleteAttachment } from "@features/csm-cases/utils/canDeleteAttachment";

const base = { caseClosed: false, isAdmin: false, myEmail: "me@wso2.com", uploaderEmail: "me@wso2.com" };

describe("canDeleteAttachment", () => {
  it("lets the uploader delete, ignoring case and spaces", () => {
    expect(canDeleteAttachment(base)).toBe(true);
    expect(canDeleteAttachment({ ...base, myEmail: " ME@wso2.com " })).toBe(true);
  });

  it("does not let someone else delete", () => {
    expect(canDeleteAttachment({ ...base, uploaderEmail: "other@wso2.com" })).toBe(false);
    expect(canDeleteAttachment({ ...base, uploaderEmail: undefined })).toBe(false);
    expect(canDeleteAttachment({ ...base, myEmail: undefined, uploaderEmail: undefined })).toBe(false);
  });

  it("lets an admin delete someone else's", () => {
    expect(canDeleteAttachment({ ...base, isAdmin: true, uploaderEmail: "other@wso2.com" })).toBe(true);
    expect(canDeleteAttachment({ ...base, isAdmin: true, myEmail: undefined, uploaderEmail: undefined })).toBe(true);
  });

  it("lets nobody delete once the case is closed, an admin included", () => {
    expect(canDeleteAttachment({ ...base, caseClosed: true })).toBe(false);
    expect(canDeleteAttachment({ ...base, caseClosed: true, isAdmin: true })).toBe(false);
  });
});
