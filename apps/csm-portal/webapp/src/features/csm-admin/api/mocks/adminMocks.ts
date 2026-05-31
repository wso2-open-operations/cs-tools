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

import type {
  CsmGroup,
  CsmPermission,
  CsmPermissionCategory,
  CsmRole,
  CsmUser,
} from "@features/csm-admin/types/csmAdmin";

const daysAgo = (n: number): string =>
  new Date(Date.now() - n * 24 * 60 * 60_000).toISOString();

// ---------- Permissions (~20 across 8 categories) ----------
const SEED_PERMISSIONS: CsmPermission[] = [
  { id: "perm.cases.view_all", name: "View all cases", description: "Read access to every customer's cases.", category: "cases" },
  { id: "perm.cases.comment", name: "Comment on cases", description: "Reply on the case comment trail.", category: "cases" },
  { id: "perm.cases.edit_state", name: "Change case state", description: "Move cases between open / WIP / awaiting / closed.", category: "cases" },
  { id: "perm.cases.reassign", name: "Reassign cases", description: "Change a case's owner to another engineer.", category: "cases" },
  { id: "perm.cases.create", name: "Create case on behalf of customer", description: "Open a case in a customer's name from inside the portal.", category: "cases" },

  { id: "perm.projects.view", name: "View projects", description: "Read project records and metadata.", category: "projects" },
  { id: "perm.projects.edit_deployments", name: "Manage deployments", description: "Create / update deployments on behalf of a customer.", category: "projects" },

  { id: "perm.accounts.view", name: "View accounts", description: "Read account records.", category: "accounts" },
  { id: "perm.accounts.edit_tier", name: "Change account tier", description: "Promote / demote an account's support tier.", category: "accounts" },

  { id: "perm.engagements.view", name: "View engagements", description: "Read PS engagement records.", category: "engagements" },
  { id: "perm.engagements.manage", name: "Manage engagements", description: "Create, update, and close engagements.", category: "engagements" },

  { id: "perm.updates.view", name: "View update levels", description: "Read product update levels per deployment.", category: "updates" },
  { id: "perm.updates.manage", name: "Manage update levels", description: "Apply update levels to deployments.", category: "updates" },

  { id: "perm.security.view", name: "View vulnerabilities", description: "Read the cross-customer security view.", category: "security" },
  { id: "perm.security.manage", name: "Manage vulnerability response", description: "Track and close vulnerability remediation.", category: "security" },

  { id: "perm.time_cards.view_team", name: "View team time cards", description: "See time-card entries for engineers in your scope.", category: "time_cards" },
  { id: "perm.time_cards.approve", name: "Approve time cards", description: "Approve submitted time-card entries.", category: "time_cards" },

  { id: "perm.admin.manage_users", name: "Manage users", description: "Invite, suspend, and edit user profiles.", category: "admin" },
  { id: "perm.admin.manage_roles", name: "Manage roles", description: "Create, edit, and delete roles.", category: "admin" },
  { id: "perm.admin.manage_groups", name: "Manage groups", description: "Create, edit, and delete groups, including membership.", category: "admin" },
];

