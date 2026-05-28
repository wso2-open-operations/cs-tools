import { useMemo } from "react";

import { useQuery } from "@tanstack/react-query";
import { Box, Dialog } from "@wso2/oxygen-ui";

import { cases } from "@features/case-types/cases/api/cases.queries";
import type { Attachment } from "@features/case-types/cases/types";
import {
  ImagePreview,
  PdfPreview,
  PreviewErrorFallback,
  PreviewHeader,
  PreviewLoadingFallback,
  PreviewUnsupportedFallback,
} from "@features/preview/components";

interface PreviewModalProps {
  open: boolean;
  attachment: Attachment | null;
  onClose: () => void;
}

export function PreviewModal({ open, attachment, onClose }: PreviewModalProps) {
  const { data, isLoading, isError } = useQuery(cases.attachment(attachment?.id));

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
        <PreviewLoadingFallback />
      ) : isError ? (
        <PreviewErrorFallback />
      ) : (
        <>
          {isTypeUnsupported && <PreviewUnsupportedFallback />}
          {isTypeImage && src && <ImagePreview src={src} />}
          {isTypePdf && src && <PdfPreview src={src} />}
        </>
      )}
    </Dialog>
  );
}
