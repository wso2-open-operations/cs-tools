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

/** An attachment upload in progress while a comment is sent. */
export interface CommentUploadProgress {
  name: string;
  percent: number;
}

/**
 * The {@link CommentUploadProgress} of a `usePostCsmCaseAttachment` mutation, or
 * `null` while it is not uploading.
 */
export function commentUploadProgress(upload: {
  isPending: boolean;
  uploadProgress: number | null;
  variables?: { name?: string; file?: File };
}): CommentUploadProgress | null {
  if (!upload.isPending || upload.uploadProgress === null) return null;
  return {
    name: upload.variables?.name || upload.variables?.file?.name || "attachment",
    percent: upload.uploadProgress,
  };
}
