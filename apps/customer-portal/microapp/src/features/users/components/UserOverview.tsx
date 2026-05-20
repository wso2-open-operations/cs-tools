import { Box, Card, Stack, Typography } from "@wso2/oxygen-ui";
import { Mail } from "@wso2/oxygen-ui-icons-react";

import { UserAvatar } from "@features/users/components";
import { useMode } from "@features/users/hooks";

export function UserOverview() {
  const { initial } = useMode();
  if (!initial) return;

  const { email, firstName, lastName } = initial;

  return (
    <Card component={Stack} textAlign="center" alignItems="center" gap={1} p={3}>
      <UserAvatar>{initial.firstName}</UserAvatar>
      <Stack textAlign="center" gap={0.5}>
        <Typography variant="h5" fontWeight="medium">
          {firstName + " " + lastName}
        </Typography>

        <Stack direction="row" justifyContent="center" alignItems="center" gap={1}>
          <Box sx={{ color: "text.secondary" }}>
            <Mail size={16} />
          </Box>
          <Typography variant="body2" fontWeight="regular" color="text.secondary">
            {email}
          </Typography>
        </Stack>
      </Stack>
    </Card>
  );
}
