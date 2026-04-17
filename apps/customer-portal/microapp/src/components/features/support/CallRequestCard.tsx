import type { CallRequestDto } from "@root/src/types/engagement.dto";
import { Card, Chip, colors, pxToRem, Stack, Typography, Box, Divider } from "@wso2/oxygen-ui";
import { Calendar, Clock, PhoneCall } from "@wso2/oxygen-ui-icons-react";

export function CallRequestCard(props: CallRequestDto) {
  return (
    <Card
      sx={{
        p: 1,
      }}
    >
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
          <Chip size="small" label={props.state.label} />
        </Stack>
        <Divider />
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Stack direction="row" gap={1} alignItems="center">
            <Box color="text.secondary">
              <Calendar size={pxToRem(13)} />
            </Box>
            <Typography variant="subtitle2" color="text.secondary">
              {props.createdOn}
            </Typography>
          </Stack>

          <Stack direction="row" gap={0.5} alignItems="center">
            <Box color="text.secondary">
              <Clock size={pxToRem(13)} />
            </Box>
            <Typography variant="subtitle2" color="text.secondary">
              {props.durationMin} Minutes
            </Typography>
          </Stack>
        </Stack>
      </Stack>
    </Card>
  );
}
