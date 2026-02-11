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
