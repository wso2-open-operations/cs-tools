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

import type { ReactNode } from "react";
import { colors, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { TrendingDown, TrendingUp } from "@wso2/oxygen-ui-icons-react";
import { WidgetBox } from "@components/ui";

export interface MetricWidgetProps {
  label: string;
  value: number | string;
  icon?: ReactNode;
  size?: "small" | "large";
  base?: boolean;
  trend?: {
    direction: "up" | "down";
    value: number | string;
  };
}

export function MetricWidget({ label, value, icon, size, base, trend }: MetricWidgetProps) {
  const small = size === "small";
  const TrendIcon = trend?.direction === "up" ? TrendingUp : TrendingDown;

  return (
    <WidgetBox>
      <Stack direction="row" justifyContent="space-between" alignItems="center">
        {!base && icon}

        {trend && !base && (
          <Stack direction="row" gap={0.5} alignItems="center">
            <TrendIcon size={pxToRem(20)} color={colors.green[500]} />
            <Typography variant="body2" fontWeight="medium" sx={{ color: "components.portal.accent.green" }}>
              {trend.value}
            </Typography>
          </Stack>
        )}
      </Stack>

      <Typography variant={small ? "h4" : "h4"} fontWeight="medium" sx={{ mt: 1 }}>
        {value}
      </Typography>

      <Typography variant={small ? "subtitle2" : "subtitle2"} fontWeight="medium" color="text.secondary">
        {label}
      </Typography>
    </WidgetBox>
  );
}
