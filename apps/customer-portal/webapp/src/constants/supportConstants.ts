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
  CircleCheck,
  Clock,
  FileText,
  MessageSquare,
  TrendingUp,
} from "@wso2/oxygen-ui-icons-react";
import { type ComponentType } from "react";
import type { ProjectSupportStats } from "@/models/responses";

// Interface for support statistics card configuration.
export interface SupportStatConfig {
  iconColor: "primary" | "secondary" | "success" | "error" | "info" | "warning";
  icon: ComponentType;
  key: keyof ProjectSupportStats;
  label: string;
  secondaryIcon?: ComponentType;
}

// Configuration for the support statistics cards.
export const SUPPORT_STAT_CONFIGS: SupportStatConfig[] = [
  {
    icon: FileText,
    iconColor: "error",
    key: "totalCases",
    label: "Ongoing Cases",
    secondaryIcon: TrendingUp,
  },
  {
    icon: MessageSquare,
    iconColor: "success",
    key: "sessionChats",
    label: "Chat Sessions",
    secondaryIcon: Bot,
  },
  {
    icon: CircleCheck,
    iconColor: "info",
    key: "resolvedChats",
    label: "Resolved via Chat",
  },
  {
    icon: Clock,
    iconColor: "warning",
    key: "activeChats",
    label: "Active Chats",
  },
];
