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
  Package,
  CircleCheck,
  CircleAlert,
  Shield,
} from "@wso2/oxygen-ui-icons-react";
import {
  UpdatesPageTabId,
  UpdatesStatKey,
  type AllUpdatesTabFilterState,
  type UpdatesStatConfigItem,
} from "@features/updates/types/updates";
import { NULL_PLACEHOLDER } from "@constants/common";

/** Placeholder when a stat or field has no value. */
export const UPDATES_NULL_PLACEHOLDER = NULL_PLACEHOLDER;

export const UPDATES_PAGE_TABS = [
  { id: UpdatesPageTabId.MyUpdates, label: "My Updates" },
  { id: UpdatesPageTabId.All, label: "All Updates" },
];

export const UPDATES_RECOMMENDED_LEVELS_LOAD_ERROR =
  "Could not load recommended update levels.";

export const ALL_UPDATES_TAB_INITIAL_FILTER: AllUpdatesTabFilterState = {
  productName: "",
  productVersion: "",
  startLevel: "",
  endLevel: "",
};

export const ALL_UPDATES_SECTION_TITLE = "Search Update Levels";

export const ALL_UPDATES_PRODUCT_LABEL = "Product Name *";

export const ALL_UPDATES_VERSION_LABEL = "Product Version *";

export const ALL_UPDATES_START_LEVEL_LABEL = "Starting Update Level *";

export const ALL_UPDATES_END_LEVEL_LABEL = "Ending Update Level *";

export const ALL_UPDATES_SELECT_PRODUCT_PLACEHOLDER = "Select Product";

export const ALL_UPDATES_SELECT_VERSION_PLACEHOLDER = "Select Version";

export const ALL_UPDATES_SELECT_LEVEL_PLACEHOLDER = "Select Level";

export const ALL_UPDATES_SEARCH_BUTTON_LABEL = "Search";

export const ALL_UPDATES_VIEW_REPORT_BUTTON_LABEL = "View Report";

export const ALL_UPDATES_IDLE_HINT =
  "Select product, version, and update level range, then click Search to view updates.";

export const ALL_UPDATES_FILTER_OPTIONS_ERROR_MESSAGE =
  "Could not load filter options. Please try again later.";

export const ALL_UPDATES_SEARCH_ERROR_MESSAGE =
  "Failed to load update levels. Please try again.";

export const ALL_UPDATES_EMPTY_SEARCH_MESSAGE =
  "No update levels found for the selected criteria.";

export const PENDING_UPDATES_LIST_ERROR_MESSAGE =
  "Failed to load pending updates.";

export const PENDING_UPDATES_LIST_EMPTY_DESCRIPTION =
  "No pending updates found for this product and version.";

export const UPDATES_STATS_GRID_SECTION_TITLE = "Overall Update Status";

export const UPDATES_SECURITY_PENDING_ACTION_CHIP_LABEL = "Action Required";

export const PENDING_UPDATES_TABLE_VIEW_BUTTON_LABEL = "View";

export const UPDATE_PRODUCT_GRID_SECTION_TITLE = "Update Details";

export const UPDATE_PRODUCT_GRID_ERROR_MESSAGE =
  "Failed to load product updates.";

export const UPDATE_PRODUCT_GRID_EMPTY_MESSAGE = "No product updates found.";

export const UPDATES_STATS: UpdatesStatConfigItem[] = [
  {
    id: UpdatesStatKey.ProductsTracked,
    label: "Products Tracked",
    icon: Package,
    iconColor: "info",
    tooltipText: "Number of products being tracked for updates",
  },
  {
    id: UpdatesStatKey.TotalUpdatesInstalled,
    label: "Total Updates Installed",
    icon: CircleCheck,
    iconColor: "success",
    tooltipText:
      "Total updates installed across all products (Regular and Security)",
  },
  {
    id: UpdatesStatKey.TotalUpdatesPending,
    label: "Total Updates Pending",
    icon: CircleAlert,
    iconColor: "warning",
    tooltipText: "Total updates pending installation (Regular and Security)",
  },
  {
    id: UpdatesStatKey.SecurityUpdatesPending,
    label: "Security Updates Pending",
    icon: Shield,
    iconColor: "error",
    tooltipText: "Security updates pending installation - action required",
  },
];
