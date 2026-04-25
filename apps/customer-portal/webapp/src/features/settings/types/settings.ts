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

import type { ComponentType } from "react";
import type { CreateProjectContactRequest } from "@features/settings/types/users";
import type { ProjectContact } from "@features/settings/types/users";
import type { RegistryToken } from "@features/settings/types/registryTokens";
import type { RegistryTokenType } from "@features/settings/types/registryTokens";

/** Main Settings page tabs (User Management, AI, Registry Tokens). */
export enum SettingsPageTabId {
  USERS = "users",
  AI = "ai",
  REGISTRY_TOKENS = "registryTokens",
}

/** Registry Tokens sub-tabs (User vs Service token lists). */
export enum RegistryTokenSubTabId {
  USER = "user",
  SERVICE = "service",
}

/** Derived display status for a registry token row. */
export enum RegistryTokenDisplayStatus {
  Active = "Active",
  Expired = "Expired",
  Revoked = "Revoked",
}

/** Role ids shown in the role-permissions reference grid. */
export enum SettingsRoleInfoId {
  ADMIN = "admin",
  PORTAL_USER = "portal_user",
  SYSTEM_USER = "system_user",
  SECURITY_USER = "security_user",
}

/** Invite flow role for Add User modal (subset of project roles). */
export enum AddUserContactRole {
  PORTAL_USER = "portal_user",
  SECURITY_USER = "security_user",
  SYSTEM_USER = "system_user",
}

/** Steps in the Add User modal wizard. */
export enum AddUserModalStep {
  EMAIL = "email",
  DETAILS = "details",
}

/** Which AI patch mutation produced a success toast. */
export enum AiAssistantPatchSuccessKind {
  NOVERA = "novera",
  KB = "kb",
}

export type SettingsRoleBadge = {
  label: string;
  Icon: ComponentType<{ size?: number }>;
  chipColor: "primary" | "info" | "error" | "default" | "warning";
};

export type SettingsUserManagementProps = {
  projectId: string;
  /** When false, hide Add User and Delete user buttons. */
  canAddOrRemoveUsers?: boolean;
};

export type SettingsRegistryTokensProps = {
  projectId: string;
  isAdmin: boolean;
  isRestricted?: boolean;
};

export type SettingsAiAssistantProps = {
  projectId: string;
  canEdit?: boolean;
};

export type AddUserModalProps = {
  open: boolean;
  projectId: string;
  onClose: () => void;
  onSubmit: (data: CreateProjectContactRequest) => void;
  isSubmitting?: boolean;
};

export type EditUserModalProps = {
  open: boolean;
  contact: ProjectContact | null;
  isSubmitting?: boolean;
  onClose: () => void;
  onSubmit: (next: { isCsAdmin: boolean; isPortalUser: boolean; isSecurityContact: boolean }) => void;
};

export type RemoveUserModalProps = {
  open: boolean;
  contact: ProjectContact | null;
  isDeleting?: boolean;
  onClose: () => void;
  onConfirm: () => void;
};

export type GenerateTokenModalProps = {
  open: boolean;
  onClose: () => void;
  projectId: string;
  tokenType: RegistryTokenType;
  isAdmin: boolean;
};

export type DeleteTokenModalProps = {
  open: boolean;
  onClose: () => void;
  projectId: string;
  token: RegistryToken | null;
};

export type RegenerateTokenModalProps = {
  open: boolean;
  onClose: () => void;
  projectId: string;
  token: RegistryToken | null;
};
