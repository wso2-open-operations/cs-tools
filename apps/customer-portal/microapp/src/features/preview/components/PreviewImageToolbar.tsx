import { Stack } from "@wso2/oxygen-ui";

import { PreviewZoomControls } from "@features/preview/components";

interface ImageToolbarProps {
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

export function PreviewImageToolbar({ zoomIn, zoomOut, reset }: ImageToolbarProps) {
  return (
    <Stack
      direction="row"
      sx={{
        bgcolor: "background.paper",
        height: "50px",
        mb: "var(--safe-bottom)",
        alignItems: "center",
        justifyContent: "center",
        gap: 2,
      }}
    >
      <PreviewZoomControls zoomIn={zoomIn} zoomOut={zoomOut} reset={reset} />
    </Stack>
  );
}
