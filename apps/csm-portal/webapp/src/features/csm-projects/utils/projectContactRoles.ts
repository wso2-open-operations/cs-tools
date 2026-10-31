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

import type { BeProjectContact } from "@api/backend/types";
import { colors } from "@wso2/oxygen-ui";
import { Briefcase, Code, Crown, Monitor, Shield, Star } from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";

type RoleIcon = ComponentType<{ size?: number }>;

export type ProjectContactRoleChip = {
  label: string;
  Icon?: RoleIcon;
  chipColor: "primary" | "warning" | "info" | "default" | "error";
};

const ADMIN_CHIP: ProjectContactRoleChip = { label: "Admin", Icon: Crown, chipColor: "primary" };
const LEAD_CHIP: ProjectContactRoleChip = { label: "Lead", Icon: Star, chipColor: "warning" };
const SYSTEM_CHIP: ProjectContactRoleChip = { label: "System User", Icon: Code, chipColor: "info" };
const PORTAL_CHIP: ProjectContactRoleChip = { label: "Portal User", Icon: Monitor, chipColor: "default" };
const SECURITY_CHIP: ProjectContactRoleChip = { label: "Security User", Icon: Shield, chipColor: "error" };
// The label the CSM contacts table used before the customer-portal chip
// names. The customer portal still omits this role; CSM keeps showing it.
const BUSINESS_CHIP: ProjectContactRoleChip = {
  label: "Business Contact",
  Icon: Briefcase,
  chipColor: "default",
};

const GLOBAL_ADMIN_ACCOUNT_ROLES = ["customer_admin", "partner_admin"];

/** Same outlined-chip spacing the customer portal uses, with Admin in purple. */
export function projectContactRoleChipSx(chipColor: ProjectContactRoleChip["chipColor"]): object {
  const base = {
    typography: "caption",
    "& .MuiChip-icon": { ml: 0.75, mr: 0.5 },
    "& .MuiChip-label": { pl: 0.5 },
  };
  if (chipColor !== "primary") return base;
  const purple = colors.purple?.[600] ?? "#7c3aed";
  return {
    ...base,
    color: purple,
    borderColor: purple,
    "& .MuiChip-icon": { ml: 0.75, mr: 0.5, color: purple },
  };
}

/**
 * Chip labels for one project-contact row, matching the customer portal's
 * `getRoleBadges` for the flags that search can supply.
 *
 * Admin shows when the project roles include ADMIN or the account roles
 * include customer_admin or partner_admin; either signal makes a project
 * admin, matching the customer portal contact list.
 * A person who is not a system user and has neither PORTAL_USER nor
 * SECURITY_CONTACT still gets Portal User. BUSINESS_CONTACT is shown as
 * Business Contact. The customer portal omits that role; CSM keeps it.
 * The customer portal's Partner chip needs the account classification, which
 * this response does not include, so it is not drawn here.
 */
export function projectContactRoleChips(
  contact: Pick<
    BeProjectContact,
    "roles" | "isCsIntegrationUser" | "accountRoles" | "customerContactPresent"
  >,
): ProjectContactRoleChip[] {
  const projectRoles = (contact.roles ?? []).map((role) => role.trim()).filter(Boolean);
  const hasProjectRole = (code: string) =>
    projectRoles.some((role) => role.toUpperCase() === code);
  const isGlobalAdmin = (contact.accountRoles ?? []).some((role) =>
    GLOBAL_ADMIN_ACCOUNT_ROLES.includes(role.trim().toLowerCase()),
  );
  const isPortalUser = hasProjectRole("PORTAL_USER");
  const isSecurityContact = hasProjectRole("SECURITY_CONTACT");
  // A row the API confirms has no linked user, and no project roles, is not a
  // portal user. An absent flag is an older response, so the fallback stays.
  const confirmedUnlinked = contact.customerContactPresent === false && projectRoles.length === 0;

  const chips: ProjectContactRoleChip[] = [];
  if (hasProjectRole("ADMIN") || isGlobalAdmin) chips.push(ADMIN_CHIP);
  if (hasProjectRole("LEAD_USER")) chips.push(LEAD_CHIP);
  if (contact.isCsIntegrationUser) {
    chips.push(SYSTEM_CHIP);
  } else {
    if (isPortalUser) chips.push(PORTAL_CHIP);
    if (isSecurityContact) chips.push(SECURITY_CHIP);
    if (!isPortalUser && !isSecurityContact && !confirmedUnlinked) chips.push(PORTAL_CHIP);
  }
  if (hasProjectRole("BUSINESS_CONTACT")) chips.push(BUSINESS_CHIP);
  return chips;
}

export function projectContactRoleLabels(
  contact: Pick<
    BeProjectContact,
    "roles" | "isCsIntegrationUser" | "accountRoles" | "customerContactPresent"
  >,
): string[] {
  return projectContactRoleChips(contact).map((chip) => chip.label);
}
