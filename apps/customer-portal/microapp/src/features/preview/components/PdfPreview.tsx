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
import { useCallback, useEffect, useRef, useState } from "react";

import { Box, Skeleton, Stack } from "@wso2/oxygen-ui";
import { Document, Page, pdfjs } from "react-pdf";
import "react-pdf/dist/Page/AnnotationLayer.css";
import "react-pdf/dist/Page/TextLayer.css";
import { type ReactZoomPanPinchRef, TransformComponent, TransformWrapper } from "react-zoom-pan-pinch";

import { PreviewErrorFallback, PreviewPdfToolbar } from "@features/preview/components";

import { PDF_JS_DIST_CDN } from "@shared/constants";

pdfjs.GlobalWorkerOptions.workerSrc = new URL(PDF_JS_DIST_CDN(pdfjs.version)).toString();

interface PdfPreviewProps {
  src: string;
}

export function PdfPreview({ src }: PdfPreviewProps) {
  const transformRef = useRef<ReactZoomPanPinchRef>(null);
  const pageRefs = useRef<(HTMLDivElement | null)[]>([]);

  const [containerWidth, setContainerWidth] = useState(0);
  const [numberOfPages, setNumberOfPages] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [isError, setIsError] = useState(false);

  const containerCallbackRef = useCallback((node: HTMLDivElement | null) => {
    if (!node) return;
    const observer = new ResizeObserver(([entry]) => {
      const width = entry.contentRect.width;
      if (width > 0) setContainerWidth(width);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const handleOnLoadSuccess = ({ numPages }: { numPages: number }) => {
    setNumberOfPages(numPages);
    setTimeout(() => transformRef.current?.resetTransform(0), 100);
    setLoading(false);
  };

  const handleOnLoadError = () => {
    setLoading(false);
    setIsError(true);
  };

  const goToPage = (page: number) => {
    const clamped = Math.min(Math.max(page, 1), numberOfPages);
    setCurrentPage(clamped);
    pageRefs.current[clamped - 1]?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    if (!numberOfPages) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.find((e) => e.isIntersecting);
        if (visible) {
          const index = pageRefs.current.indexOf(visible.target as HTMLDivElement);
          if (index !== -1) setCurrentPage(index + 1);
        }
      },
      { threshold: 0.5 },
    );

    pageRefs.current.forEach((page) => page && observer.observe(page));
    return () => observer.disconnect();
  }, [numberOfPages]);

  if (isError) return <PreviewErrorFallback />;

  return (
    <>
      <TransformWrapper
        centerZoomedOut
        ref={transformRef}
        initialScale={1}
        minScale={0.5}
        maxScale={5}
        wheel={{ step: 0.2 }}
        doubleClick={{ disabled: true }}
      >
        <Box
          ref={containerCallbackRef}
          sx={{
            flex: 1,
            overflow: "hidden",
            bgcolor: "background.default",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            position: "relative",
          }}
        >
          <TransformComponent
            wrapperStyle={{ width: "100%", height: "100%" }}
            contentStyle={{ width: "fit-content", height: "fit-content" }}
          >
            <Document file={src} onLoadSuccess={handleOnLoadSuccess} onLoadError={handleOnLoadError}>
              <Stack gap={0.5}>
                {Array.from({ length: numberOfPages }, (_, i) => (
                  <Box
                    key={i}
                    ref={(el) => {
                      pageRefs.current[i] = el as HTMLDivElement;
                    }}
                  >
                    <Page width={containerWidth} pageNumber={i + 1} />
                  </Box>
                ))}
              </Stack>
            </Document>
          </TransformComponent>

          {loading && (
            <Box sx={{ width: "100%", height: "100%", position: "absolute", bgcolor: "background.default" }}>
              <Skeleton variant="rectangular" width="100%" height="100%" />
            </Box>
          )}
        </Box>
      </TransformWrapper>

      <PreviewPdfToolbar
        currentPage={currentPage}
        numberOfPages={numberOfPages}
        goToPage={goToPage}
        zoomIn={() => transformRef.current?.zoomIn()}
        zoomOut={() => transformRef.current?.zoomOut()}
        reset={() => transformRef.current?.resetTransform()}
      />
    </>
  );
}
