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
