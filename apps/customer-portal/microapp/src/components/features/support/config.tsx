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
  Briefcase,
  MessageSquare,
  OctagonAlert,
  RefreshCcw,
  Settings,
  Shield,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import type { ItemType } from "./ItemCard";
import { colors, type ChipProps } from "@wso2/oxygen-ui";

export const TYPE_CONFIG: Record<ItemType, { icon: LucideIcon; color: string }> = {
  case: {
    icon: OctagonAlert,
    color: colors.red[500],
  },
  chat: {
    icon: MessageSquare,
    color: colors.blue[500],
  },
  service: {
    icon: Settings,
    color: colors.purple[500],
  },
  change: {
    icon: RefreshCcw,
    color: colors.cyan[500],
  },
  sra: {
    icon: Shield,
    color: colors.orange[500],
  },
  engagement: {
    icon: Briefcase,
    color: colors.grey[600],
  },
};

export const PRIORITY_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  13: "success",
  12: "info",
  11: "primary",
  10: "primary",
};

export const IMPACT_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  1: "error",
  2: "warning",
  3: "success",
};

export const CASE_STATUS_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  1: "info",
  10: "primary",
  18: "success",
  1003: "info",
  6: "default",
  3: "info",
  1006: "info",
};

export const CONVERSATION_STATUS_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  1: "info",
  2: "success",
  3: "success",
  4: "primary",
  5: "default",
};

export const CHANGE_REQUEST_STATUS_CHIP_COLOR_CONFIG: Record<string, ChipProps["color"]> = {
  "-5": "default",
  "-4": "info",
  "-3": "warning",
  "5": "warning",
  "-2": "primary",
  "-1": "primary",
  "0": "info",
  "1": "info",
  "2": "error",
  "3": "success",
  "4": "default",
};

export const STRING_OVERRIDES = {
  // Engagement Type Labels Overrides
  "New Feature / Improvement": "Improvement",

  // Severity Type Names
  "Low (P4)": "S4",
  "Medium (P3)": "S3",
  "High (P2)": "S2",
  "Critical (P1)": "S1",
  "Catastrophic (P0)": "S0",
};
