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

import type { JSX } from "react";
import type { StatCardColor } from "@features/dashboard/types/dashboard";

// Enum for trend direction.
export enum TrendDirection {
  UP = "up",
  DOWN = "down",
}

// Enum for trend color.
export enum TrendColor {
  SUCCESS = "success",
  ERROR = "error",
  INFO = "info",
  WARNING = "warning",
}

// Type for trend data.
export type TrendData = {
  value: string;
  direction: TrendDirection;
  color: TrendColor;
};

// Trend indicator props.
export type TrendIndicatorProps = {
  trend?: TrendData;
  isLoading?: boolean;
  isError?: boolean;
};

// Stat card props.
export type StatCardProps = {
  label: string;
  value: string | number;
  icon: JSX.Element;
  iconColor: StatCardColor;
  tooltipText: string;
  trend?: TrendData;
  showTrend?: boolean;
  isLoading?: boolean;
  isError?: boolean;
  isTrendError?: boolean;
  onClick?: () => void;
};
