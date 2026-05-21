import { alpha, Card, Stack, Typography } from "@wso2/oxygen-ui";

export function ProfileEditCallout() {
  return (
    <Card
      component={Stack}
      direction="row"
      gap={2}
      sx={(theme) => ({ bgcolor: alpha(theme.palette.info.main, 0.2), p: 1.5 })}
    >
      <Typography variant="subtitle2" color="text.secondary">
        Keep your contact information up to date for better communication with our support team.
      </Typography>
    </Card>
  );
}
