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

/** Data attribute every KB article quick-preview eye button carries, and
 * the selector `useCloseOnOutsideClick` excludes so a click on one of them
 * is left to its own onClick handler -- which already decides the next
 * preview state (open a different row, or toggle the current one closed)
 * -- rather than being raced by the drawer's own click-outside-closes
 * listener. Mirrors csm-cases' `quickPreviewEye.ts`. */
export const KB_QUICK_PREVIEW_EYE_ATTRIBUTE = "data-kb-quick-preview-eye";
export const KB_QUICK_PREVIEW_EYE_SELECTOR = `[${KB_QUICK_PREVIEW_EYE_ATTRIBUTE}]`;
