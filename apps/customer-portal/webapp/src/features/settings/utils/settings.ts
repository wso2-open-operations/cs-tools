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

import { normalizeCustomerRoles } from "@hooks/useCustomerPermissions";
import { colors } from "@wso2/oxygen-ui";
import { Code, Crown, Monitor, Shield, Star, Users } from "@wso2/oxygen-ui-icons-react";
import {
  SETTINGS_CUSTOMER_ADMIN_ROLES,
  NULL_PLACEHOLDER,
  SETTINGS_AVATAR_BACKGROUND_COLORS,
} from "@features/settings/constants/settingsConstants";
import type { SettingsRoleBadge } from "@features/settings/types/settings";
import type { ProjectContact } from "@features/settings/types/users";

/**
 * Returns all applicable role badges for a contact.
 *
 * @param contact - The project contact.
 * @returns Array of role badges to display.
 */
/**
 * Reports whether the user's roles include the customer admin role, in either
 * of the spellings entity-service uses (see SETTINGS_CUSTOMER_ADMIN_ROLES).
 * Compare through this rather than against one raw string, which would stop
 * matching at cutover.
 *
 * @param {string[] | undefined} roles - Roles as reported by entity-service.
 * @returns {boolean} Whether the user is a customer admin.
 */
export function hasCustomerAdminRole(roles: string[] | undefined): boolean {
  return (roles ?? []).some((r) =>
    SETTINGS_CUSTOMER_ADMIN_ROLES.includes(r.trim().toLowerCase()),
  );
}

/**
 * Maps role strings to user-friendly labels. Returns highest role when multiple roles present.
 * Supports both ServiceNow prefixes (e.g. sn_customerservice.customer_admin) and
 * CSM database/Postgres roles (e.g. customer_admin, admin).
 *
 * @param {string[] | undefined} roles - Array of role strings from users/me endpoint.
 * @returns {string} The user-friendly role label.
 */
export function getRoleLabel(roles: string[] | undefined): string {
  if (!roles || roles.length === 0) return "Not Available";

  const canonicalRoles = normalizeCustomerRoles(roles);
  const rawLower = roles.map((r) => r.trim().toLowerCase());

  // 1. Admin (ServiceNow, CSM Postgres, or Asgardeo variants)
  if (
    canonicalRoles.includes("admin") ||
    canonicalRoles.includes("customer_admin") ||
    rawLower.includes("customer_admin") ||
    rawLower.includes("admin")
  ) {
    return "Admin";
  }

  // 2. Partner Admin
  if (
    canonicalRoles.includes("partner_admin") ||
    rawLower.includes("partner_admin")
  ) {
    return "Partner Admin";
  }

  // 3. Lead
  if (rawLower.includes("lead")) {
    return "Lead";
  }

  // 4. Security User
  if (
    rawLower.includes("security_user") ||
    rawLower.includes("security") ||
    rawLower.includes("sn_customerservice.security_user")
  ) {
    return "Security User";
  }

  // 5. Partner User
  if (
    canonicalRoles.includes("partner_user") ||
    rawLower.includes("partner")
  ) {
    return "Partner";
  }

  // 6. Internal / Agent
  if (
    canonicalRoles.includes("internal") ||
    canonicalRoles.includes("agent") ||
    rawLower.includes("internal") ||
    rawLower.includes("agent")
  ) {
    return "Internal User";
  }

  // 7. System User (only when explicitly flagged as a system/integration user)
  if (
    rawLower.includes("system_user") ||
    rawLower.includes("system") ||
    rawLower.includes("integration_user") ||
    rawLower.includes("sn_customerservice.integration_user")
  ) {
    return "System User";
  }

  // 8. Portal User (customer user persona)
  return "Portal User";
}

export function getRoleBadges(contact: ProjectContact): SettingsRoleBadge[] {
  const badges: SettingsRoleBadge[] = [];

  if (contact.isCsAdmin || contact.isAccountAdmin) {
    badges.push({ label: "Admin", Icon: Crown, chipColor: "primary" });
  }

  if (contact.isLead) {
    badges.push({ label: "Lead", Icon: Star, chipColor: "warning" });
  }

  if (contact.isCsIntegrationUser) {
    badges.push({ label: "System User", Icon: Code, chipColor: "info" });
  } else {
    if (contact.isPortalUser) {
      badges.push({ label: "Portal User", Icon: Monitor, chipColor: "default" });
    }
    if (contact.isSecurityContact) {
      badges.push({ label: "Security User", Icon: Shield, chipColor: "error" });
    }
    if (!contact.isPortalUser && !contact.isSecurityContact) {
      badges.push({ label: "Portal User", Icon: Monitor, chipColor: "default" });
    }
  }

  if (contact.account?.classification === "Partner") {
    badges.push({ label: "Partner", Icon: Users, chipColor: "primary" });
  }

  return badges;
}

/**
 * Returns sx props for role chips based on their color category.
 *
 * @param chipColor - The color key (primary, info, warning, etc).
 * @returns MUI sx object for the chip.
 */
export function getRoleChipSx(chipColor: string): object {
  const purple = colors.purple?.[600] ?? "#7c3aed";
  const baseStyles = {
    typography: "caption",
    "& .MuiChip-icon": { ml: 0.75, mr: 0.5 },
    "& .MuiChip-label": { pl: 0.5 },
  };
  switch (chipColor) {
    case "primary":
      return {
        ...baseStyles,
        color: purple,
        borderColor: purple,
        "& .MuiChip-icon": { ml: 0.75, mr: 0.5, color: purple },
      };
    default:
      return baseStyles;
  }
}

/**
 * Returns initials from name or email.
 *
 * @param firstName - First name.
 * @param lastName - Last name.
 * @param email - Email fallback.
 * @returns 1–2 character initials.
 */
export function getInitials(
  firstName?: string,
  lastName?: string,
  email?: string,
): string {
  if (firstName || lastName) {
    const first = (firstName ?? "").charAt(0).toUpperCase();
    const last = (lastName ?? "").charAt(0).toUpperCase();
    if (first || last) return `${first}${last}`.trim() || NULL_PLACEHOLDER;
  }
  if (email) {
    const parts = email.split("@")[0];
    return parts.length >= 2
      ? parts.slice(0, 2).toUpperCase()
      : parts.toUpperCase();
  }
  return "?";
}

/**
 * Returns a stable avatar color from a string id (e.g. contact.id or email).
 *
 * @param id - Stable identifier for the contact.
 * @returns Hex color string.
 */
export function getAvatarColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash << 5) - hash + id.charCodeAt(i);
    hash |= 0;
  }
  const index = Math.abs(hash) % SETTINGS_AVATAR_BACKGROUND_COLORS.length;
  return SETTINGS_AVATAR_BACKGROUND_COLORS[index] as string;
}
