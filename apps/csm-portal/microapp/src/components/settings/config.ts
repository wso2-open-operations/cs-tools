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

import type { AdminUserRole } from "@src/types";

// Mirrors the webapp's ALL_ROLES/INTERNAL_USER_ROLES (apps/csm-portal/webapp/src/features/csm-users/pages/CsmUsersPage.tsx).
export const INTERNAL_USER_ROLES: AdminUserRole[] = ["internal", "agent", "admin"];

export const ALL_USER_ROLES: AdminUserRole[] = [
  ...INTERNAL_USER_ROLES,
  "commenter",
  "external",
  "customer",
  "customer_admin",
  "partner",
  "partner_admin",
];

export const USER_ROLE_LABELS: Record<AdminUserRole, string> = {
  internal: "Internal",
  agent: "Agent",
  admin: "Admin",
  commenter: "Commenter",
  external: "External",
  customer: "Customer",
  customer_admin: "Customer admin",
  partner: "Partner",
  partner_admin: "Partner admin",
};

export type AdminTabId = "users" | "roles" | "groups" | "permissions";

export interface AdminTabConfig {
  id: AdminTabId;
  label: string;
  wip?: boolean;
  description?: string;
  blockedOn?: string;
}

// Mirrors the webapp's ADMIN_TABS (apps/csm-portal/webapp/src/features/csm-admin/pages/CsmAdminLayout.tsx)
// and the CsmComingSoonPage copy each WIP tab renders (App.tsx's /admin/* routes).
export const ADMIN_TABS: AdminTabConfig[] = [
  { id: "users", label: "Users" },
  {
    id: "roles",
    label: "Roles",
    wip: true,
    description: "Role-based access control: define roles and their permission sets.",
    blockedOn: "csm-portal/backend roles endpoints",
  },
  {
    id: "groups",
    label: "Groups",
    wip: true,
    description: "User groups for bulk role assignment and access control.",
    blockedOn: "csm-portal/backend groups endpoints",
  },
  {
    id: "permissions",
    label: "Permissions",
    wip: true,
    description: "Fine-grained permission catalog and assignment view.",
    blockedOn: "csm-portal/backend permissions endpoints",
  },
];
