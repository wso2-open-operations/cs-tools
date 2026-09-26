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
import { normalizeUserDetail, type SnUserDetail } from "./csmUsers";

// What entity-service returns for GET /users/{id} on the postgres data source:
// no lockedOut, timeZone or notificationsEnabled, because that schema has no
// column for them.
const postgresProfile = {
  id: "c1",
  userName: "cy@customer.example",
  name: "Cy Customer",
  email: "cy@customer.example",
  userType: "customer",
  active: true,
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-02-01T00:00:00Z",
  roles: ["customer"],
  groups: [{ id: "t1", name: "CAB Approval" }],
  projectAccess: [
    {
      projectId: "p1",
      projectName: "Acme",
      projectKey: "ACME",
      contactEmail: "cy@customer.example",
      contactRecordPresent: true,
      contactRecordEmail: "cy@customer.example",
      registrationState: "REGISTERED",
      roles: ["PORTAL_USER"],
      grantsCaseAccess: true,
    },
  ],
} as unknown as SnUserDetail;

describe("normalizeUserDetail (postgres profile)", () => {
  it("keeps the name, roles, groups and project access", () => {
    const n = normalizeUserDetail(postgresProfile);
    expect(n.name).toBe("Cy Customer");
    expect(n.userType).toBe("customer");
    expect(n.roles).toEqual(["customer"]);
    expect(n.groups).toEqual([{ id: "t1", name: "CAB Approval" }]);
    expect(n.projectAccess?.[0]).toMatchObject({ projectKey: "ACME", grantsCaseAccess: true, roles: ["PORTAL_USER"] });
  });

  it("does not report a lock state the data source cannot know", () => {
    // The page shows a "Locked out" chip whenever this is defined, so it must stay undefined.
    const n = normalizeUserDetail(postgresProfile);
    expect(n.lockedOut).toBeUndefined();
    expect(n.timezone).toBeNull();
    expect(n.active).toBe(true);
  });
});
