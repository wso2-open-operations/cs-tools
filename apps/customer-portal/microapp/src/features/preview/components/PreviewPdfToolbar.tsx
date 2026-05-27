import { ButtonGroup, IconButton, Stack, Typography } from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp } from "@wso2/oxygen-ui-icons-react";

import { PreviewZoomControls } from "@features/preview/components";

interface PdfToolbarProps {
  currentPage: number;
  numberOfPages: number;
  goToPage: (page: number) => void;
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

export function PreviewPdfToolbar({ currentPage, numberOfPages, goToPage, zoomIn, zoomOut, reset }: PdfToolbarProps) {
  return (
    <Stack
      direction="row"
      sx={{
        bgcolor: "background.paper",
        height: "50px",
        mb: "var(--safe-bottom)",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 2,
        px: 1,
      }}
    >
      <PreviewZoomControls zoomIn={zoomIn} zoomOut={zoomOut} reset={reset} />
      <Stack direction="row" alignItems="center" gap={1}>
        <ButtonGroup variant="contained" color="inherit">
          <IconButton disabled={currentPage <= 1} onClick={() => goToPage(currentPage - 1)}>
            <ChevronUp />
          </IconButton>
          <IconButton disabled={currentPage >= numberOfPages} onClick={() => goToPage(currentPage + 1)}>
            <ChevronDown />
          </IconButton>
        </ButtonGroup>
        <Typography sx={{ minWidth: 60, textAlign: "center" }}>
          {currentPage} / {numberOfPages}
        </Typography>
      </Stack>
    </Stack>
  );
}
