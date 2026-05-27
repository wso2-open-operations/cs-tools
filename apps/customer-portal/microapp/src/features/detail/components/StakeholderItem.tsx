import { Box, Card, pxToRem, Skeleton, Stack, Typography } from "@wso2/oxygen-ui";
import { User } from "@wso2/oxygen-ui-icons-react";

export function StakeholderItem({ name, role }: { name: string; role: string }) {
  return (
    <Card sx={{ bgcolor: "background.default" }} component={Stack} direction="row" justifyContent="space-between" p={1}>
      <Stack direction="row" gap={1}>
        <Box color="text.secondary">
          <User size={pxToRem(18)} />
        </Box>

        <Typography variant="body1" fontWeight="medium">
          {name}
        </Typography>
      </Stack>

      <Typography variant="body2" fontWeight="regular" color="text.secondary">
        {role}
      </Typography>
    </Card>
  );
}

export function StakeholderItemSkeleton() {
  return (
    <Card
      sx={{ bgcolor: "background.default" }}
      component={Stack}
      direction="row"
      justifyContent="space-between"
      alignItems="center"
      p={1}
    >
      <Stack direction="row" gap={1} alignItems="center">
        <Box color="text.secondary">
          <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
        </Box>

        <Skeleton variant="text" width={pxToRem(100)} height={pxToRem(20)} />
      </Stack>

      <Skeleton variant="text" width={pxToRem(60)} height={pxToRem(16)} />
    </Card>
  );
}
