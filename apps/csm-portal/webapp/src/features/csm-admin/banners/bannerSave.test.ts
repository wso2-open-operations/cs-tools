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
import { statusOnSchedule } from "@features/csm-admin/banners/bannerSave";

const now = new Date("2026-10-26T12:00:00");

describe("statusOnSchedule", () => {
  it("keeps a banner a draft until the start and unpublishes it after the end", () => {
    expect(statusOnSchedule("published", "2026-11-01T09:00", "", now)).toBe("draft");
    expect(statusOnSchedule("published", "2026-10-01T09:00", "2026-10-20T09:00", now)).toBe("unpublished");
    expect(statusOnSchedule("published", "2026-10-01T09:00", "2026-12-01T09:00", now)).toBe("published");
  });
});
