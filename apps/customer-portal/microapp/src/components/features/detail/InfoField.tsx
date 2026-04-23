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

import { Stack, Typography, pxToRem } from "@wso2/oxygen-ui";
import type { LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { ReactNode } from "react";

interface InfoFieldProps {
  label: string;
  value: string | ReactNode;
  icon?: LucideIcon;
}

export function InfoField({ label, value, icon }: InfoFieldProps) {
  const Icon = icon;

  return (
    <Stack gap={0.5}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Stack direction="row" alignItems="center" gap={1}>
        {Icon && <Icon size={pxToRem(16)} />}
        <Typography variant="body2" component="div">
          {value}
        </Typography>
      </Stack>
    </Stack>
  );
}
