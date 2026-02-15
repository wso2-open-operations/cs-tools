import type { ReactNode } from "react";
import { Card, Stack, Typography } from "@wso2/oxygen-ui";

export function SectionCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Card component={Stack} p={1.5} gap={1.5} sx={{ bgcolor: "background.paper" }}>
      <Typography variant="h6" fontWeight="medium">
        {title}
      </Typography>
      {children}
    </Card>
  );
}
