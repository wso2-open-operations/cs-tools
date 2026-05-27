import type { ReactNode } from "react";

import { Card, Divider, Stack, Typography } from "@wso2/oxygen-ui";

export function SettingsSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Stack gap={1}>
      <Typography variant="subtitle1" color="text.secondary">
        {title}
      </Typography>
      <Card component={Stack} elevation={0} divider={<Divider />}>
        {children}
      </Card>
    </Stack>
  );
}
