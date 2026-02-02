import type { ReactNode } from "react";
import { Card, Stack, Typography } from "@wso2/oxygen-ui";

export function SectionCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card component={Stack} p={1.5} gap={1.5}>
      <Typography variant="h5" fontWeight="medium">
        {title}
      </Typography>
      {children}
    </Card>
  );
}
