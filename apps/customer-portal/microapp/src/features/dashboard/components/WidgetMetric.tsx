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

import { Skeleton, Stack, Typography } from "@wso2/oxygen-ui";

import { WidgetRoot } from "@features/dashboard/components";

export interface WidgetMetricProps {
  label: string;
  value?: number | string;
  icon?: ReactNode;
  variant?: "default" | "minimal";
  onClick?: () => void;
}

export function WidgetMetric({ label, value, icon, variant = "default", onClick }: WidgetMetricProps) {
  return (
    <WidgetRoot onClick={onClick}>
      {variant === "default" && icon && (
        <Stack direction="row" alignItems="center">
          {icon}
        </Stack>
      )}
      <Typography variant="h4" fontWeight="medium" sx={{ mt: 1 }}>
        {value ?? <Skeleton width="60%" animation="wave" />}
      </Typography>
      <Typography variant="subtitle2" fontWeight="medium" color="text.secondary">
        {label}
      </Typography>
    </WidgetRoot>
  );
}
