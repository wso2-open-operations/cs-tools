import { Box, Card, Stack, Typography } from "@wso2/oxygen-ui";
import { Clock4 } from "@wso2/oxygen-ui-icons-react";

export function InvitationExpiryCallout() {
  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="center"
      px={2}
      py={1.5}
      gap={2}
      sx={{ bgcolor: "components.popover.state.active.background" }}
    >
      <Box sx={{ color: "primary.main" }}>
        <Clock4 size={50} />
      </Box>
      <Typography variant="subtitle2" fontWeight="medium" color="text.secondary">
        Important: &nbsp;
        <Typography component="span" variant="subtitle2" fontWeight="regular">
          Invitation links expire after 7 days. If the user doesn't accept the invitation within this timeframe, you'll
          need to send a new invitation.
        </Typography>
      </Typography>
    </Card>
  );
}
