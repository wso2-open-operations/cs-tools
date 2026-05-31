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

import { Box, Skeleton } from "@wso2/oxygen-ui";
import type { SxProps, Theme } from "@wso2/oxygen-ui";
import type { JSX, KeyboardEvent } from "react";
import { useAttachmentPreview } from "@api/useAttachmentPreview";

interface AuthenticatedImageProps {
  attachmentId: string | null | undefined;
  alt?: string;
  sx?: SxProps<Theme>;
  onClick?: () => void;
}

/**
 * Fetches an attachment via the authenticated backend and displays it as an image.
 * Shows a skeleton while loading and nothing on error.
 *
 * @param {AuthenticatedImageProps} props - Component props.
 * @returns {JSX.Element} The image or skeleton.
 */
export default function AuthenticatedImage({
  attachmentId,
  alt = "",
  sx,
  onClick,
}: AuthenticatedImageProps): JSX.Element {
  const { data: dataUrl, isLoading } = useAttachmentPreview(attachmentId);

  if (isLoading) {
    return (
      <Skeleton
        variant="rectangular"
        sx={{ maxWidth: "100%", maxHeight: 200, width: 200, height: 150, ...sx }}
      />
    );
  }

  if (!dataUrl) return <></>;

  const interactiveProps = onClick
    ? {
        role: "button" as const,
        tabIndex: 0,
        onKeyDown: (e: KeyboardEvent) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onClick();
          }
        },
      }
    : {};

  return (
    <Box
      component="img"
      src={dataUrl}
      alt={alt}
      loading="lazy"
      onClick={onClick}
      {...interactiveProps}
      sx={{
        display: "block",
        maxWidth: "100%",
        maxHeight: 200,
        width: "auto",
        height: "auto",
        objectFit: "contain",
        cursor: onClick ? "pointer" : "default",
        ...sx,
      }}
    />
  );
}
