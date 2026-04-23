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

import type { ElementType, JSX } from "react";
import type { StatCardColor } from "@/features/dashboard/constants/dashboard";

// Model type for update level breakdown.
export type UpdateLevelBreakdown = { regular: number; security: number };

// Response type for updates statistics.
export type UpdatesStats = {
  productsTracked: number | null;
  totalUpdatesInstalled: number | null;
  totalUpdatesInstalledBreakdown?: UpdateLevelBreakdown;
  totalUpdatesPending: number | null;
  totalUpdatesPendingBreakdown?: UpdateLevelBreakdown;
  securityUpdatesPending: number | null;
}

// Item type for a single product recommended update level item.
export type RecommendedUpdateLevelItem = {
  productName: string;
  productBaseVersion: string;
  channel: string;
  startingUpdateLevel: number;
  endingUpdateLevel: number;
  installedUpdatesCount: number;
  installedSecurityUpdatesCount: number;
  timestamp: number;
  recommendedUpdateLevel: number;
  availableUpdatesCount: number;
  availableSecurityUpdatesCount: number;
}

// Item type for product update levels.
export type ProductUpdateLevelEntry = {
  productBaseVersion: string;
  channel: string;
  updateLevels: number[];
}

// Item type for one product's update levels.
export type ProductUpdateLevelsItem = {
  productName: string;
  productUpdateLevels: ProductUpdateLevelEntry[];
}

// Item type for a security advisory item inside an update description level.
export type SecurityAdvisory = {
  id: string;
  overview: string;
  severity: string;
  description: string;
  impact: string;
  solution: string;
  notes: string;
  credits: string;
}

// Item type for a single update description entry within an update level.
export type UpdateDescriptionLevel = {
  updateLevel: number;
  productName: string;
  productVersion: string;
  channel: string;
  updateType: string;
  updateNumber: number;
  description: string;
  instructions: string;
  bugFixes: string;
  filesAdded: string;
  filesModified: string;
  filesRemoved: string;
  bundlesInfoChanges: string | null;
  dependantReleases: string | null;
  timestamp: number;
  securityAdvisories: SecurityAdvisory[];
}

// Item type for a single update level key from POST /updates/levels/search.
export type UpdateLevelEntry = {
  updateType: string;
  updateDescriptionLevels: UpdateDescriptionLevel[];
}

// Response type for POST /updates/levels/search.
export type UpdateLevelsSearchResponse = Record<string, UpdateLevelEntry>;

// Request type for searching update levels.
export type UpdateLevelsSearchRequest = {
  startingUpdateLevel: number;
  endingUpdateLevel: number;
  productName: string;
  productVersion: string;
};

// --- UI enums ---------------------------------------------------------------

/**
 * Stat card keys for the Updates overview grid (`UPDATES_STATS`).
 */
export enum UpdatesStatKey {
  ProductsTracked = "productsTracked",
  TotalUpdatesInstalled = "totalUpdatesInstalled",
  TotalUpdatesPending = "totalUpdatesPending",
  SecurityUpdatesPending = "securityUpdatesPending",
}

/**
 * Main Updates page tab ids (`TabBar`).
 */
export enum UpdatesPageTabId {
  MyUpdates = "my-updates",
  All = "all",
}

/**
 * Pending level row update classification (chip / table).
 */
export enum UpdatePendingLevelType {
  Security = "security",
  Regular = "regular",
}

/**
 * Header / progress accent for product update cards.
 */
export enum UpdateProductCardHeaderStatus {
  Info = "info",
  Warning = "warning",
  Success = "success",
  Error = "error",
}

// --- Stat grid config --------------------------------------------------------

export type UpdatesStatConfigItem = {
  id: UpdatesStatKey;
  label: string;
  icon: ElementType;
  iconColor: StatCardColor;
  tooltipText: string;
};

// --- All Updates tab ----------------------------------------------------------

export type AllUpdatesTabFilterState = {
  productName: string;
  productVersion: string;
  startLevel: string;
  endLevel: string;
};

/** Same shape as {@link UpdateLevelsSearchRequest} (All Updates tab search). */
export type AllUpdatesTabSearchParams = UpdateLevelsSearchRequest;

export type AllUpdatesFilterValidation =
  | { valid: false }
  | { valid: true; start: number; end: number };

// --- Pending updates list / report ------------------------------------------

export type PendingUpdateLevelRow = {
  updateLevel: number;
  updateType: UpdatePendingLevelType;
};

export type PendingUpdatesListProps = {
  data: UpdateLevelsSearchResponse | null;
  isError: boolean;
  onView: (levelKey: string) => void;
};

export type UpdateLevelsReportParams = {
  productName: string;
  productVersion: string;
  startLevel: number;
  endLevel: number;
  data: UpdateLevelsSearchResponse;
};

export type UpdateLevelsReportTableRow = {
  levelKey: string;
  typeLabel: string;
  updatesCount: number;
  releaseDate: string;
  applied: string;
};

export type UpdateLevelsReportData = {
  generatedStr: string;
  productName: string;
  productVersion: string;
  startLevel: number;
  endLevel: number;
  securityCount: number;
  regularCount: number;
  mixedCount: number;
  totalUpdates: number;
  levelCount: number;
  levelsRange: string;
  tableRows: UpdateLevelsReportTableRow[];
};

export type UpdateLevelsReportModalProps = {
  open: boolean;
  reportData: UpdateLevelsReportData | null;
  onClose: () => void;
};

// --- Stats grid & cards ------------------------------------------------------

export type UpdatesStatsGridProps = {
  data: RecommendedUpdateLevelItem[] | undefined;
  isLoading: boolean;
  isError: boolean;
};

export type StatCardProps = {
  label: string;
  value?: string | number;
  icon: JSX.Element;
  iconColor: StatCardColor;
  tooltipText: string;
  isLoading?: boolean;
  isError?: boolean;
  extraContent?: JSX.Element;
};

export type UpdateProductGridProps = {
  data: RecommendedUpdateLevelItem[] | undefined;
  isLoading: boolean;
  isError: boolean;
  projectId?: string;
};

export type UpdateProductCardProps = {
  item: RecommendedUpdateLevelItem;
  onInstalledClick?: () => void;
  onPendingClick?: () => void;
};

export type UpdateCardHeaderProps = {
  productName: string;
  productBaseVersion: string;
  percentage: number;
  statusColor: UpdateProductCardHeaderStatus;
};

export type UpdateCardBreakdownProps = {
  installedRegular: number;
  installedSecurity: number;
  pendingRegular: number;
  pendingSecurity: number;
  totalPending: number;
  onInstalledClick?: () => void;
  onPendingClick?: () => void;
};

export type UpdateCardLevelsProps = {
  currentUpdateLevel: number;
  recommendedUpdateLevel: number;
  pendingLevels: number;
};
