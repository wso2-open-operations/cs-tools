import { Box, Popover, Stack, Typography, type PopoverProps } from "@mui/material";
import { ProjectPopoverItem } from "@features/projects";

export function ProjectSelector({ open, anchorEl, onClose }: PopoverProps) {
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
      elevation={0}
      slotProps={{
        paper: {
          sx: (theme) => ({
            py: 2,
            width: "100%",
            border: `1px solid ${theme.palette.divider}`,
            borderRadius: 3,
          }),
        },
      }}
    >
      <Typography
        color="text.secondary"
        fontWeight="medium"
        sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}
        px={2}
      >
        Select Project
      </Typography>
      <Stack gap={1} pt={1}>
        <ProjectPopoverItem name="Dreamworks Inc" type="Managed Cloud" status="All Good" numberOfOpenCases={3} active />
        <ProjectPopoverItem name="Newsline Enterprise" type="Regular" status="All Good" numberOfOpenCases={1} />
        <ProjectPopoverItem
          name="Goods Store Mart"
          type="Managed Cloud"
          status="Needs Attention"
          numberOfOpenCases={5}
        />
      </Stack>
    </Popover>
  );
}
