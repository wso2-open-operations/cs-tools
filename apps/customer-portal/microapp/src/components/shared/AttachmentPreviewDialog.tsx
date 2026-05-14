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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

import type { Attachment } from "@root/src/types";
import {
  Box,
  Button,
  ButtonGroup,
  CircularProgress,
  Dialog,
  IconButton,
  pxToRem,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown, ChevronUp, File, Image, Minus, Plus, X } from "@wso2/oxygen-ui-icons-react";
import { TransformWrapper, TransformComponent, type ReactZoomPanPinchRef } from "react-zoom-pan-pinch";
import { Document, Page, pdfjs } from "react-pdf";
import { PDF_JS_DIST_CDN } from "@root/src/config/endpoints";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import "react-pdf/dist/Page/AnnotationLayer.css";
import "react-pdf/dist/Page/TextLayer.css";
import { useQuery } from "@tanstack/react-query";
import { cases } from "@root/src/services/cases";

pdfjs.GlobalWorkerOptions.workerSrc = new URL(PDF_JS_DIST_CDN(pdfjs.version)).toString();

interface PreviewHeaderProps {
  fileName: string | undefined;
  isTypeImage: boolean;
  onClose: () => void;
}

function PreviewHeader({ fileName, isTypeImage, onClose }: PreviewHeaderProps) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: "var(--safe-top)", p: 1 }}>
      <Stack direction="row" alignItems="center" flex={1} gap={1}>
        {isTypeImage ? <Image size={pxToRem(18)} /> : <File size={pxToRem(18)} />}
        <Typography noWrap variant="subtitle1">
          {fileName}
        </Typography>
      </Stack>
      <IconButton size="small" onClick={onClose}>
        <X />
      </IconButton>
    </Stack>
  );
}

function UnsupportedPreview() {
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
        <File size={pxToRem(48)} />
      </Box>
      <Typography variant="body1" lineHeight={0.6}>
        Preview not available for this file type.
      </Typography>
      <Typography variant="caption">Please use the web app to download or view this file.</Typography>
    </Stack>
  );
}

interface ZoomControlsProps {
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

function ZoomControls({ zoomIn, zoomOut, reset }: ZoomControlsProps) {
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

interface ImageToolbarProps {
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

function ImageToolbar({ zoomIn, zoomOut, reset }: ImageToolbarProps) {
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
      <ZoomControls zoomIn={zoomIn} zoomOut={zoomOut} reset={reset} />
    </Stack>
  );
}

interface PdfToolbarProps {
  currentPage: number;
  numberOfPages: number;
  goToPage: (page: number) => void;
  zoomIn: () => void;
  zoomOut: () => void;
  reset: () => void;
}

function PdfToolbar({ currentPage, numberOfPages, goToPage, zoomIn, zoomOut, reset }: PdfToolbarProps) {
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
      <ZoomControls zoomIn={zoomIn} zoomOut={zoomOut} reset={reset} />
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

interface ImagePreviewProps {
  src: string;
}

function ImagePreview({ src }: ImagePreviewProps) {
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

      <ImageToolbar
        zoomIn={() => transformRef.current?.zoomIn()}
        zoomOut={() => transformRef.current?.zoomOut()}
        reset={() => transformRef.current?.resetTransform()}
      />
    </>
  );
}

interface PdfPreviewProps {
  src: string;
}

function PdfPreview({ src }: PdfPreviewProps) {
  const transformRef = useRef<ReactZoomPanPinchRef>(null);
  const pageRefs = useRef<(HTMLDivElement | null)[]>([]);

  const [containerWidth, setContainerWidth] = useState(0);
  const [numberOfPages, setNumberOfPages] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [loading, setLoading] = useState(true);

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
            <Document file={src} onLoadSuccess={handleOnLoadSuccess}>
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

      <PdfToolbar
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

function LoadingFallback() {
  return (
    <Stack flex={1} alignItems="center" justifyContent="center" sx={{ bgcolor: "background.default" }}>
      <CircularProgress size={30} />
    </Stack>
  );
}

interface AttachmentPreviewDialogProps {
  open: boolean;
  attachment: Attachment | null;
  onClose: () => void;
}

export function AttachmentPreviewDialog({ open, attachment, onClose }: AttachmentPreviewDialogProps) {
  const { data, isLoading } = useQuery(cases.attachment(attachment?.id));

  const src = useMemo(() => {
    if (!data) return null;
    const [prefix, base64] = data.content.split(",");
    return `${prefix},${base64}`;
  }, [data]);

  const isTypeImage = attachment?.type === "image";
  const isTypePdf = attachment?.type === "pdf";
  const isTypeUnsupported = attachment?.type === "others";

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullScreen
      fullWidth
      slots={{
        container: Box,
        paper: Box,
      }}
      slotProps={{
        container: {
          sx: {
            bgcolor: "background.paper",
            height: "100dvh",
          },
        },
        paper: {
          sx: {
            bgcolor: "background.paper",
            height: "100%",
            display: "flex",
            flexDirection: "column",
          },
        },
      }}
    >
      <PreviewHeader fileName={attachment?.fileName} isTypeImage={isTypeImage} onClose={onClose} />

      {isLoading ? (
        <LoadingFallback />
      ) : (
        <>
          {isTypeUnsupported && <UnsupportedPreview />}
          {isTypeImage && src && <ImagePreview src={src} />}
          {isTypePdf && src && <PdfPreview src={src ?? "#"} />}
        </>
      )}
    </Dialog>
  );
}
