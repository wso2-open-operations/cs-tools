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

import type { ImageFullscreenModalProps } from "@features/support/types/supportComponents";
import { Box, Dialog, IconButton } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

/**
 * Fullscreen modal to display an image with close button.
 *
 * @param {ImageFullscreenModalProps} props - open, imageSrc, onClose.
 * @returns {JSX.Element} The fullscreen image modal.
 */
export default function ImageFullscreenModal({
  open,
  imageSrc,
  onClose,
}: ImageFullscreenModalProps): JSX.Element {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth={false}
      fullWidth
      aria-label="Image preview full screen"
      sx={{
        "& .MuiDialog-paper": {
          maxWidth: "100vw",
          maxHeight: "100vh",
          m: 0,
          bgcolor: "rgba(0,0,0,0.9)",
        },
      }}
    >
      <Box
        sx={{
          position: "relative",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: "80vh",
          p: 2,
        }}
      >
        <IconButton
          aria-label="Close"
          size="small"
          onClick={onClose}
          color="warning"
          sx={{
            position: "absolute",
            right: 16,
            top: 16,
            zIndex: 1,
            border: "1px solid",
            borderColor: "warning.main",
            color: "warning.main",
          }}
        >
          <X size={24} aria-hidden />
        </IconButton>
        {imageSrc && (
          <Box
            component="img"
            src={imageSrc}
            alt="Full size"
            sx={{
              maxWidth: "90vw",
              maxHeight: "90vh",
              width: "auto",
              height: "auto",
              objectFit: "contain",
              display: "block",
            }}
          />
        )}
      </Box>
    </Dialog>
  );
}
