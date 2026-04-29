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

import { useState } from "react";
import type { Attachment } from "@features/cases/types/case.model";

export function useAttachmentPreview() {
  const [previewAttachment, setPreviewAttachment] = useState<Attachment | null>(null);
  const [previewSrc, setPreviewSrc] = useState<string | null>(null);

  const open = async (attachment: Attachment, blob: Blob) => {
    setPreviewAttachment(attachment);
    setPreviewSrc(URL.createObjectURL(blob));
  };

  const close = () => {
    setPreviewAttachment(null);
    setPreviewSrc(null);
  };

  return { previewAttachment, previewSrc, open, close };
}
