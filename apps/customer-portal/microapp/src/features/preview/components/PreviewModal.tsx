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
