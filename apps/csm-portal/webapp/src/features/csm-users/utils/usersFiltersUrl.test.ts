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
  DEFAULT_USERS_FILTERS,
  readUsersFiltersFromUrl,
  writeUsersFiltersToUrl,
  type UsersFilters,
} from "@features/csm-users/utils/usersFiltersUrl";

describe("usersFiltersUrl", () => {
  it("round-trips a fully populated filter set through the URL", () => {
    const filters: UsersFilters = {
      search: "jane",
      groupIds: ["11111111-1111-1111-1111-111111111111"],
      teamIds: ["alpha"],
      active: "active",
    };
    const params = writeUsersFiltersToUrl(filters);
    expect(readUsersFiltersFromUrl(params)).toEqual(filters);
  });

  it("omits default values so an unfiltered list has a clean URL", () => {
    const params = writeUsersFiltersToUrl(DEFAULT_USERS_FILTERS);
    expect(params.toString()).toBe("");
  });

  it("reads an empty URL back to the defaults", () => {
    expect(readUsersFiltersFromUrl(new URLSearchParams())).toEqual(
      DEFAULT_USERS_FILTERS,
    );
  });

  it("ignores an invalid active value rather than throwing", () => {
    const params = new URLSearchParams("active=bogus");
    expect(readUsersFiltersFromUrl(params).active).toBe("all");
  });
});
