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

import {
  Bot,
  Code,
  Crown,
  KeyRound,
  Monitor,
  Shield,
  Users,
} from "@wso2/oxygen-ui-icons-react";
import { colors } from "@wso2/oxygen-ui";
import {
  AddUserContactRole,
  SettingsPageTabId,
  SettingsRoleInfoId,
} from "@features/settings/types/settings";
import { NULL_PLACEHOLDER as COMMON_NULL_PLACEHOLDER } from "@constants/common";

/** Placeholder for empty/null values in user management UI. */
export const NULL_PLACEHOLDER = COMMON_NULL_PLACEHOLDER;

/** @deprecated Use NULL_PLACEHOLDER — same value for registry token tables. */
export const SETTINGS_NULL_PLACEHOLDER = NULL_PLACEHOLDER;

/** Role that can see AI Assistant tab and User Management Add/Delete. */
export const SETTINGS_CUSTOMER_ADMIN_ROLE = "sn_customerservice.customer_admin";

export const SETTINGS_PAGE_TABS = [
  {
    id: SettingsPageTabId.USERS,
    label: "User Management",
    icon: Users,
  },
  {
    id: SettingsPageTabId.AI,
    label: "AI Assistant",
    icon: Bot,
  },
  {
    id: SettingsPageTabId.REGISTRY_TOKENS,
    label: "Registry Tokens",
    icon: KeyRound,
  },
] as const;

export const SETTINGS_PROJECT_NOT_FOUND_MESSAGE =
  "Project not found. Please select a project.";

/** Role configuration for user management: labels, icons, permissions, palette keys. */
export const ROLE_CONFIG = [
  {
    id: SettingsRoleInfoId.ADMIN,
    label: "Admin",
    Icon: Crown,
    paletteKey: "secondary" as const,
    permissions: [
      "Manage users within the assigned project",
      "Add or remove project members",
    ],
  },
  {
    id: SettingsRoleInfoId.PORTAL_USER,
    label: "Portal User",
    Icon: Monitor,
    paletteKey: "info" as const,
    permissions: [
      "Can log in to and access the Support Portal",
      "Create and manage support cases within assigned projects",
    ],
  },
  {
    id: SettingsRoleInfoId.SYSTEM_USER,
    label: "System User",
    Icon: Code,
    paletteKey: "info" as const,
    permissions: [
      "Used exclusively for system to system integrations",
      "Cannot log in to the Support Portal",
    ],
  },
  {
    id: SettingsRoleInfoId.SECURITY_USER,
    label: "Security User",
    Icon: Shield,
    paletteKey: "error" as const,
    permissions: [
      "Access security advisories",
      "Create security cases",
      "View vulnerability reports",
    ],
  },
] as const;

export const SETTINGS_USER_SEARCH_PLACEHOLDER =
  "Search users by name, email, or role...";

export const SETTINGS_USER_ADD_BUTTON_LABEL = "Add User";

export const SETTINGS_USER_TABLE_HEADERS = {
  user: "User",
  role: "Role",
  status: "Status",
  actions: "Actions",
} as const;

export const SETTINGS_USER_EMPTY_MESSAGE = "No users found.";

export const SETTINGS_USER_ROLE_PERMISSIONS_TITLE = "Role Permissions";

export const SETTINGS_USER_INVITE_SUCCESS = "Invitation sent successfully";

export const SETTINGS_USER_ADD_ERROR = "Failed to add user. Please try again.";

export const SETTINGS_USER_REMOVE_SUCCESS = "User removed successfully";

export const SETTINGS_USER_REMOVE_ERROR =
  "Failed to remove user. Please try again.";

export const SETTINGS_USER_SYSTEM_NOT_SECURITY_ERROR =
  "System Users cannot be security contacts.";

export const SETTINGS_USER_SECURITY_UPDATE_SUCCESS = "Security contact updated";

export const SETTINGS_USER_UPDATE_ERROR =
  "Failed to update user. Please try again.";

export const SETTINGS_USER_EDIT_TOOLTIP = "Edit user";

export const SETTINGS_USER_REMOVE_TOOLTIP = "Remove user";

export const ADD_USER_EMAIL_REGEX = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export const ADD_USER_ROLE_OPTIONS = [
  {
    id: AddUserContactRole.PORTAL_USER,
    label: "Portal User",
    Icon: Monitor,
  },
  {
    id: AddUserContactRole.SECURITY_USER,
    label: "Security User",
    Icon: Shield,
  },
  {
    id: AddUserContactRole.SYSTEM_USER,
    label: "System User",
    Icon: Code,
  },
] as const;

export const ADD_USER_MODAL_CANCEL = "Cancel";

export const ADD_USER_MODAL_VALIDATING = "Validating...";

export const ADD_USER_MODAL_NEXT = "Next";

export const ADD_USER_DETAILS_INTRO =
  "Send an invitation to a new user to access the portal";

export const ADD_USER_FIRST_NAME_LABEL = "First Name";

export const ADD_USER_FIRST_NAME_PLACEHOLDER = "Enter first name";

export const ADD_USER_LAST_NAME_LABEL = "Last Name";

export const ADD_USER_LAST_NAME_PLACEHOLDER = "Enter last name";

export const ADD_USER_TYPE_LABEL = "User Type";

export const ADD_USER_MODAL_BACK = "Back";

