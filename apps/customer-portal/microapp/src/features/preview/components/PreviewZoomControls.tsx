import { Button, ButtonGroup, IconButton, Stack } from "@wso2/oxygen-ui";
import { Minus, Plus } from "@wso2/oxygen-ui-icons-react";

interface ZoomControlsProps {
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

export function PreviewZoomControls({ zoomIn, zoomOut, reset }: ZoomControlsProps) {
  return (
    <Stack direction="row" alignItems="center" gap={1}>
      <ButtonGroup variant="contained" color="inherit">
        <IconButton onClick={zoomOut}>
          <Minus />
        </IconButton>
        <IconButton onClick={zoomIn}>
          <Plus />
        </IconButton>
      </ButtonGroup>
      <Button variant="text" color="inherit" onClick={reset}>
        Reset
      </Button>
    </Stack>
  );
}
