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

package handler

import "regexp"

// base64ImagePayloadRe matches the `data:image/...;base64,<payload>` prefix
// plus payload of an inline image embedded directly in comment/description
// HTML — content authored before pasted images were stored as real,
// separate attachments never had its image extracted, so it stays as raw
// base64 image data inside the comment/case JSON itself. The base64 alphabet (`A-Za-z0-9+/=`) never includes `"`, so the
// payload capture naturally stops one character before the JSON string's
// closing quote without needing to look for it explicitly.
var base64ImagePayloadRe = regexp.MustCompile(`data:image/[a-zA-Z0-9.+-]+;base64,[A-Za-z0-9+/=]+`)

// redactRawBase64ImagesRaw is the placeholder src substituted in for a caller
// who may not see the real image. It deliberately keeps the "data:image/"
// prefix — apps/csm-portal/webapp's useResolvedInlineImageHtml (see that
// repo's own CLAUDE.md) already recognizes any `data:image/...` src as
// gated content when the caller lacks canDownloadAttachment and swaps in its
// own "You don't have permission to view this image" placeholder UI, so
// this value only has to be inert, not meaningful on its own.
const redactedInlineImageSrc = "data:image/png;base64,redacted"

// redactRawBase64Images replaces every embedded `data:image/...;base64,...`
// occurrence in raw JSON response bytes with redactedInlineImageSrc, for a
// caller who does not hold PermDownloadAttachment — so the real image bytes
// are never sent to them at all, closing the gap the frontend's own
// denyRawBase64 mitigation left open (that one only hides the image after
// it already reached the browser — see apps/csm-portal/webapp's CLAUDE.md
// for the full history of this gap).
//
// Operates directly on the raw response bytes rather than unmarshaling into
// a typed struct: comment/description HTML appears under different field
// names across endpoints (content, bodyHtml, description, ...) and this
// backend already treats these responses as raw passthrough (see this
// file's own "Response shape" conventions in CLAUDE.md) — a byte-level
// substitution keeps that convention rather than adding a typed reshape
// solely for this, and can't miss a field by name.
//
// An inline image referenced by id (a `.iix` reference or `/<attachment id>`)
// is untouched: its bytes are never in this response at all — the frontend
// fetches those separately, through a route itself already gated by
// PermDownloadAttachment.
func redactRawBase64Images(body []byte) []byte {
	return base64ImagePayloadRe.ReplaceAll(body, []byte(redactedInlineImageSrc))
}

// shouldRedactInlineImages reports whether a response about to be sent to a
// caller holding roles needs redactRawBase64Images run over it first. A nil
// access guard fails closed (redacts), never open — mirroring CaseHandler's
// own "nil fails that check closed" convention for PermViewSecurityCenter.
func shouldRedactInlineImages(access *AccessGuard, roles []string) bool {
	return access == nil || !access.Permits(PermDownloadAttachment, roles)
}
