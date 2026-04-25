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

/** Which main block `ProjectHub` renders below the header. */
export enum ProjectHubContentView {
  REDIRECT_LOADER = "redirect_loader",
  AUTH_PENDING = "auth_pending",
  LOADING_SKELETONS = "loading_skeletons",
  ERROR = "error",
  NO_GRID = "no_grid",
  EMPTY_STATE = "empty_state",
  PROJECT_LIST = "project_list",
}

/** Typography variant for `ClampedTextWithTooltip`. */
export enum ClampedTextVariant {
  H6 = "h6",
  BODY2 = "body2",
}

export type ProjectCardProps = {
  date: string;
  id: string;
  activeCasesCount: number;
  activeChatsCount: number;
  actionRequiredCount: number;
  closureState?: string | null;
  onViewDashboard?: () => void;
  projectKey: string;
  slaStatus: string;
  title: string;
};

export type ProjectCardBadgesProps = {
  projectKey: string;
};

export type ProjectCardStatsProps = {
  activeChatsCount: number | undefined;
  date: string;
  activeCasesCount: number | undefined;
  actionRequiredCount: number | undefined;
  isError?: boolean;
  isLoading?: boolean;
};

export type ProjectCardInfoProps = {
  title: string;
};

export type ProjectCardActionsProps = {
  onViewDashboard?: () => void;
};

export type ClampedTextWithTooltipProps = {
  text: string;
  lineClamp: number;
  variant?: ClampedTextVariant;
  sx?: Record<string, unknown>;
};
