import { Box, Card, Stack, Typography, pxToRem, colors } from "@wso2/oxygen-ui";
import { BookOpen } from "@wso2/oxygen-ui-icons-react";

export function KBCard({ id, title }: { id: string; title: string }) {
  return (
    <Card component={Stack} direction="row" alignItems="center" gap={2} p={1} sx={{ bgcolor: "background.default" }}>
      <Box color={colors.blue[500]}>
        <BookOpen size={pxToRem(18)} />
      </Box>
      <Stack>
        <Typography variant="body2">{title}</Typography>
        <Typography variant="caption" color="text.secondary">
          {id}
        </Typography>
      </Stack>
    </Card>
  );
}
