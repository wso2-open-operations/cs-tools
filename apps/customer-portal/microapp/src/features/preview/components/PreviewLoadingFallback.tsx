import { CircularProgress, Stack } from "@wso2/oxygen-ui";

export function PreviewLoadingFallback() {
  return (
    <Stack flex={1} alignItems="center" justifyContent="center" sx={{ bgcolor: "background.default" }}>
      <CircularProgress size={30} />
    </Stack>
  );
}
