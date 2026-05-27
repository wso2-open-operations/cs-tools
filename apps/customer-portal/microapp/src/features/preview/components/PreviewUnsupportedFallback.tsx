import { Box, Stack, Typography } from "@wso2/oxygen-ui";
import { File } from "@wso2/oxygen-ui-icons-react";

export function PreviewUnsupportedFallback() {
  return (
    <Stack
      alignItems="center"
      gap={1}
      sx={{
        flex: 1,
        justifyContent: "center",
        color: "text.secondary",
        bgcolor: "background.default",
        textAlign: "center",
        p: 2,
      }}
    >
      <Box mb={1}>
        <File size={48} />
      </Box>
      <Typography variant="body1" lineHeight={0.6}>
        Preview not available for this file type.
      </Typography>
      <Typography variant="caption">Please use the web app to download or view this file.</Typography>
    </Stack>
  );
}
