import { Check, CheckBox } from "@mui/icons-material";
import { Stack, Typography } from "@mui/material";

export function ChecklistItem({ children, variant = "check" }: { children: string; variant?: "check" | "checkbox" }) {
  const Icon = variant === "checkbox" ? CheckBox : Check;

  return (
    <Stack direction="row" gap={2}>
      <Icon color="success" />
      <Typography variant="body2">{children}</Typography>
    </Stack>
  );
}
