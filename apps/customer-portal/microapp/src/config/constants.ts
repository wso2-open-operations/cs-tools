// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { colors } from "@wso2/oxygen-ui";
import {
  Calendar,
  CircleAlert,
  Cloud,
  MessageSquare,
  Moon,
  RefreshCcw,
  Settings,
  Users,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import type {
  ProjectMetricKey,
  ProjectMetricMeta,
  ProjectStatus,
  ProjectType,
} from "@root/src/components/features/projects";

export const INPUT_INVALID_MSG_GATEWAY = "INPUT_INVALID_MSG_GATEWAY";

export const PROJECT_METRIC_META: Record<ProjectMetricKey, ProjectMetricMeta> = {
  cases: { label: "Cases:", color: colors.red[300], icon: CircleAlert },
  chats: { label: "Chats:", color: colors.indigo[300], icon: MessageSquare },
  service: { label: "Service:", color: colors.purple[300], icon: Settings },
  change: { label: "Change:", color: colors.cyan[300], icon: RefreshCcw },
  users: { label: "Users:", color: "text.primary", icon: Users },
  date: { label: "Date:", icon: Calendar },
};

export const PROJECT_TYPE_META: Record<ProjectType, { icon: LucideIcon }> = {
  Regular: { icon: Moon },
  "Managed Cloud": { icon: Cloud },
};

export const PROJECT_STATUS_META: Record<ProjectStatus, { color: "success" | "warning" }> = {
  "All Good": { color: "success" },
  "Needs Attention": { color: "warning" },
};
