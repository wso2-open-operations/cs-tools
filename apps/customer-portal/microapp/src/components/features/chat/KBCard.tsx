import { Article } from "@mui/icons-material";
import { Card, Stack, Typography } from "@mui/material";

export function KBCard({ id, title }: { id: string; title: string }) {
  return (
    <Card
      component={Stack}
      direction="row"
      alignItems="center"
      gap={2}
      p={1}
      elevation={0}
      sx={{ bgcolor: "background.card" }}
    >
      <Article sx={{ color: "components.portal.accent.blue" }} />
      <Stack>
        <Typography variant="subtitle1">{title}</Typography>
        <Typography variant="caption" color="text.secondary">
          {id}
        </Typography>
      </Stack>
    </Card>
  );
}
