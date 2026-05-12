import { useQuery } from "@tanstack/react-query";
import { Box, Button, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
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
      <Button sx={{ justifyContent: "space-between", p: 0, mt: 2 }} onClick={open} disableRipple>
        <Stack direction="row" sx={{ alignItems: "center", flexGrow: 1, minWidth: 0, gap: 1 }}>
          <Box color="text.secondary">
            <Folder size={pxToRem(18)} />
          </Box>
          <Typography variant="body1" color="text.secondary" sx={{ textTransform: "initial" }} noWrap>
            {project.name}
          </Typography>
        </Stack>
        <Box color="text.secondary">
          <ChevronDown size={pxToRem(18)} />
        </Box>
      </Button>

      {/* Popover */}
      <ProjectSelector anchorEl={anchor} open={isOpen} onClose={close} />
    </>
  );
}