// ---------- Roles ----------
// Aligned with the BRD persona model. Real production role ids (such as
// `sn_customerservice.proxy_case_creator`) are kept in `externalId` so future
// platform→SN mapping is traceable.
const SEED_ROLES: CsmRole[] = [
  {
    id: "role.cre_engineer",
    name: "CRE Engineer",
    description: "CRE day-to-day case handler. Full case workflow on owned cases.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.create",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.updates.view",
      "perm.engagements.view",
      "perm.security.view",
    ],
  },
  {
    id: "role.cre_lead",
    name: "CRE Lead",
    description: "Manages a CRE team; handles escalations, assignments, and time-card approval.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.reassign",
      "perm.cases.create",
      "perm.projects.view",
      "perm.projects.edit_deployments",
      "perm.accounts.view",
      "perm.accounts.edit_tier",
      "perm.updates.view",
      "perm.updates.manage",
      "perm.engagements.view",
      "perm.engagements.manage",
      "perm.security.view",
      "perm.security.manage",
      "perm.time_cards.view_team",
      "perm.time_cards.approve",
    ],
  },
  {
    id: "role.sre_engineer",
    name: "SRE Engineer",
    description: "SRE day-to-day case handler. Managed cloud focus.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.create",
      "perm.projects.view",
      "perm.projects.edit_deployments",
      "perm.accounts.view",
      "perm.updates.view",
      "perm.updates.manage",
      "perm.security.view",
    ],
  },
  {
    id: "role.sre_lead",
    name: "SRE Lead",
    description: "Manages an SRE team; handles escalations and time-card approval.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.reassign",
      "perm.cases.create",
      "perm.projects.view",
      "perm.projects.edit_deployments",
      "perm.accounts.view",
      "perm.accounts.edit_tier",
      "perm.updates.view",
      "perm.updates.manage",
      "perm.security.view",
      "perm.security.manage",
      "perm.time_cards.view_team",
      "perm.time_cards.approve",
    ],
  },
  {
    id: "role.fde_engineer",
    name: "FDE Engineer",
    description: "FDE day-to-day task handler (Field Delivery Engineer).",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.create",
      "perm.projects.view",
      "perm.projects.edit_deployments",
      "perm.accounts.view",
      "perm.engagements.view",
      "perm.engagements.manage",
    ],
  },
  {
    id: "role.fde_lead",
    name: "FDE Lead",
    description: "Manages an FDE team; handles assignments and time-card approval.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.reassign",
      "perm.cases.create",
      "perm.projects.view",
      "perm.projects.edit_deployments",
      "perm.accounts.view",
      "perm.engagements.view",
      "perm.engagements.manage",
      "perm.time_cards.view_team",
      "perm.time_cards.approve",
    ],
  },
  {
    id: "role.operational_lead",
    name: "Operational Lead",
    description: "Cross-team operational visibility and management.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.reassign",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.accounts.edit_tier",
      "perm.updates.view",
      "perm.engagements.view",
      "perm.engagements.manage",
      "perm.security.view",
      "perm.security.manage",
      "perm.time_cards.view_team",
      "perm.time_cards.approve",
      "perm.admin.manage_users",
    ],
  },
  {
    id: "role.functional_lead",
    name: "Functional Lead",
    description: "Manages a specific functional area (e.g. security, SRE).",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.cases.edit_state",
      "perm.cases.reassign",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.updates.view",
      "perm.engagements.view",
      "perm.security.view",
      "perm.security.manage",
      "perm.time_cards.view_team",
    ],
  },
  {
    id: "role.leadership",
    name: "Leadership",
    description: "Executive visibility into operational health (read-only across the board).",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.updates.view",
      "perm.engagements.view",
      "perm.security.view",
      "perm.time_cards.view_team",
    ],
  },
  {
    id: "role.pre_sales",
    name: "Pre-Sales Engineer",
    description: "Supports customer evaluations and trials.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.updates.view",
    ],
  },
  {
    id: "role.account_manager",
    name: "Account Manager",
    description: "Customer sales — read-only view across the customer's portfolio.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.engagements.view",
    ],
  },
  {
    id: "role.rnd_maintenance",
    name: "R&D Maintenance",
    description: "Handles patch requests and maintenance issues escalated from CS.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.updates.view",
      "perm.updates.manage",
      "perm.security.view",
      "perm.security.manage",
    ],
  },
  {
    id: "role.rnd_leadership",
    name: "R&D Leadership",
    description: "Visibility into issues escalated to R&D.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.updates.view",
      "perm.security.view",
    ],
  },
  {
    id: "role.sales_engineer",
    name: "Sales Engineer",
    description: "Technical owner of a set of customer accounts.",
    builtIn: true,
    permissionIds: [
      "perm.cases.view_all",
      "perm.cases.comment",
      "perm.projects.view",
      "perm.accounts.view",
      "perm.engagements.view",
    ],
  },
];

