import { Box, Card, Chip, colors, Divider, pxToRem, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { Calendar, Clock, PhoneCall } from "@wso2/oxygen-ui-icons-react";

import type { CallRequestDto } from "@features/case-types/engagements/types/engagement.dto";

import { useDateTime } from "@shared/hooks";

export function CallRequestItem({ state: { label }, createdOn, durationMin }: CallRequestDto) {
  const { format } = useDateTime();

  return (
    <Card sx={{ p: 1 }}>
      <Stack spacing={1}>
        <Stack direction="row" spacing={2} alignItems="flex-start" justifyContent="space-between">
          <Stack direction="row" spacing={1.5} alignItems="center">
            <PhoneCall color={colors.blue[600]} size={pxToRem(17)} />
            <Box>
              <Typography variant="body1" fontWeight="medium" color="text.primary">
                Call Request
              </Typography>
            </Box>
          </Stack>

          <Chip size="small" label={label} />
        </Stack>

        <Divider />

        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" gap={1} alignItems="center">
            <Box color="text.secondary">
              <Calendar size={pxToRem(13)} />
            </Box>

            <Typography variant="subtitle2" color="text.secondary">
              {format(createdOn)}
            </Typography>
          </Stack>

          <Stack direction="row" gap={0.5} alignItems="center">
            <Box color="text.secondary">
              <Clock size={pxToRem(13)} />
            </Box>

            <Typography variant="subtitle2" color="text.secondary">
              {durationMin} {durationMin === 1 ? "Minute" : "Minutes"}
            </Typography>
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}

export function CallRequestItemSkeleton() {
  return (
    <Card sx={{ p: 1 }}>
      <Stack spacing={1.5}>
        <Stack direction="row" spacing={2} alignItems="flex-start" justifyContent="space-between">
          <Stack direction="row" spacing={1.5} alignItems="center">
            <Skeleton variant="circular" width={18} height={18} />
            <Skeleton variant="text" width={80} height={20} />
          </Stack>
          <Skeleton variant="rounded" width={60} height={24} />
        </Stack>
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Skeleton variant="text" width={80} height={18} />
          <Skeleton variant="text" width={70} height={18} />
        </Stack>
      </Stack>
    </Card>
  );
}
