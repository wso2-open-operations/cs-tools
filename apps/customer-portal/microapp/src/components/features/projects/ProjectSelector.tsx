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

import { alpha, Box, Popover, pxToRem, Stack, Typography, type PopoverProps } from "@wso2/oxygen-ui";
import { ProjectPopoverItem } from "@components/features/projects";
import { useProject } from "@context/project";
import { MOCK_PROJECTS } from "@src/mocks/data/projects";

export function ProjectSelector({ open, anchorEl, onClose }: PopoverProps) {
  const { projectId, setProjectId } = useProject();

  return (
    <Popover
      component={Box}
      open={open}
      anchorEl={anchorEl}
      onClose={onClose}
      transformOrigin={{
        vertical: "center",
        horizontal: "center",
      }}
      anchorOrigin={{
        vertical: "bottom",
        horizontal: "center",
      }}
      slotProps={{
        paper: {
          sx: (theme) => ({
            py: 2,
            width: "100%",
            border: `1px solid ${theme.palette.divider}`,
            borderRadius: 3,
            boxShadow: `${alpha(theme.palette.text.primary, 0.3)} 0px 48px 100px 0px`,
          }),
        },
      }}
    >
      <Typography color="text.secondary" fontWeight="medium" sx={{ fontSize: pxToRem(13) }} px={2}>
        Select Project
      </Typography>
      <Stack gap={1} pt={1}>
        {MOCK_PROJECTS.map((props) => (
          <ProjectPopoverItem
            {...props}
            key={props.id}
            active={props.id === projectId}
            onClick={() => {
              setProjectId(props.id);
              onClose?.({}, "backdropClick");
            }}
          />
        ))}
      </Stack>
    </Popover>
  );
}
