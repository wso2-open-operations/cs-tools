import { Stack } from "@wso2/oxygen-ui";

import { ErrorState } from "@shared/components/common";

export function PreviewErrorFallback() {
  return (
    <Stack flex={1} alignItems="center" justifyContent="center" sx={{ bgcolor: "background.default" }}>
      <ErrorState />
    </Stack>
  );
}
