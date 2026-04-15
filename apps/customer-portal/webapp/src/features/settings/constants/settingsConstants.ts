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

import { Code, Crown, Monitor, Shield } from "@wso2/oxygen-ui-icons-react";

/** Placeholder for empty/null values in user management UI. */
export const NULL_PLACEHOLDER = "--";

/** Role configuration for user management: labels, icons, permissions, palette keys. */
export const ROLE_CONFIG = [
  {
    id: "admin",
    label: "Admin",
    Icon: Crown,
    paletteKey: "secondary" as const,
    permissions: [
      "Manage users within the assigned project",
      "Add or remove project members",
    ],
  },
  {
    id: "portal_user",
    label: "Portal User",
    Icon: Monitor,
    paletteKey: "info" as const,
    permissions: [
      "Can log in to and access the Support Portal",
      "Create and manage support cases within assigned projects",
    ],
  },
  {
    id: "system_user",
    label: "System User",
    Icon: Code,
    paletteKey: "info" as const,
    permissions: [
      "Used exclusively for system to system integrations",
      "Cannot log in to the Support Portal",
    ],
  },
  {
    id: "security_user",
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
