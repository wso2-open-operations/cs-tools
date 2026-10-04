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
import "react-pdf/dist/Page/AnnotationLayer.css";
import "react-pdf/dist/Page/TextLayer.css";
import pdfWorkerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { attachments as attachmentsService } from "@src/services/attachments";
import { getAttachmentPreviewKind } from "@utils/attachmentPreview";
import { Logger } from "@utils/logger";
import type { CaseAttachment } from "@src/types";

// The worker ships with the bundle (same pdfjs-dist version react-pdf resolves), so PDF preview
// does not depend on a third-party CDN at runtime.
pdfjs.GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

// Full-screen zoom/pan image + paginated PDF viewer — mirrors the customer-portal microapp's own
// AttachmentPreviewDialog (components/shared/AttachmentPreviewDialog.tsx) UI, but sources bytes
// the way the webapp does (attachments.getContent -> Blob -> object URL) rather than a JSON
// base64 endpoint, since this backend's content route doesn't have that shape — see
// attachments.ts's getAttachmentContent comment.

interface PreviewHeaderProps {
  fileName: string | undefined;
  isTypeImage: boolean;
  onClose: () => void;
}

function PreviewHeader({ fileName, isTypeImage, onClose }: PreviewHeaderProps) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mt: "var(--safe-top)", p: 1 }}>
      <Stack direction="row" alignItems="center" flex={1} gap={1} sx={{ minWidth: 0 }}>
        {isTypeImage ? <Image size={pxToRem(18)} /> : <File size={pxToRem(18)} />}
        <Typography noWrap variant="subtitle1">
          {fileName}
        </Typography>
      </Stack>
      <IconButton size="small" onClick={onClose} aria-label="Close preview">
        <X />
      </IconButton>
    </Stack>
  );
}

function PreviewError({ message }: { message: string }) {
  return (
    <Stack
      flex={1}
      alignItems="center"
      justifyContent="center"
      gap={1}
      sx={{ bgcolor: "background.default", p: 2, textAlign: "center" }}
    >
      <Typography variant="body2" color="error">
        {message}
      </Typography>
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
      <Typography variant="body1">Preview not available for this file type.</Typography>
      <Typography variant="caption">Use Download instead.</Typography>
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
        <IconButton onClick={zoomOut} aria-label="Zoom out">
          <Minus />
        </IconButton>
        <IconButton onClick={zoomIn} aria-label="Zoom in">
          <Plus />
        </IconButton>
      </ButtonGroup>
      <Button variant="text" color="inherit" onClick={reset}>
        Reset
      </Button>
    </Stack>
  );
}

