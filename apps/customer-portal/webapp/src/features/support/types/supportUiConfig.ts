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

import type { ComponentType } from "react";
import type { ProjectSupportStats } from "@features/project-hub/types/projects";
import type { CaseMetadataResponse } from "@features/support/types/cases";

/** Configuration for support / operations / engagements stat cards (shared grid). */
export type SupportStatConfig<Key = keyof ProjectSupportStats> = {
  iconColor: "primary" | "secondary" | "success" | "error" | "info" | "warning";
  icon: ComponentType<{ size?: number; color?: string }>;
  key: Key;
  label: string;
  secondaryIcon?: ComponentType<{ size?: number; color?: string }>;
};

/** Case details tab row (label + icon). */
export type CaseDetailsTabConfig = {
  label: string;
  Icon: ComponentType<{ size?: number }>;
};

export type CaseStatusPaletteIntent = "error" | "warning" | "success" | "info";

/** Case status action in the case details action row. */
export type CaseStatusAction = {
  label: string;
  Icon: ComponentType<{ size?: number }>;
  paletteIntent: CaseStatusPaletteIntent;
};

/** All-cases list filter field mapping. */
export type AllCasesFilterDefinition = {
  id: string;
  metadataKey: keyof CaseMetadataResponse;
  filterKey: string;
  useLabelAsValue?: boolean;
};

/** All-conversations list filter field mapping. */
export type AllConversationsFilterDefinition = {
  id: string;
  metadataKey: keyof CaseMetadataResponse;
  filterKey: string;
};

/** Announcements page filter state shape. */
export type AnnouncementFilterValues = {
  [key: string]: string | undefined;
  statusId?: string;
};

export type AnnouncementFilterDefinition = {
  filterKey: string;
  id: string;
  metadataKey: keyof CaseMetadataResponse;
  useLabelAsValue?: boolean;
};

/** Case type object for selects and stats. */
export type CaseTypeObject = {
  id: string;
  label: string;
};
