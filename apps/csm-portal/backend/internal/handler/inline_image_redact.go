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

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// inlineImageRe matches an inline image data URI inside a (decoded) string:
// `data:image/<subtype>[;param...][;base64],<payload>`, case-insensitively,
// with any MIME parameters before the comma. The payload is the base64
// alphabet plus the URL-encoding characters a non-base64 data URI uses, and
// may be wrapped across lines (`\n` or `\r\n`, as e-mail clients wrap base64
// at 76 columns). It stops at a quote, `<`, `>` or other whitespace, which is
// where an HTML attribute value or the surrounding text resumes.
var inlineImageRe = regexp.MustCompile(`(?i)data:image/[^,"'<>\s]*,(?:[A-Za-z0-9+/=%._~-]|\r?\n)*`)

// redactedInlineImageSrc is the placeholder substituted for an inline image a
// caller may not see. It deliberately keeps the "data:image/" prefix: the
// webapp's inline-image resolver already treats any `data:image/...` src as
// gated content for a caller without the download permission and shows its
// own "no permission" placeholder, so this value only has to be inert.
const redactedInlineImageSrc = "data:image/png;base64,redacted"

// redactInlineImagesInString replaces every inline image data URI in s.
func redactInlineImagesInString(s string) string {
	return inlineImageRe.ReplaceAllString(s, redactedInlineImageSrc)
}

// redactRawBase64Images removes inline image data from a JSON response body.
//
// Content authored before (or without) attachment storage keeps a pasted
// image as a data URI inside the comment/description HTML itself, so every
// read of that text would hand the image to any caller, including one
// without PermDownloadAttachment. This walks every string value of the
// decoded JSON — field names differ across endpoints (content, bodyHtml,
// description, ...) and nesting varies, so no field is singled out — and
// redacts the data URIs found after JSON escapes (`\n`, `\/`, `+`, ...)
// are decoded, which a byte-level match on the encoded text would stop at.
//
// When nothing is redacted the original bytes are returned unchanged. When
// something is, the value is re-encoded (numbers kept verbatim via
// json.Number, HTML characters not escaped); object keys come out sorted,
// which no JSON consumer depends on. A body that is not a single JSON value
// falls back to a case-insensitive match over the raw bytes.
func redactRawBase64Images(body []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return inlineImageRe.ReplaceAll(body, []byte(redactedInlineImageSrc))
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return inlineImageRe.ReplaceAll(body, []byte(redactedInlineImageSrc))
	}
	changed := false
	v = redactInlineImagesInValue(v, &changed)
	if !changed {
		return body
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		// Unreachable for a value that was just decoded; never send the
		// unredacted body.
		return []byte(`null`)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

func redactInlineImagesInValue(v any, changed *bool) any {
	switch t := v.(type) {
	case string:
		r := redactInlineImagesInString(t)
		if r != t {
			*changed = true
		}
		return r
	case map[string]any:
		for k, child := range t {
			t[k] = redactInlineImagesInValue(child, changed)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = redactInlineImagesInValue(child, changed)
		}
		return t
	default:
		return v
	}
}

// shouldRedactInlineImages reports whether a response about to be sent to a
// caller holding roles needs redactRawBase64Images run over it first. A nil
// access guard fails closed (redacts), never open.
func shouldRedactInlineImages(access *AccessGuard, roles []string) bool {
	return access == nil || !access.Permits(PermDownloadAttachment, roles)
}

// RedactInlineImages wraps a route handler so that, for a caller without
// PermDownloadAttachment, every JSON response it writes has inline image data
// removed (redactRawBase64Images). It is applied once to every route in
// cmd/server/main.go, so a new read endpoint carrying rich text is covered
// without remembering a per-handler call. A caller holding the permission is
// passed straight through with no buffering; for everyone else the response
// is buffered, and only an application/json body is rewritten — binary and
// other content types are written back byte for byte.
func RedactInlineImages(access *AccessGuard, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserInfoFromContext(r.Context())
		if user != nil && !shouldRedactInlineImages(access, user.Roles) {
			next(w, r)
			return
		}
		bw := &bufferedResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next(bw, r)
		body := bw.buf.Bytes()
		if isJSONContentType(w.Header().Get("Content-Type")) && len(body) > 0 {
			body = redactRawBase64Images(body)
			w.Header().Del("Content-Length")
		}
		w.WriteHeader(bw.status)
		_, _ = w.Write(body)
	}
}

// bufferedResponseWriter holds a handler's status and body so
// RedactInlineImages can rewrite the body before anything is sent. Headers are
// shared with the real writer.
type bufferedResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	buf         bytes.Buffer
}

func (b *bufferedResponseWriter) WriteHeader(code int) {
	if b.wroteHeader {
		return
	}
	b.wroteHeader = true
	b.status = code
}

func (b *bufferedResponseWriter) Write(p []byte) (int, error) {
	b.wroteHeader = true
	return b.buf.Write(p)
}

func isJSONContentType(ct string) bool {
	if ct == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(ct)
	return err == nil && (mt == "application/json" || mt == "application/problem+json")
}
