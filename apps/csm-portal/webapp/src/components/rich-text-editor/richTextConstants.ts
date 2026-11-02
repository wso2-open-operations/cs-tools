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

// Constants for the shared rich text editor. Relocated here from the support
// feature so the shared editor no longer imports from `@features/support`.

// The largest image that can be pasted or inserted is a runtime setting, not a
// constant: see `inlineImageMaxBytes` in `@config/attachmentLimitsConfig`.

// MIME types accepted for inline images (paste or toolbar upload), mirroring
// the backend's entity-service allow-list ("Unsupported inline image type"
// error) — SVG is deliberately excluded there since it can carry scripts.
export const ALLOWED_IMAGE_MIME_TYPES = [
  "image/png",
  "image/jpeg",
  "image/webp",
] as const;

export const ALLOWED_IMAGE_TYPES_LABEL = "PNG, JPEG, or WEBP";

/**
 * The most images one comment draft may hold when images are stored through an
 * `onImageUpload` handler (each one costs a server-side share link and a rate
 * limit). The base64 editors are capped by the server instead.
 */
export const MAX_COMMENT_IMAGES = 5;

export const MAX_COMMENT_IMAGES_MESSAGE = `A comment can have at most ${MAX_COMMENT_IMAGES} images.`;

export type RichTextBlockVariant =
  | "h1"
  | "h2"
  | "h3"
  | "h4"
  | "h5"
  | "h6"
  | "subtitle1"
  | "subtitle2"
  | "body1"
  | "body2"
  | "caption";

export const RICH_TEXT_BLOCK_TAGS: Array<{
  value: string;
  label: string;
  variant: RichTextBlockVariant;
}> = [
  { value: "h1", label: "Heading 1", variant: "h1" },
  { value: "h2", label: "Heading 2", variant: "h2" },
  { value: "h3", label: "Heading 3", variant: "h3" },
  { value: "h4", label: "Heading 4", variant: "h4" },
  { value: "h5", label: "Heading 5", variant: "h5" },
  { value: "h6", label: "Heading 6", variant: "h6" },
  { value: "subtitle1", label: "Subtitle 1", variant: "subtitle1" },
  { value: "subtitle2", label: "Subtitle 2", variant: "subtitle2" },
  { value: "body1", label: "Body 1", variant: "body1" },
  { value: "body2", label: "Body 2", variant: "body2" },
  { value: "caption", label: "Caption", variant: "caption" },
];