// ---------- Groups (sampled from production SN sys_user_group) ----------
// Each group also carries roleIds — every member inherits these on top of
// whatever roles are assigned to them directly.
const SEED_GROUPS: CsmGroup[] = [
  {
    id: "grp.cre_team",
    name: "CRE team",
    description: "Default CRE assignment group for inbound cases.",
    memberIds: ["usr.sajithe", "usr.priya", "usr.dilan"],
    roleIds: ["role.cre_engineer"],
  },
  {
    id: "grp.cre_atlas",
    name: "CRE-Atlas",
    description: "CRE sub-group focused on the Atlas account portfolio.",
    memberIds: ["usr.tharindu", "usr.bhanuka"],
    roleIds: ["role.cre_engineer"],
  },
  {
    id: "grp.cre_leads",
    name: "CRE_LEADS_GROUP",
    description: "CRE leadership escalation group.",
    memberIds: ["usr.nadeesha", "usr.sajithe"],
    roleIds: ["role.cre_lead"],
  },
  {
    id: "grp.cre_int_ocs",
    name: "CRE_INT_OCS_GRP",
    description: "On-call schedule group for the internal incident escalation flow.",
    memberIds: ["usr.sajithe", "usr.priya"],
    roleIds: ["role.cre_engineer"],
  },
  {
    id: "grp.apollo_sre",
    name: "Apollo SRE Group",
    description: "Apollo (Choreo SaaS) site reliability engineering.",
    memberIds: ["usr.maya"],
    roleIds: ["role.sre_engineer"],
  },
  {
    id: "grp.artemis_sre",
    name: "Artemis SRE Group",
    description: "Artemis (Asgardeo SaaS) site reliability engineering.",
    memberIds: ["usr.kasun"],
    roleIds: ["role.sre_engineer"],
  },
  {
    id: "grp.bcentral_sre",
    name: "B-Central SRE Group",
    description: "Ballerina Central SRE Team.",
    memberIds: ["usr.kasun"],
    roleIds: ["role.sre_engineer"],
  },
  {
    id: "grp.choreo_sre",
    name: "Choreo SRE group",
    description: "Choreo SRE Team; carries the internal incident role.",
    memberIds: ["usr.maya", "usr.kasun"],
    roleIds: ["role.sre_engineer"],
  },
  {
    id: "grp.bijira_sre",
    name: "Bijira SRE group",
    description: "Bijira SRE Team.",
    memberIds: ["usr.bhanuka"],
    roleIds: ["role.sre_engineer"],
  },
  {
    id: "grp.devant_sre",
    name: "Devant SRE group",
    description: "Devant SRE Team.",
    memberIds: ["usr.tharindu"],
    roleIds: ["role.sre_engineer"],
  },
  {
    id: "grp.customer_service_support",
    name: "Customer Service Support",
    description: "Customer Service Support group responsible for all chat-related support.",
    memberIds: ["usr.priya", "usr.dilan"],
    roleIds: ["role.cre_engineer"],
  },
  {
    id: "grp.consumer_service_support",
    name: "Consumer Service Support",
    description: "Consumer Service Support group responsible for all chat-related support.",
    memberIds: ["usr.dilan"],
    roleIds: ["role.cre_engineer"],
  },
  {
    id: "grp.billing_support",
    name: "Billing Support",
    description: "Billing & subscription support.",
    memberIds: ["usr.audit"],
    roleIds: ["role.account_manager"],
  },
  {
    id: "grp.business_leadership",
    name: "Business Leadership",
    description: "Executive group; routed for high-severity incident notifications.",
    memberIds: ["usr.nadeesha"],
    roleIds: ["role.leadership"],
  },
  {
    id: "grp.choreo_engineering_mgmt",
    name: "Choreo Engineering Management",
    description: "Cross-team engineering management for the Choreo product line.",
    memberIds: ["usr.nadeesha"],
    roleIds: ["role.operational_lead"],
  },
];

