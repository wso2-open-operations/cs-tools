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
