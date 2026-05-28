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
import { useRef } from "react";

import { Box } from "@wso2/oxygen-ui";
import { type ReactZoomPanPinchRef, TransformComponent, TransformWrapper } from "react-zoom-pan-pinch";

import { PreviewImageToolbar } from "@features/preview/components";

interface ImagePreviewProps {
  src: string;
}

export function ImagePreview({ src }: ImagePreviewProps) {
  const transformRef = useRef<ReactZoomPanPinchRef>(null);

  return (
    <>
      <TransformWrapper
        centerZoomedOut
        centerOnInit
        ref={transformRef}
        initialScale={1}
        minScale={0.5}
        maxScale={5}
        wheel={{ step: 0.2 }}
        doubleClick={{ disabled: true }}
      >
        <Box
          sx={{
            flex: 1,
            overflow: "hidden",
            bgcolor: "background.default",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <TransformComponent
            wrapperStyle={{ width: "100%", height: "100%" }}
            contentStyle={{ width: "fit-content", height: "fit-content" }}
          >
            <Box
              component="img"
              src={src}
              sx={{
                maxWidth: "100%",
                maxHeight: "100%",
                objectFit: "contain",
                userSelect: "none",
                pointerEvents: "none",
              }}
            />
          </TransformComponent>
        </Box>
      </TransformWrapper>

      <PreviewImageToolbar
        zoomIn={() => transformRef.current?.zoomIn()}
        zoomOut={() => transformRef.current?.zoomOut()}
        reset={() => transformRef.current?.resetTransform()}
      />
    </>
  );
}
