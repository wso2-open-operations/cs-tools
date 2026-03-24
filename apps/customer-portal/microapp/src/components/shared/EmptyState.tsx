import { Box, Stack, Typography } from "@wso2/oxygen-ui";
import { Inbox } from "@wso2/oxygen-ui-icons-react";

interface EmptyStateProps {
  title?: string;
  description?: string;
  icon?: React.ReactNode;
}

export default function EmptyState({
  title = "No items found",
  description = "There are no items to display at the moment.",
  icon = <Inbox size={48} />,
}: EmptyStateProps) {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        py: 8,
        px: 2,
        textAlign: "center",
        borderRadius: 2,
      }}
    >
      <Stack spacing={2} alignItems="center">
        <Box
          sx={{
            color: "text.secondary",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            mb: 1,
            opacity: 0.6,
          }}
        >
          {icon}
        </Box>

        <Box>
          <Typography variant="h6" fontWeight="600" gutterBottom>
            {title}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 300 }}>
            {description}
          </Typography>
        </Box>
      </Stack>
    </Box>
  );
}
