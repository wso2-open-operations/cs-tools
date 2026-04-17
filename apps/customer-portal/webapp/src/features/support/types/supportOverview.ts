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

import type { ComponentType, ReactNode } from "react";
import type { SxProps, Theme } from "@wso2/oxygen-ui";

/**
 * Accent palette for the overview card header icon container.
 */
export enum SupportOverviewIconVariant {
  Orange = "orange",
  Blue = "blue",
}

export type SupportOverviewHeaderAction = {
  label: string;
  onClick?: () => void;
};

export type SupportOverviewFooterButton = {
  label: string;
  onClick?: () => void;
};

export type SupportOverviewCardProps = {
  title: string;
  subtitle: string;
  icon: ComponentType<{ size?: number; color?: string }>;
  iconVariant?: SupportOverviewIconVariant;
  children: ReactNode;
  footerButtonLabel?: string;
  onFooterClick?: () => void;
  footerButtons?: SupportOverviewFooterButton[];
  sx?: SxProps<Theme>;
  isError?: boolean;
  headerAction?: SupportOverviewHeaderAction;
};
