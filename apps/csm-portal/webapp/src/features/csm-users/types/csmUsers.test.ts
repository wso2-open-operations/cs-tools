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
import { normalizeUser, type SnUser, type User } from "./csmUsers";

// Shape entity-service sends for the postgres data source: firstName/lastName,
// createdOn/updatedOn (not createdAt/updatedAt), and roles from user_role.
const postgresUser: User = {
  id: "u1",
  userName: "jane@example.com",
  firstName: "Jane",
  lastName: "Doe",
  email: "jane@example.com",
  userType: "internal",
  roles: ["admin", "internal"],
  createdOn: "2026-09-16T14:24:00+05:30",
  updatedOn: "2026-09-18T11:38:33+05:30",
};

describe("normalizeUser (postgres source)", () => {
  it("keeps the name and shows the roles a postgres user now carries", () => {
    // Regression: `roles` used to be a ServiceNow-only field, so its presence
    // switched the user to the ServiceNow branch and blanked the name.
    const n = normalizeUser(postgresUser);
    expect(n.name).toBe("Jane Doe");
    expect(n.roles).toEqual(["admin", "internal"]);
  });

  it("reads the createdOn/updatedOn entity-service sends", () => {
    const n = normalizeUser(postgresUser);
    expect(n.createdOn).toBe("2026-09-16T14:24:00+05:30");
    expect(n.updatedOn).toBe("2026-09-18T11:38:33+05:30");
  });

  it("falls back to createdAt/updatedAt", () => {
    const n = normalizeUser({ ...postgresUser, createdOn: undefined, updatedOn: undefined, createdAt: "c", updatedAt: "u" });
    expect(n.createdOn).toBe("c");
    expect(n.updatedOn).toBe("u");
  });

  it("does not invent an active flag or lock state", () => {
    const n = normalizeUser(postgresUser);
    expect(n.active).toBeUndefined();
    expect(n.lockedOut).toBeUndefined();
  });

  it("still works for a user with no roles field", () => {
    const { roles: _roles, ...rest } = postgresUser;
    const n = normalizeUser(rest as User);
    expect(n.name).toBe("Jane Doe");
    expect(n.roles).toBeUndefined();
    // Without roles this used to read createdAt/updatedAt, which entity-service
    // never sends, so a postgres user's dates came out undefined.
    expect(n.createdOn).toBe("2026-09-16T14:24:00+05:30");
    expect(n.updatedOn).toBe("2026-09-18T11:38:33+05:30");
  });
});

describe("normalizeUser (ServiceNow source)", () => {
  const snUser: SnUser = {
    id: "s1",
    userName: "sn.user",
    name: "  Sam Nowak ",
    email: "sam@example.com",
    active: true,
    lockedOut: false,
    createdOn: "2026-01-01",
    updatedOn: "2026-01-02",
    roles: ["agent"],
  };

  it("is unchanged: trimmed name, active flag and roles", () => {
    const n = normalizeUser(snUser);
    expect(n.name).toBe("Sam Nowak");
    expect(n.active).toBe(true);
    expect(n.roles).toEqual(["agent"]);
  });
});
