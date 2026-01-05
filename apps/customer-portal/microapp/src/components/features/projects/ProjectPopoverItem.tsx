import type { ProjectCardProps } from "@root/src/components/features/projects";
import { Chip, Stack, Typography } from "@mui/material";
import { Check, Circle } from "@mui/icons-material";

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
  const StatusChipIcon = PROJECT_STATUS_META[status].icon;
  const statusChipColorVariant = PROJECT_STATUS_META[status].color;

  return (
    <Stack
      component="button"
      bgcolor={active ? "components.popover.state.active.background" : "inherit"}
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
        {active && <Check color="primary" />}
      </Stack>
      <Stack direction="row" alignItems="center" gap={1.5}>
        <Chip label={status} size="small" color={statusChipColorVariant} iconPosition="end" icon={<StatusChipIcon />} />
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
