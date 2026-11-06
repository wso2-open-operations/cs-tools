/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it, vi } from "vitest";
import type { RotaAccess, RotaAdminGrant } from "../types";

vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({}) }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

const { isRotaAdminOf, mayEditWeekOf } = await import("./useRotaAccess");

function grant(over: Partial<RotaAdminGrant>): RotaAdminGrant {
  return {
    userId: "u1",
    name: "Jane Doe",
    email: "jane.doe@example.com",
    grantedBy: "admin@example.com",
    grantedOn: "2026-10-01T00:00:00Z",
    active: true,
    family: "CRE",
    ...over,
  };
}

function access(...rotaAdmins: RotaAdminGrant[]): RotaAccess {
  return { canManage: { rotaAdmins: false, markerFamilies: ["CRE"] }, rotaAdmins, markers: [] };
}

describe("isRotaAdminOf", () => {
  it("is true for the reader's own active grant on that family, whatever the email's case", () => {
    expect(isRotaAdminOf(access(grant({})), " Jane.Doe@Example.com ", "CRE")).toBe(true);
  });

  it("is false for another family, an inactive grant, someone else's grant, or no reader", () => {
    expect(isRotaAdminOf(access(grant({})), "jane.doe@example.com", "SRE")).toBe(false);
    expect(isRotaAdminOf(access(grant({ active: false })), "jane.doe@example.com", "CRE")).toBe(false);
    expect(isRotaAdminOf(access(grant({ email: "john.roe@example.com" })), "jane.doe@example.com", "CRE")).toBe(false);
    expect(isRotaAdminOf(access(grant({})), "", "CRE")).toBe(false);
    expect(isRotaAdminOf(undefined, "jane.doe@example.com", "CRE")).toBe(false);
  });

  it("is false for a CSM admin who holds no rota admin grant", () => {
    const csmAdmin: RotaAccess = {
      canManage: { rotaAdmins: true, markerFamilies: ["CRE", "SRE", "SME"] },
      rotaAdmins: [grant({ email: "someone.else@example.com" })],
      markers: [],
    };
    expect(isRotaAdminOf(csmAdmin, "jane.doe@example.com", "CRE")).toBe(false);
  });
});

describe("mayEditWeekOf", () => {
  it("lets a rota admin edit their own family's week and no other", () => {
    const a = access(grant({}));
    expect(mayEditWeekOf(a, "jane.doe@example.com", "CRE")).toBe(true);
    expect(mayEditWeekOf(a, "jane.doe@example.com", "SRE")).toBe(false);
  });

  it("lets a CSM or portal admin edit every family's week", () => {
    const admin: RotaAccess = { canManage: { rotaAdmins: true, markerFamilies: ["CRE", "SRE", "SME"] }, rotaAdmins: [], markers: [] };
    for (const f of ["CRE", "SRE", "SME"] as const) expect(mayEditWeekOf(admin, "admin@example.com", f)).toBe(true);
  });

  it("does not let a reader who may only name markers edit the week", () => {
    const leadOnly: RotaAccess = { canManage: { rotaAdmins: false, markerFamilies: [] }, rotaAdmins: [], markers: [] };
    expect(mayEditWeekOf(leadOnly, "lead@example.com", "CRE")).toBe(false);
    expect(mayEditWeekOf(undefined, "lead@example.com", "CRE")).toBe(false);
  });
});
