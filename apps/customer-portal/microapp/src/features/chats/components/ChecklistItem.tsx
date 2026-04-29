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

import { Box, pxToRem, Stack, Typography, type SvgIconProps } from "@wso2/oxygen-ui";
import { Check, CircleCheck } from "@wso2/oxygen-ui-icons-react";
import type { ElementType } from "react";

export function ChecklistItem({
  children,
  variant = "check",
  color = "success",
  icon,
}: {
  children: string;
  variant?: "check" | "checkbox";
  icon?: ElementType<SvgIconProps>;
  color?: SvgIconProps["color"];
}) {
  const Icon = icon ?? (variant === "checkbox" ? CircleCheck : Check);

  return (
    <Stack direction="row" gap={2}>
      <Box sx={(theme) => ({ color: theme.palette[color]?.main ?? theme.palette.primary.main })}>
        <Icon size={pxToRem(18)} />
      </Box>
      <Typography variant="body2">{children}</Typography>
    </Stack>
  );
}
