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

import { MessageSquare, OctagonAlert, RefreshCcw, Settings, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ItemType, Priority, Status } from "./ItemCard";
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
};

export const PRIORITY_CHIP_COLOR_CONFIG: Record<Priority, ChipProps["color"]> = {
  low: "success",
  medium: "info",
  high: "primary",
};

export const STATUS_CHIP_COLOR_CONFIG: Record<Status, ChipProps["color"]> = {
  "in progress": "info",
  open: "primary",
  resolved: "success",
  waiting: "info",
  closed: "default",
  active: "info",
  scheduled: "warning",
  approved: "success",
  draft: "default",
  rejected: "error",
  "pending approval": "warning",
};
