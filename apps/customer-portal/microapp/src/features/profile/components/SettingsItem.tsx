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

import { Box, Stack } from "@mui/system";
import { Skeleton, type SxProps, Typography } from "@wso2/oxygen-ui";
import { ChevronRight, type LucideIcon } from "@wso2/oxygen-ui-icons-react";

type SettingsItemProps = {
  label: string;
  description?: string;
  value?: string | ReactNode;
  suffix?: "chevron" | ReactNode;
  loading?: boolean;
  slotProps?: {
    icon?: {
      component: LucideIcon;
      sx?: SxProps;
    };
  };

  onClick?: () => void;
};

export function SettingsItem({
  label,
  description,
  value,
  suffix,
  loading = false,
  slotProps,
  onClick,
}: SettingsItemProps) {
  const Icon = slotProps?.icon?.component;

  return (
    <Stack
      onClick={onClick}
      sx={{
        p: 1.5,
        cursor: "pointer",
        textDecoration: "none",
        color: "inherit",
        bgcolor: "background.paper",
        flexDirection: "row",
        alignItems: "center",
        justifyContent: "space-between",
      }}
    >
      <Stack direction="row" alignItems="center" gap={1.5} width="100%">
        {Icon && (
          <Stack
            sx={{
              width: 40,
              height: 40,
              alignItems: "center",
              justifyContent: "center",
              borderRadius: 1,
              ...slotProps?.icon?.sx,
            }}
          >
            <Icon size={18} />
          </Stack>
        )}

        <Stack width="100%">
          {value && (
            <Typography variant="caption" color="text.secondary">
              {label}
            </Typography>
          )}

          <Typography variant="body1" sx={{ flex: 1 }}>
            {loading ? <Skeleton variant="text" width="100%" height={25} /> : (value ?? label)}
          </Typography>

          {description && (
            <Typography variant="caption" fontWeight="regular" color="text.secondary">
              {description}
            </Typography>
          )}
        </Stack>
      </Stack>

      {suffix === "chevron" ? (
        <Box sx={{ flexShrink: 0, color: "text.secondary" }}>
          <ChevronRight size={16} />
        </Box>
      ) : (
        suffix
      )}
    </Stack>
  );
}