// ---------- Users (~10) ----------
const SEED_USERS: CsmUser[] = [
  {
    id: "usr.sajithe",
    name: "Sajith Ekanayaka",
    email: "sajithe@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E1",
    roleIds: ["role.cre_lead"],
    groupIds: ["grp.cre_team", "grp.cre_leads", "grp.cre_int_ocs"],
    lastActiveAt: daysAgo(0),
  },
  {
    id: "usr.priya",
    name: "Priya N.",
    email: "priya@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E2",
    roleIds: ["role.cre_engineer"],
    groupIds: ["grp.cre_team", "grp.cre_int_ocs", "grp.customer_service_support"],
    lastActiveAt: daysAgo(1),
  },
  {
    id: "usr.dilan",
    name: "Dilan W.",
    email: "dilanw@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E3",
    roleIds: ["role.cre_engineer"],
    groupIds: ["grp.cre_team", "grp.customer_service_support", "grp.consumer_service_support"],
    lastActiveAt: daysAgo(0),
  },
  {
    id: "usr.maya",
    name: "Maya R.",
    email: "mayar@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E4",
    roleIds: ["role.sre_engineer"],
    groupIds: ["grp.apollo_sre", "grp.choreo_sre"],
    lastActiveAt: daysAgo(2),
  },
  {
    id: "usr.tharindu",
    name: "Tharindu A.",
    email: "tharinduab@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E5",
    roleIds: ["role.fde_engineer"],
    groupIds: ["grp.cre_atlas", "grp.devant_sre"],
    lastActiveAt: daysAgo(0),
  },
  {
    id: "usr.bhanuka",
    name: "Bhanuka P.",
    email: "bhanukap@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E6",
    roleIds: ["role.sre_engineer"],
    groupIds: ["grp.cre_atlas", "grp.bijira_sre"],
    lastActiveAt: daysAgo(3),
  },
  {
    id: "usr.kasun",
    name: "Kasun H.",
    email: "kasunh@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E7",
    roleIds: ["role.sre_lead"],
    groupIds: ["grp.artemis_sre", "grp.bcentral_sre", "grp.choreo_sre"],
    lastActiveAt: daysAgo(5),
  },
  {
    id: "usr.nadeesha",
    name: "Nadeesha S.",
    email: "nadeesha@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E8",
    roleIds: ["role.operational_lead"],
    groupIds: ["grp.cre_leads", "grp.business_leadership", "grp.choreo_engineering_mgmt"],
    lastActiveAt: daysAgo(1),
  },
  {
    id: "usr.audit",
    name: "Anjana K.",
    email: "anjana@wso2.com",
    status: "Active",
    externalId: "asg-01H8X4S6E9",
    roleIds: ["role.account_manager"],
    groupIds: ["grp.billing_support"],
    lastActiveAt: daysAgo(14),
  },
  {
    id: "usr.invited_intern",
    name: "Asanka R.",
    email: "asanka.intern@wso2.com",
    status: "Invited",
    externalId: "asg-01H8X4S6EA",
    roleIds: ["role.pre_sales"],
    groupIds: [],
    lastActiveAt: daysAgo(0),
  },
];

// ---------- Runtime store (mutable in-session) ----------
const permissions: CsmPermission[] = SEED_PERMISSIONS.map((p) => ({ ...p }));
const roles: CsmRole[] = SEED_ROLES.map((r) => ({
  ...r,
  permissionIds: [...r.permissionIds],
}));
const groups: CsmGroup[] = SEED_GROUPS.map((g) => ({
  ...g,
  memberIds: [...g.memberIds],
  roleIds: [...g.roleIds],
}));
const users: CsmUser[] = SEED_USERS.map((u) => ({
  ...u,
  roleIds: [...u.roleIds],
  groupIds: [...u.groupIds],
}));

// ---------- Reads ----------
export function getMockPermissions(): CsmPermission[] {
  return permissions.map((p) => ({ ...p }));
}

export function getMockRoles(): CsmRole[] {
  return roles.map((r) => ({ ...r, permissionIds: [...r.permissionIds] }));
}

export function getMockRoleById(id: string): CsmRole | undefined {
  const r = roles.find((x) => x.id === id);
  return r ? { ...r, permissionIds: [...r.permissionIds] } : undefined;
}

function cloneGroup(g: CsmGroup): CsmGroup {
  return { ...g, memberIds: [...g.memberIds], roleIds: [...g.roleIds] };
}

export function getMockGroups(): CsmGroup[] {
  return groups.map(cloneGroup);
}

export function getMockGroupById(id: string): CsmGroup | undefined {
  const g = groups.find((x) => x.id === id);
  return g ? cloneGroup(g) : undefined;
}

export function getMockUsers(): CsmUser[] {
  return users.map((u) => ({
    ...u,
    roleIds: [...u.roleIds],
    groupIds: [...u.groupIds],
  }));
}

export function getMockUserById(id: string): CsmUser | undefined {
  const u = users.find((x) => x.id === id);
  return u
    ? { ...u, roleIds: [...u.roleIds], groupIds: [...u.groupIds] }
    : undefined;
}

export function getPermissionsByCategory(): Record<
  CsmPermissionCategory,
  CsmPermission[]
> {
  const out: Record<CsmPermissionCategory, CsmPermission[]> = {
    cases: [],
    projects: [],
    accounts: [],
    engagements: [],
    updates: [],
    security: [],
    time_cards: [],
    admin: [],
  };
  for (const p of permissions) out[p.category].push({ ...p });
  return out;
}

