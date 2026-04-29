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
import { pxToRem, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { ChevronRight, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import { Link } from "react-router-dom";

export function SettingListItem({
  name,
  value,
  icon,
  iconColor,
  iconBackgroundColor,
  description,
  suffix,
  to,
  onClick,
}: {
  name: string;
  icon: LucideIcon;
  iconColor?: string;
  iconBackgroundColor?: string;
  value?: string | ReactNode;
  description?: string;
  suffix?: "chevron" | ReactNode;
  to?: string;
  onClick?: () => void;
}) {
  const theme = useTheme();
  const Icon = icon;

  return (
    <Stack
      direction="row"
      justifyContent="space-between"
      alignItems="center"
      bgcolor="background.paper"
      sx={{ cursor: "pointer", textDecoration: "none", color: "inherit" }}
      p={1.5}
      onClick={onClick}
      {...(to && { component: Link, to })}
    >
      <Stack direction="row" alignItems="center" gap={1.5} width="100%">
        <Stack
          width={40}
          height={40}
          alignItems="center"
          justifyContent="center"
          borderRadius={1}
          bgcolor={iconBackgroundColor}
        >
          <Icon size={pxToRem(18)} color={iconColor} />
        </Stack>
        <Stack width="100%">
          {value && (
            <Typography variant="caption" color="text.secondary">
              {name}
            </Typography>
          )}

          <Typography variant="body1" sx={{ flex: 1 }}>
            {value ?? name}
          </Typography>

          {description && (
            <Typography variant="caption" fontWeight="regular" color="text.secondary">
              {description}
            </Typography>
          )}
        </Stack>
      </Stack>
      {suffix && suffix === "chevron" ? (
        <ChevronRight size={pxToRem(16)} color={theme.palette.text.secondary} style={{ flexShrink: 0 }} />
      ) : (
        suffix
      )}
    </Stack>
  );
}
