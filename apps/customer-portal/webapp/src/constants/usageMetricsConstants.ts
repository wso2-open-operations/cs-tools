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

import { UsageTimeRange } from "@/types/usage";

export const USAGE_LINE_CHART_MARGIN = {
  top: 5,
  right: 40,
  left: 20,
  bottom: 40,
};

export const USAGE_TIME_RANGE_LABELS: Record<UsageTimeRange, string> = {
  [UsageTimeRange.THREE_MONTHS]: "Last 3 months",
  [UsageTimeRange.SIX_MONTHS]: "Last 6 months",
  [UsageTimeRange.TWELVE_MONTHS]: "Last 12 months",
  [UsageTimeRange.CUSTOM]: "Custom range",
};
