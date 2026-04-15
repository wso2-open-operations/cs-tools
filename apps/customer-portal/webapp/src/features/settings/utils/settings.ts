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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { colors } from "@wso2/oxygen-ui";
import type { ComponentType } from "react";
import { NULL_PLACEHOLDER } from "@features/settings/constants/settingsConstants";
import type { ProjectContact } from "@features/settings/types/users";
import { Code, Crown, Monitor, Shield, Users } from "@wso2/oxygen-ui-icons-react";

/** Priority: Admin > System User > Portal User (default). */

interface RoleBadge {
  label: string;
  Icon: ComponentType<{ size?: number }>;
  chipColor: "primary" | "info" | "error" | "default" | "warning";
}

/**
 * Returns all applicable role badges for a contact.
 *
 * @param {ProjectContact} contact - The project contact.
 * @returns {RoleBadge[]} Array of role badges to display.
 */
export function getRoleBadges(contact: ProjectContact): RoleBadge[] {
  const badges: RoleBadge[] = [];

  if (contact.isCsAdmin) {
    badges.push({ label: "Admin", Icon: Crown, chipColor: "primary" });
  }

  if (contact.isCsIntegrationUser) {
    badges.push({ label: "System User", Icon: Code, chipColor: "info" });
  } else if (contact.isSecurityContact) {
    badges.push({ label: "Security User", Icon: Shield, chipColor: "error" });
  } else {
    badges.push({ label: "Portal User", Icon: Monitor, chipColor: "default" });
  }

  if (contact.account?.classification === "Partner") {
    badges.push({ label: "Partner", Icon: Users, chipColor: "warning" });
  }

  return badges;
}

/**
 * Returns sx props for role chips based on their color category.
 *
 * @param {string} chipColor - The color key (primary, info, warning, etc).
 * @returns {object} MUI sx object for the chip.
 */
export function getRoleChipSx(chipColor: string): object {
  const purple = colors.purple?.[600] ?? "#7c3aed";
  // Base styles shared by all chips
  const baseStyles = {
    typography: "caption",
    "& .MuiChip-icon": { ml: 0.75, mr: 0.5 },
    "& .MuiChip-label": { pl: 0.5 },
  };
  if (chipColor === "primary") {
    return {
      ...baseStyles,
      color: purple,
      borderColor: purple,
      "& .MuiChip-icon": { ml: 0.75, mr: 0.5, color: purple },
    };
  }
  return baseStyles;
}

/**
 * Returns initials from name or email.
 *
 * @param {string} [firstName] - First name.
 * @param {string} [lastName] - Last name.
 * @param {string} [email] - Email fallback.
 * @returns {string} 1–2 character initials.
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

const AVATAR_COLORS = [
  colors.purple[600],
  colors.blue[600],
  colors.green[600],
  colors.orange[600],
  colors.pink[500],
] as const;

/**
 * Returns a stable avatar color from a string id (e.g. contact.id or email).
 *
 * @param {string} id - Stable identifier for the contact.
 * @returns {string} Hex color string.
 */
export function getAvatarColor(id: string): string {
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash << 5) - hash + id.charCodeAt(i);
    hash |= 0;
  }
  const index = Math.abs(hash) % AVATAR_COLORS.length;
  return AVATAR_COLORS[index] as string;
}
