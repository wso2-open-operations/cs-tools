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

import { Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { Check } from "@wso2/oxygen-ui-icons-react";
import type { Project } from "@src/types";

type ProjectPopoverItemProps = Pick<Project, "name" | "metrics"> & {
  active?: boolean;
  onClick: () => void;
};

export function ProjectPopoverItem({ name, active = false, onClick }: ProjectPopoverItemProps) {
  const theme = useTheme();

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
      <Stack direction="row" justifyContent="space-between" alignItems="center" width="100%" gap={1}>
        <Typography variant="subtitle1" fontWeight="medium" color="text.primary" textAlign="left">
          {name}
        </Typography>
        {active && <Check style={{ flexShrink: 0 }} color={theme.palette.primary.main} />}
      </Stack>
    </Stack>
  );
}
