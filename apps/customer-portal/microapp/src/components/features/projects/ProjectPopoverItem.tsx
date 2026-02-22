import { alpha, Chip, Skeleton, Stack, Typography, useTheme } from "@wso2/oxygen-ui";
import { Check } from "@wso2/oxygen-ui-icons-react";
import { Circle } from "@mui/icons-material";
import type { Project } from "@src/types";

import { PROJECT_STATUS_META } from "@config/constants";

export function ProjectPopoverItem({
  name,
  type,
  status,
  metrics,
  active = false,
  onClick,
}: Pick<Project, "name" | "type" | "status" | "metrics"> & {
  active?: boolean;
  onClick: () => void;
}) {
  const theme = useTheme();
  const statusChipColorVariant = status ? PROJECT_STATUS_META[status].color : "default";

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
        {status ? (
          <Chip
            label={status}
            size="small"
            sx={(theme) => ({
              bgcolor: alpha(theme.palette[statusChipColorVariant].light, 0.1),
              color: theme.palette[statusChipColorVariant].light,
            })}
          />
        ) : (
          <Skeleton variant="rounded" width={80} height={24} />
        )}
        <Typography color="text.secondary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}>
          {type}
        </Typography>
      </Stack>
      <Stack direction="row" alignItems="center" gap={1}>
        <Circle sx={(theme) => ({ fontSize: theme.typography.pxToRem(6), color: "primary.main" })} />
        <Typography color="text.secondary" sx={(theme) => ({ fontSize: theme.typography.pxToRem(13) })}>
          {metrics.cases ?? 0} Open Cases
        </Typography>
      </Stack>
    </Stack>
  );
}
