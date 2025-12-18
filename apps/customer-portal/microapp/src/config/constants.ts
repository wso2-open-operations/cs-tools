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

import {
  AutorenewOutlined,
  Bedtime,
  CalendarMonthOutlined,
  ChatBubbleOutline,
  Cloud,
  ErrorOutline,
  PeopleAltOutlined,
  Report,
  SettingsOutlined,
  ThumbUpAlt,
  type SvgIconComponent,
} from "@mui/icons-material";
import type { ProjectMetricKey, ProjectMetricMeta, ProjectStatus, ProjectType } from "@features/projects";

export const INPUT_INVALID_MSG_GATEWAY = "INPUT_INVALID_MSG_GATEWAY";

export const PROJECT_METRIC_META: Record<ProjectMetricKey, ProjectMetricMeta> = {
  cases: { label: "Cases:", color: "semantic.portal.accent.orange", icon: ErrorOutline },
  chats: { label: "Chats:", color: "semantic.portal.accent.blue", icon: ChatBubbleOutline },
  service: { label: "Service:", color: "semantic.portal.accent.purple", icon: SettingsOutlined },
  change: { label: "Change:", color: "semantic.portal.accent.cyan", icon: AutorenewOutlined },
  users: { label: "Users:", color: "text.primary", icon: PeopleAltOutlined },
  date: { label: "Date:", icon: CalendarMonthOutlined },
};

export const PROJECT_TYPE_META: Record<ProjectType, { icon: SvgIconComponent }> = {
  Regular: { icon: Bedtime },
  "Managed Cloud": { icon: Cloud },
};

export const PROJECT_STATUS_META: Record<ProjectStatus, { color: "success" | "warning"; icon: SvgIconComponent }> = {
  "All Good": { color: "success", icon: ThumbUpAlt },
  "Needs Attention": { color: "warning", icon: Report },
};