function ImageToolbar({ zoomIn, zoomOut, reset }: ZoomControlsProps) {
  return (
    <Stack
      direction="row"
      sx={{
        bgcolor: "background.paper",
        height: 50,
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

interface PdfToolbarProps extends ZoomControlsProps {
  currentPage: number;
  numberOfPages: number;
  goToPage: (page: number) => void;
}

function PdfToolbar({ currentPage, numberOfPages, goToPage, zoomIn, zoomOut, reset }: PdfToolbarProps) {
  return (
    <Stack
      direction="row"
      sx={{
        bgcolor: "background.paper",
        height: 50,
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
          <IconButton disabled={currentPage <= 1} onClick={() => goToPage(currentPage - 1)} aria-label="Previous page">
            <ChevronUp />
          </IconButton>
          <IconButton
            disabled={currentPage >= numberOfPages}
            onClick={() => goToPage(currentPage + 1)}
            aria-label="Next page"
          >
            <ChevronDown />
          </IconButton>
        </ButtonGroup>
        {/* Hidden until the page count is known, rather than showing "1 / 0" while the PDF is
            still loading. aria-live announces page changes as the user scrolls/paginates. */}
        {numberOfPages > 0 && (
          <Typography sx={{ minWidth: 60, textAlign: "center" }} aria-live="polite">
            {currentPage} / {numberOfPages}
          </Typography>
        )}
      </Stack>
    </Stack>
  );
}

function ImagePreview({ src, alt, onError }: { src: string; alt: string; onError: () => void }) {
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
              alt={alt}
              onError={onError}
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

function PdfPreview({ src }: { src: string }) {
  const transformRef = useRef<ReactZoomPanPinchRef>(null);
  const pageRefs = useRef<(HTMLDivElement | null)[]>([]);

  const [containerWidth, setContainerWidth] = useState(0);
  const [numberOfPages, setNumberOfPages] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
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

  if (isError) return <PreviewError message="Could not render this PDF." />;

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
            <Document file={src} onLoadSuccess={handleOnLoadSuccess} onLoadError={() => setIsError(true)}>
              <Stack gap={0.5}>
                {/* Don't render pages until the container's real width is measured — Document's
                    onLoadSuccess can fire before the ResizeObserver's first callback, and a Page
                    given width={0} renders collapsed/broken until the next re-render. */}
                {containerWidth > 0 &&
                  Array.from({ length: numberOfPages }, (_, i) => (
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

          {(numberOfPages === 0 || containerWidth === 0) && (
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
  /** Attachment being previewed; the dialog is closed when this is null. */
  attachment: CaseAttachment | null;
  onClose: () => void;
}

export function AttachmentPreviewDialog({ attachment, onClose }: AttachmentPreviewDialogProps) {
  const [objectUrl, setObjectUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Reconcile state *synchronously during render* the moment `attachment` changes (React's
  // "adjusting state when a prop changes" pattern) — resetting from a useEffect instead leaves a
  // frame where the dialog paints the *previous* attachment's stale objectUrl/error before the
  // effect fires, a visible flash of the wrong preview.
  const [renderedFor, setRenderedFor] = useState(attachment);
  if (attachment !== renderedFor) {
    setRenderedFor(attachment);
    setObjectUrl(null);
    setError(null);
  }

  const kind = attachment ? getAttachmentPreviewKind(attachment.type) : null;

  useEffect(() => {
    // Both callers only ever open this dialog for a previewable kind (see AttachmentsTab.tsx /
    // CaseActivityFeed.tsx's own getAttachmentPreviewKind-gated Preview button), but guard here
    // too rather than trust that invariant — fetching content that can only ever render
    // UnsupportedPreview would be a wasted request.
    if (!attachment || !kind) return;

    let cancelled = false;
    let createdUrl: string | null = null;

    attachmentsService
      .getContent(attachment)
      .then((blob) => {
        if (cancelled) return;
        createdUrl = URL.createObjectURL(blob);
        setObjectUrl(createdUrl);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        Logger.error(`Failed to load attachment preview for ${attachment.id}`, err);
        setError("Could not load the preview.");
      });

    return () => {
      cancelled = true;
      if (createdUrl) URL.revokeObjectURL(createdUrl);
    };
  }, [attachment, kind]);

  const open = !!attachment;
  const loading = open && !objectUrl && !error;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullScreen
      fullWidth
      slots={{ container: Box, paper: Box }}
      slotProps={{
        container: { sx: { bgcolor: "background.paper", height: "100dvh" } },
        paper: { sx: { bgcolor: "background.paper", height: "100%", display: "flex", flexDirection: "column" } },
      }}
      aria-label={attachment ? `Preview ${attachment.name}` : "Preview"}
    >
      <PreviewHeader fileName={attachment?.name} isTypeImage={kind === "image"} onClose={onClose} />

      {loading ? (
        <LoadingFallback />
      ) : error ? (
        <PreviewError message={error} />
      ) : (
        <>
          {kind === null && <UnsupportedPreview />}
          {kind === "image" && objectUrl && (
            <ImagePreview
              src={objectUrl}
              alt={attachment?.name ?? "Attachment preview"}
              onError={() => setError("Could not display this image.")}
            />
          )}
          {kind === "pdf" && objectUrl && <PdfPreview src={objectUrl} />}
        </>
      )}
    </Dialog>
  );
}
