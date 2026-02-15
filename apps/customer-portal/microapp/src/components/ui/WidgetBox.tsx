import type { ReactNode } from "react";
import { Card, Stack, Typography, type CardProps } from "@wso2/oxygen-ui";

interface WidgetBoxProps extends Omit<CardProps, "variant"> {
  title?: string;
  children: ReactNode;
}

export function WidgetBox({ title, children, ...props }: WidgetBoxProps) {
  return (
    <Card component={Stack} p={1.2} gap={0.5} sx={{ height: "100%", bgcolor: "background.paper" }} {...props}>
      {title && (
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Typography variant="body1" fontWeight="medium" color="text.primary">
            {title}
          </Typography>
        </Stack>
      )}
      {children}
    </Card>
  );
}