export const ADD_USER_MODAL_SENDING = "Sending...";

export const ADD_USER_MODAL_SEND_INVITATION = "Send Invitation";

export const ADD_USER_MODAL_TITLE = "Add New User";

export const ADD_USER_EMAIL_STEP_DESCRIPTION =
  "Enter the email address of the user you want to add";

export const ADD_USER_EMAIL_LABEL = "Email Address";

export const ADD_USER_EMAIL_PLACEHOLDER = "user@company.com";

export const ADD_USER_EMAIL_REQUIRED_ERROR = "Email address is required";

export const ADD_USER_EMAIL_INVALID_ERROR =
  "Enter a valid email address (e.g. user@company.com)";

export const ADD_USER_EMAIL_INVALID_CONTACT_ERROR =
  "This email cannot be added.";

export const ADD_USER_EMAIL_VALIDATE_ERROR =
  "Email validation failed. Please try again.";

export const REGISTRY_TOKEN_ROBOT_NAME_REGEX = /^[a-z0-9-]+$/;

export const REGISTRY_TOKEN_EXPIRY_WARNING_DAYS = 7;

export const REGISTRY_TOKEN_DATE_LOCALE = "en-GB";

export const REGISTRY_TOKEN_TIMESTAMP_NEVER_LABEL = "Never";

export const REGISTRY_TOKEN_DESC_SYSTEM_GENERATED = "System generated token";

export const SETTINGS_AI_NOVERA_SUCCESS_MESSAGE =
  "AI Chat Assistant (Novera) was updated successfully.";

export const SETTINGS_AI_KB_SUCCESS_MESSAGE =
  "Smart Knowledge Base suggestions were updated successfully.";

export const SETTINGS_AI_PATCH_ERROR =
  "Failed to update AI assistant settings.";

export const SETTINGS_AI_HEADER_TITLE = "AI-Powered Support Assistant";

export const SETTINGS_AI_HEADER_BODY =
  "Configure Novera, your intelligent support assistant. Enable or disable specific AI capabilities based on your team's needs. Changes take effect immediately.";

export const SETTINGS_AI_CAPABILITIES_SECTION_TITLE = "Support Capabilities";

export const SETTINGS_AI_ADMIN_ONLY_HINT =
  "Only customer admins can update AI assistant settings.";

export const SETTINGS_AI_NOVERA_LABEL = "AI Chat Assistant (Novera)";

export const SETTINGS_AI_NOVERA_DESCRIPTION =
  "Enable AI-powered chat assistant to help with troubleshooting before creating cases";

export const SETTINGS_AI_KB_LABEL = "Smart Knowledge Base Suggestions";

export const SETTINGS_AI_KB_DESCRIPTION =
  "Get AI-powered article recommendations based on your issue description";

export const SETTINGS_AI_BEST_PRACTICES_TITLE = "AI Best Practices";

export const SETTINGS_AI_BEST_PRACTICES_ITEMS = [
  "Enable AI Chat Assistant to reduce case creation time by 60%",
  "Smart suggestions help users find relevant knowledge base articles",
  "Automatic categorization improves case routing and faster resolution",
  "AI insights help identify patterns and prevent recurring issues",
] as const;

export const SETTINGS_AI_ENABLED_CAPABILITIES_LABEL = (n: number) =>
  `${n} capabilities enabled`;

export const SETTINGS_AI_DISABLED_CAPABILITIES_LABEL = (n: number) =>
  `${n} capabilities disabled`;

export const REGISTRY_ADMIN_ALERT_PREFIX = "Admin View:";

export const REGISTRY_ADMIN_ALERT_BODY =
  "You can view and manage all tokens across the organization.";

export const REGISTRY_PAGE_TITLE = "Registry Tokens";

export const REGISTRY_PAGE_DESCRIPTION =
  "Manage registry tokens for WSO2 Updates 2.0. User tokens are for individual access, while service tokens are for automation and CI/CD pipelines.";

export const REGISTRY_SUBTAB_USER_BASE = "User Tokens";

export const REGISTRY_SUBTAB_SERVICE_BASE = "Service Tokens";

export const REGISTRY_STAT_TOTAL_LABEL = "Total Tokens";

export const REGISTRY_STAT_ACTIVE_LABEL = "Active Tokens";

export const REGISTRY_STAT_EXPIRING_LABEL = "Expiring Soon";

export const REGISTRY_USER_TOKENS_EMPTY = "No user tokens found.";

export const REGISTRY_SERVICE_TOKENS_EMPTY = "No service tokens found.";

export const REGISTRY_SEARCH_PLACEHOLDER_USER = "Search by user, token name...";

export const REGISTRY_SEARCH_PLACEHOLDER_SERVICE =
  "Search by token name, description...";

export const REGISTRY_GENERATE_USER_TOKEN = "Generate User Token";

export const REGISTRY_GENERATE_SERVICE_TOKEN = "Generate Service Token";

export const REGISTRY_MENU_REGENERATE = "Regenerate Secret";

export const REGISTRY_MENU_DELETE = "Delete Token";

/** Avatar background palette for user list (deterministic hash). */
export const SETTINGS_AVATAR_BACKGROUND_COLORS = [
  colors.purple[600],
  colors.blue[600],
  colors.green[600],
  colors.orange[600],
  colors.pink[500],
] as const;
