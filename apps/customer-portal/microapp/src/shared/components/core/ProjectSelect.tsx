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
import { useQuery } from "@tanstack/react-query";
import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronDown, Folder } from "@wso2/oxygen-ui-icons-react";

import { useProject } from "@context/project";

import { projects } from "@features/projects/api/projects.queries";
import { ProjectSelector } from "@features/projects/components";
import { usePopoverAnchor } from "@features/projects/hooks";

export function ProjectSelect() {
  const { projectId } = useProject();
  const { anchor, isOpen, open, close } = usePopoverAnchor();

  const project = useQuery(projects.all()).data?.find((project) => project.id === projectId); // TODO: CHECK THIS AGAIN AFTER REFACTORING EVERYTHING

  if (!project) return null;

  return (
    <>
      <Button sx={{ justifyContent: "space-between", px: 1 }} onClick={open} disableRipple>
        <Stack direction="row" sx={{ flexGrow: 1, minWidth: 0, gap: 1 }}>
          <Box color="text.secondary">
            <Folder size={18} />
          </Box>
          <Typography variant="body1" color="text.secondary" sx={{ textTransform: "initial" }} noWrap>
            {project.name}
          </Typography>
        </Stack>
        <Box color="text.secondary">
          <ChevronDown size={18} />
        </Box>
      </Button>

      {/* Popover */}
      <ProjectSelector anchorEl={anchor} open={isOpen} onClose={close} />
    </>
  );
}
