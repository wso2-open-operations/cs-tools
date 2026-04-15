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
  BarChart3,
  Briefcase,
  Cog,
  FolderOpen,
  Headset,
  LayoutDashboard,
  Megaphone,
  RefreshCw,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";

export interface AppShellNavItem {
  id: string;
  label: string;
  path: string;
  icon: ComponentType;
}

// Navigation items for the app shell.
export const APP_SHELL_NAV_ITEMS: AppShellNavItem[] = [
  { id: "dashboard", label: "Dashboard", path: "dashboard", icon: LayoutDashboard },
  { id: "support", label: "Support", path: "support", icon: Headset },
  { id: "operations", label: "Operations", path: "operations", icon: Cog},
  { id: "updates", label: "Updates", path: "updates", icon: RefreshCw },
  {
    id: "security-center",
    label: "Security Center",
    path: "security-center",
    icon: Shield,
  },
  { id: "engagements", label: "Engagements", path: "engagements", icon: Briefcase },
  {
    id: "usage-metrics",
    label: "Usage & Metrics",
    path: "usage-metrics",
    icon: BarChart3,
  },
  {
    id: "project-details",
    label: "Project Details",
    path: "project-details",
    icon: FolderOpen,
  },
  {
    id: "announcements",
    label: "Announcements",
    path: "announcements",
    icon: Megaphone,
  },
];

// URL for privacy policy.
export const PRIVACY_POLICY_URL: string = "https://wso2.com/privacy-policy/";

// URL for terms of service.
export const TERMS_OF_SERVICE_URL: string = "https://wso2.com/terms-of-use/";

// Company name.
export const COMPANY_NAME: string = "WSO2 LLC";

// URL for join our community.
export const JOIN_COMMUNITY_URL: string = "https://wso2.com/community";
