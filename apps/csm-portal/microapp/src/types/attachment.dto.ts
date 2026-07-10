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

export type AttachmentReferenceType = "case" | "conversation" | "change_request" | "deployment";

export interface AttachmentCreatePayloadDto {
  referenceId: string;
  referenceType: AttachmentReferenceType;
  name: string;
  type: string;
  /** Base64 data URI, e.g. "data:image/png;base64,...". Max 10 MB decoded. */
  file: string;
}

export interface AttachmentDetailDto {
  id: string;
  sizeBytes?: number;
  downloadUrl?: string;
}

export interface AttachmentCreateResponseDto {
  message?: string;
  attachment: AttachmentDetailDto;
}
