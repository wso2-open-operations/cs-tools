import { Box, Card, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
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
