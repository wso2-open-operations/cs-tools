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

package service

import "regexp"

// inlineBase64ImageTagRe matches a whole <img ...> tag whose src is a raw
// `data:image/...;base64,<payload>` inline image -- content authored before
// pasted images were stored as attachments (or migrated with such content)
// never had its image extracted into a real, separately-stored attachment, so it stays as raw
// base64 image data directly inside the case description/comment HTML (see
// apps/csm-portal/backend's own CLAUDE.md, "Redacting raw base64 inline
// images", for the read-side version of this same gap). Unlike that read-side
// redaction, which only swaps out the base64 payload itself (keeping a
// recognizable `data:image/` src prefix for the webapp's own JS to detect),
// this replaces the ENTIRE <img> tag with plain text: the result here can
// end up directly in a Kafka event payload and, on the email fallback path,
// directly in a rendered notification email with no further processing step
// to hide a still-broken image behind a UI placeholder. Matches the src
// attribute quoted either way (`"..."` or `'...'`) -- a single-quoted image
// left unmatched would both survive into the event uncounted toward the
// byte-cap backstop's intent and risk that backstop later cutting straight
// through the middle of its base64 payload instead of the text after it.
// The base64 alphabet (`A-Za-z0-9+/=`) never includes `"` or `'`, so each
// payload capture naturally stops one character before its own attribute's
// closing quote.
var inlineBase64ImageTagRe = regexp.MustCompile(`<img\b[^>]*\bsrc\s*=\s*(?:"data:image/[a-zA-Z0-9.+-]+;base64,[A-Za-z0-9+/=]+"|'data:image/[a-zA-Z0-9.+-]+;base64,[A-Za-z0-9+/=]+')[^>]*>`)

// inlineImageUnavailablePlaceholder replaces each stripped <img> tag. Plain
// text, not a data: URI or any other value an email client might try to
// interpret as image data -- no fragment of the original base64 payload or
// its MIME type is ever carried into this value, so nothing encoded (and
// nothing that looks like a rendering bug) ever reaches a Chat card or,
// on the email fallback path below, a rendered notification email.
const inlineImageUnavailablePlaceholder = "[Image not available in this notification]"

// maxEventTextFieldBytes bounds a case description/comment's length after
// stripInlineBase64Images has already removed every embedded image -- a
// generous backstop (not a tuned-to-the-limit value) against any other,
// non-image cause of an oversized field (e.g. an extremely long pasted
// document), chosen to leave ample headroom under Event Hub's own ~1 MiB
// per-message ceiling for the rest of the event's JSON envelope (recipients,
// ids, every other field) and for several such events to safely coexist in
// a single Kafka batch.
const maxEventTextFieldBytes = 200 * 1024

// truncatedTextSuffix is appended when maxEventTextFieldBytes forces a cut --
// plain, readable text, not a silent mid-word truncation that could look like
// a rendering bug.
const truncatedTextSuffix = " […content truncated]"

// stripInlineBase64Images replaces every embedded `data:image/...;base64,...`
// <img> tag in html with inlineImageUnavailablePlaceholder, then -- as a
// backstop against any other unbounded-text cause -- truncates the result to
// maxEventTextFieldBytes. truncated is true if either step actually changed
// the content, so a caller building a case.created/case.comment_added event
// payload can flag the field as truncated: csm-notification-service uses that
// flag to re-fetch the full, untouched content via REST (outside Kafka's size
// limit entirely) when building a notification email, while a Chat card --
// which never rendered embedded images from this field to begin with --
// simply uses the returned, already-safe text as is.
func stripInlineBase64Images(html string) (stripped string, truncated bool) {
	withoutImages := inlineBase64ImageTagRe.ReplaceAllString(html, inlineImageUnavailablePlaceholder)
	truncated = withoutImages != html

	if len(withoutImages) <= maxEventTextFieldBytes {
		return withoutImages, truncated
	}

	cut := maxEventTextFieldBytes
	for cut > 0 && !isUTF8RuneBoundaryStart(withoutImages[cut]) {
		cut--
	}
	return withoutImages[:cut] + truncatedTextSuffix, true
}

// isUTF8RuneBoundaryStart reports whether b begins a UTF-8 rune (i.e. is not
// a continuation byte) -- used to make maxEventTextFieldBytes's cut never
// split a multi-byte rune in half.
func isUTF8RuneBoundaryStart(b byte) bool {
	return b&0xC0 != 0x80
}
