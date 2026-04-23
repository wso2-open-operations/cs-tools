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

import { alpha, Chip, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import type { ProjectCardProps } from "@components/features/projects";
import { Check } from "@wso2/oxygen-ui-icons-react";
import { Circle } from "@mui/icons-material";

import { PROJECT_STATUS_META } from "@config/constants";

export function ProjectPopoverItem({
  name,
  type,
  status,
  numberOfOpenCases,
  active = false,
  onClick,
}: Pick<ProjectCardProps, "name" | "type" | "status" | "numberOfOpenCases"> & {
  active?: boolean;
  onClick: () => void;
}) {
  const theme = useTheme();
  const statusChipColorVariant = PROJECT_STATUS_META[status].color;

  return (
    <Stack
      component="button"
      bgcolor={active ? "background.secondary" : "inherit"}
      sx={{ cursor: "pointer", border: "none" }}
      gap={0.6}
      px={2}
      py={0.5}
      onClick={onClick}
    >
      <Stack direction="row" gap={1}>
        <Typography variant="subtitle1" fontWeight="medium" color="text.primary">
          {name}
        </Typography>
        {active && <Check color={theme.palette.primary.main} />}
      </Stack>
      <Stack direction="row" alignItems="center" gap={1.5}>
        <Chip
          label={status}
          size="small"
          sx={(theme) => ({
            bgcolor: alpha(theme.palette[statusChipColorVariant].light, 0.1),
            color: theme.palette[statusChipColorVariant].light,
          })}
        />
        <Typography color="text.secondary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}>
          {type}
        </Typography>
      </Stack>
      <Stack direction="row" alignItems="center" gap={1}>
        <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(6), color: "primary.main" })} />
        <Typography color="text.secondary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}>
          {numberOfOpenCases} Open Cases
        </Typography>
      </Stack>
    </Stack>
  );
}