// ---------- Mutations (assignments) ----------
function uniq<T>(arr: T[]): T[] {
  return Array.from(new Set(arr));
}

export function setUserRoles(userId: string, roleIds: string[]): CsmUser {
  const u = users.find((x) => x.id === userId);
  if (!u) throw new Error(`User ${userId} not found`);
  u.roleIds = uniq(roleIds);
  return { ...u, roleIds: [...u.roleIds], groupIds: [...u.groupIds] };
}

export function setUserGroups(userId: string, groupIds: string[]): CsmUser {
  const u = users.find((x) => x.id === userId);
  if (!u) throw new Error(`User ${userId} not found`);
  const next = uniq(groupIds);
  // Keep group.memberIds in sync — bidirectional relation.
  const previous = u.groupIds;
  u.groupIds = next;
  for (const gid of previous) {
    if (!next.includes(gid)) {
      const g = groups.find((x) => x.id === gid);
      if (g) g.memberIds = g.memberIds.filter((m) => m !== userId);
    }
  }
  for (const gid of next) {
    if (!previous.includes(gid)) {
      const g = groups.find((x) => x.id === gid);
      if (g && !g.memberIds.includes(userId)) g.memberIds.push(userId);
    }
  }
  return { ...u, roleIds: [...u.roleIds], groupIds: [...u.groupIds] };
}

export function setRolePermissions(
  roleId: string,
  permissionIds: string[],
): CsmRole {
  const r = roles.find((x) => x.id === roleId);
  if (!r) throw new Error(`Role ${roleId} not found`);
  r.permissionIds = uniq(permissionIds);
  return { ...r, permissionIds: [...r.permissionIds] };
}

export function setGroupMembers(
  groupId: string,
  memberIds: string[],
): CsmGroup {
  const g = groups.find((x) => x.id === groupId);
  if (!g) throw new Error(`Group ${groupId} not found`);
  const next = uniq(memberIds);
  const previous = g.memberIds;
  g.memberIds = next;
  // Bidirectional: keep user.groupIds in sync.
  for (const uid of previous) {
    if (!next.includes(uid)) {
      const u = users.find((x) => x.id === uid);
      if (u) u.groupIds = u.groupIds.filter((gid) => gid !== groupId);
    }
  }
  for (const uid of next) {
    if (!previous.includes(uid)) {
      const u = users.find((x) => x.id === uid);
      if (u && !u.groupIds.includes(groupId)) u.groupIds.push(groupId);
    }
  }
  return cloneGroup(g);
}

/**
 * Replace the set of roles assigned to a group. No bidirectional sync needed —
 * roles don't track which groups reference them.
 */
export function setGroupRoles(groupId: string, roleIds: string[]): CsmGroup {
  const g = groups.find((x) => x.id === groupId);
  if (!g) throw new Error(`Group ${groupId} not found`);
  g.roleIds = uniq(roleIds);
  return cloneGroup(g);
}

/**
 * Set the list of users that have this role, editing from the role side.
 * For each user in the new list, ensure the role is present; for each user
 * not in the list, ensure it is absent.
 */
export function setRoleUsers(roleId: string, userIds: string[]): void {
  if (!roles.some((r) => r.id === roleId)) {
    throw new Error(`Role ${roleId} not found`);
  }
  const target = new Set(uniq(userIds));
  for (const u of users) {
    const has = u.roleIds.includes(roleId);
    const should = target.has(u.id);
    if (has && !should) u.roleIds = u.roleIds.filter((r) => r !== roleId);
    if (!has && should) u.roleIds = [...u.roleIds, roleId];
  }
}

/**
 * Set the list of groups that have this role, editing from the role side.
 */
export function setRoleGroups(roleId: string, groupIds: string[]): void {
  if (!roles.some((r) => r.id === roleId)) {
    throw new Error(`Role ${roleId} not found`);
  }
  const target = new Set(uniq(groupIds));
  for (const g of groups) {
    const has = g.roleIds.includes(roleId);
    const should = target.has(g.id);
    if (has && !should) g.roleIds = g.roleIds.filter((r) => r !== roleId);
    if (!has && should) g.roleIds = [...g.roleIds, roleId];
  }
}

