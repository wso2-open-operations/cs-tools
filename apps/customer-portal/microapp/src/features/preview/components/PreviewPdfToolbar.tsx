// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
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
