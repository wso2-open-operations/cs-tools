import type { ReactNode } from "react";
import { Card, Stack, Typography, type CardProps } from "@mui/material";

interface WidgetBoxProps extends Omit<CardProps, "variant"> {
  title?: string;
  children: ReactNode;
}

export function WidgetBox({ title, children, ...props }: WidgetBoxProps) {
  return (
    <Card component={Stack} p={1.2} gap={0.5} elevation={0} {...props}>
      {title && (
        <Stack direction="row" justifyContent="space-between" alignItems="center">
          <Typography variant="h6" fontWeight="medium" color="text.secondary">
            {title}
          </Typography>
        </Stack>
      )}
      {children}
    </Card>
  );
}
