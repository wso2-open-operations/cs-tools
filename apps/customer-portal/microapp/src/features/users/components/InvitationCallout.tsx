import { alpha, Box, Card, Stack, Typography } from "@wso2/oxygen-ui";
import { Info } from "@wso2/oxygen-ui-icons-react";

export function InvitationCallout() {
  return (
    <Card
      component={Stack}
      direction="row"
      sx={(theme) => ({ bgcolor: alpha(theme.palette.info.main, 0.2), p: 1.5, gap: 2 })}
    >
      <Box sx={{ color: "info.main" }}>
        <Info size={18} />
      </Box>
      <Stack>
        <Typography variant="body2" fontWeight="medium" color="info">
          Direct User Invitation
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          Send an email invitation directly to a user to join this project. The invitation link will be valid for 7
          days.
        </Typography>
      </Stack>
    </Card>
  );
}
