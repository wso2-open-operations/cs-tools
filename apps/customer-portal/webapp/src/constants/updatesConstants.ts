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

import type React from "react";
import {
  Package,
  CircleCheck,
  CircleAlert,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import type { StatCardColor } from "@constants/dashboardConstants";

export interface UpdatesStatConfigItem {
  id:
    | "productsTracked"
    | "totalUpdatesInstalled"
    | "totalUpdatesPending"
    | "securityUpdatesPending";
  label: string;
  icon: React.ElementType;
  iconColor: StatCardColor;
  tooltipText: string;
}

export const UPDATES_STATS: UpdatesStatConfigItem[] = [
  {
    id: "productsTracked",
    label: "Products Tracked",
    icon: Package,
    iconColor: "info",
    tooltipText: "Number of products being tracked for updates",
  },
  {
    id: "totalUpdatesInstalled",
    label: "Total Updates Installed",
    icon: CircleCheck,
    iconColor: "success",
    tooltipText:
      "Total updates installed across all products (Regular and Security)",
  },
  {
    id: "totalUpdatesPending",
    label: "Total Updates Pending",
    icon: CircleAlert,
    iconColor: "warning",
    tooltipText: "Total updates pending installation (Regular and Security)",
  },
  {
    id: "securityUpdatesPending",
    label: "Security Updates Pending",
    icon: Shield,
    iconColor: "error",
    tooltipText: "Security updates pending installation - action required",
  },
];
