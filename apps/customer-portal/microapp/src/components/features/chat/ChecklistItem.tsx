import { Check } from "@mui/icons-material";
import { Stack, Typography } from "@mui/material";

export function ChecklistItem({ children }: { children: string }) {
  return (
    <Stack direction="row" gap={2}>
      <Check color="success" />
      <Typography variant="body2">{children}</Typography>
    </Stack>
  );
}