/** Return all user ids currently holding the role (direct assignment only). */
export function getUserIdsWithRole(roleId: string): string[] {
  return users.filter((u) => u.roleIds.includes(roleId)).map((u) => u.id);
}

/** Return all group ids currently holding the role. */
export function getGroupIdsWithRole(roleId: string): string[] {
  return groups.filter((g) => g.roleIds.includes(roleId)).map((g) => g.id);
}

// ---------- Create ----------

function slugify(input: string): string {
  return input
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 32);
}

function uniqueId(prefix: string, base: string): string {
  const slug = slugify(base) || "item";
  let candidate = `${prefix}.${slug}`;
  let n = 1;
  const allIds = new Set<string>([
    ...users.map((u) => u.id),
    ...roles.map((r) => r.id),
    ...groups.map((g) => g.id),
  ]);
  while (allIds.has(candidate)) {
    n += 1;
    candidate = `${prefix}.${slug}_${n}`;
  }
  return candidate;
}

export interface CreateUserInput {
  name: string;
  email: string;
  roleIds?: string[];
  groupIds?: string[];
}

export function createUser(input: CreateUserInput): CsmUser {
  const id = uniqueId("usr", input.name);
  const u: CsmUser = {
    id,
    name: input.name.trim(),
    email: input.email.trim(),
    status: "Invited",
    externalId: `asg-${Math.random().toString(16).slice(2, 14).toUpperCase()}`,
    roleIds: uniq(input.roleIds ?? []),
    groupIds: [],
    lastActiveAt: new Date().toISOString(),
  };
  users.push(u);
  // Mirror initial group memberships into both sides of the relation.
  if (input.groupIds?.length) {
    setUserGroups(id, input.groupIds);
  }
  // Return a fresh copy reflecting current state.
  const fresh = users.find((x) => x.id === id)!;
  return { ...fresh, roleIds: [...fresh.roleIds], groupIds: [...fresh.groupIds] };
}

export interface CreateRoleInput {
  name: string;
  description: string;
  permissionIds?: string[];
}

export function createRole(input: CreateRoleInput): CsmRole {
  const id = uniqueId("role", input.name);
  const r: CsmRole = {
    id,
    name: input.name.trim(),
    description: input.description.trim(),
    permissionIds: uniq(input.permissionIds ?? []),
    builtIn: false,
  };
  roles.push(r);
  return { ...r, permissionIds: [...r.permissionIds] };
}

export interface CreateGroupInput {
  name: string;
  description: string;
  memberIds?: string[];
  roleIds?: string[];
}

export function createGroup(input: CreateGroupInput): CsmGroup {
  const id = uniqueId("grp", input.name);
  const g: CsmGroup = {
    id,
    name: input.name.trim(),
    description: input.description.trim(),
    memberIds: [],
    roleIds: uniq(input.roleIds ?? []),
  };
  groups.push(g);
  if (input.memberIds?.length) {
    setGroupMembers(id, input.memberIds);
  }
  const fresh = groups.find((x) => x.id === id)!;
  return cloneGroup(fresh);
}

// ---------- Delete (with cascade cleanup) ----------

export function deleteUser(userId: string): void {
  const idx = users.findIndex((u) => u.id === userId);
  if (idx === -1) throw new Error(`User ${userId} not found`);
  // Remove from any group memberships.
  for (const g of groups) {
    g.memberIds = g.memberIds.filter((m) => m !== userId);
  }
  users.splice(idx, 1);
}

export function deleteRole(roleId: string): void {
  const r = roles.find((x) => x.id === roleId);
  if (!r) throw new Error(`Role ${roleId} not found`);
  if (r.builtIn) {
    throw new Error(`Role ${roleId} is built-in and cannot be deleted`);
  }
  // Remove the role assignment from every user.
  for (const u of users) {
    u.roleIds = u.roleIds.filter((rid) => rid !== roleId);
  }
  const idx = roles.findIndex((x) => x.id === roleId);
  roles.splice(idx, 1);
}

export function deleteGroup(groupId: string): void {
  const g = groups.find((x) => x.id === groupId);
  if (!g) throw new Error(`Group ${groupId} not found`);
  // Remove group reference from every user.
  for (const u of users) {
    u.groupIds = u.groupIds.filter((gid) => gid !== groupId);
  }
  const idx = groups.findIndex((x) => x.id === groupId);
  groups.splice(idx, 1);
}
