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

// Port of v3's src/lib/surfaces.ts. Shared "acrylic" surface treatment
// (translucent background + backdrop blur) for card-like containers,
// matching Oxygen UI's Acrylic Purple material. Components use plain `Box`
// rather than `Card`/`Paper` (for full control over the existing bespoke
// layout), so this is reproduced explicitly and spread into each
// component's `sx`.
export const acrylicSurfaceSx = {
  bgcolor: "var(--sla-card)",
  backdropFilter: "var(--sla-blur)",
  WebkitBackdropFilter: "var(--sla-blur)",
} as const;
