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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

func TestRedactRawBase64Images(t *testing.T) {
	t.Run("replaces an embedded base64 image payload, leaving the rest of the HTML intact", func(t *testing.T) {
		body := []byte(`{"comments":[{"content":"<p><span>before image</span><img src=\"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAXc\"></p>"}]}`)
		got := string(redactRawBase64Images(body))
		if strings.Contains(got, "iVBORw0KGgo") {
			t.Errorf("real base64 payload still present: %s", got)
		}
		if !strings.Contains(got, "before image") {
			t.Errorf("surrounding text was dropped: %s", got)
		}
		if !strings.Contains(got, redactedInlineImageSrc) {
			t.Errorf("redacted placeholder not present: %s", got)
		}
		// Redaction must produce valid JSON -- a broken response would fail
		// every caller regardless of role.
		if !json.Valid([]byte(got)) {
			t.Errorf("redacted body is not valid JSON: %s", got)
		}
	})

	t.Run("redacts every occurrence in a multi-comment response independently", func(t *testing.T) {
		body := []byte(`{"comments":[` +
			`{"content":"<img src=\"data:image/png;base64,AAAA\">"},` +
			`{"content":"<img src=\"data:image/jpeg;base64,BBBB\">"}` +
			`]}`)
		got := string(redactRawBase64Images(body))
		if strings.Contains(got, "AAAA") || strings.Contains(got, "BBBB") {
			t.Errorf("real base64 payloads still present: %s", got)
		}
		if strings.Count(got, redactedInlineImageSrc) != 2 {
			t.Errorf("expected both images redacted independently, got: %s", got)
		}
	})

	t.Run("leaves a response with no embedded image untouched", func(t *testing.T) {
		body := []byte(`{"comments":[{"content":"<p>plain text</p>"}]}`)
		got := redactRawBase64Images(body)
		if string(got) != string(body) {
			t.Errorf("body was modified when it had nothing to redact: %s", got)
		}
	})

	t.Run("a real .iix reference (not base64) is untouched -- it carries no bytes to redact", func(t *testing.T) {
		body := []byte(`{"content":"<img src=\"/inline/0123456789abcdef0123456789abcdef.iix\">"}`)
		got := redactRawBase64Images(body)
		if string(got) != string(body) {
			t.Errorf("a .iix reference should never be touched by this redaction: %s", got)
		}
	})
}

func TestShouldRedactInlineImages(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())

	t.Run("nil guard fails closed", func(t *testing.T) {
		if !shouldRedactInlineImages(nil, []string{"test-admin"}) {
			t.Error("a nil access guard must redact, never pass through")
		}
	})

	t.Run("a role without PermDownloadAttachment is redacted", func(t *testing.T) {
		for _, role := range []string{"test-viewer", "test-escalator", "test-usage-metrics-viewer"} {
			if !shouldRedactInlineImages(g, []string{role}) {
				t.Errorf("%s: expected redaction, got none", role)
			}
		}
	})

	t.Run("a role with PermDownloadAttachment is not redacted", func(t *testing.T) {
		for _, role := range []string{"test-attachment-downloader", "test-cs-engineer", "test-admin"} {
			if shouldRedactInlineImages(g, []string{role}) {
				t.Errorf("%s: expected no redaction, got redaction", role)
			}
		}
	})
}

func TestRedactRawBase64Images_Formats(t *testing.T) {
	const payload = "iVBORw0KGgoAAAANSUhEUgAAAXc"
	const bs = `\`
	cases := []struct {
		name string
		body string
	}{
		{"upper-case scheme and BASE64", `{"c":"<img src=\"DATA:IMAGE/PNG;BASE64,` + payload + `\">"}`},
		{"MIME parameters before base64", `{"c":"<img src=\"data:image/png;name=a.png;charset=x;base64,` + payload + `\">"}`},
		{"escaped slash in the payload", `{"c":"<img src=\"data:image\/png;base64,iVBORw0K\/GgoAAAANSUhEUgAAAXc\">"}`},
		// The JSON \u escapes are assembled from bs+"u..." so the source holds
		// the six-character escape, not the decoded character.
		{"unicode-escaped plus", `{"c":"<img src=\"data:image/png;base64,iVBORw0K` + bs + `u002bGgoAAAANSUhEUgAAAXc\">"}`},
		{"unicode-escaped data URI prefix", `{"c":"<img src=\"` + bs + `u0064ata:image` + bs + `u002fpng;base64,iVBORw0KGgoAAAANSUhEUgAAAXc\">"}`},
		{"76-column wrapped with LF", `{"c":"<img src=\"data:image/png;base64,iVBORw0KGgo\nAAAANSUhEUgAAAXc\">"}`},
		{"wrapped with CRLF", `{"c":"<img src=\"data:image/png;base64,iVBORw0KGgo\r\nAAAANSUhEUgAAAXc\">"}`},
		{"single-quoted attribute", `{"c":"<img src='data:image/jpeg;base64,` + payload + `'>"}`},
		{"url-encoded svg", `{"c":"<img src=\"data:image/svg+xml,%3Csvg%20xmlns%3D%22iVBORw0KGgoAAAANSUhEUgAAAXc%22%3E\">"}`},
		{"nested arrays and objects", `{"a":[{"b":{"c":["x","data:image/gif;base64,` + payload + `"]}}],"n":12345678901234567890}`},
		{"top-level string", `"data:image/png;base64,` + payload + `"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(redactRawBase64Images([]byte(tc.body)))
			for _, frag := range []string{"iVBORw0K", "AAAANSUhEUgAAAXc", "GgoAAAA"} {
				if strings.Contains(got, frag) {
					t.Fatalf("payload fragment %q survived: %s", frag, got)
				}
			}
			if !strings.Contains(got, redactedInlineImageSrc) {
				t.Fatalf("placeholder missing: %s", got)
			}
			if !json.Valid([]byte(got)) {
				t.Fatalf("not valid JSON: %s", got)
			}
		})
	}

	t.Run("large numbers and HTML survive a rewrite verbatim", func(t *testing.T) {
		got := string(redactRawBase64Images([]byte(`{"n":12345678901234567890,"c":"<p>a & b</p><img src=\"data:image/png;base64,AAAA\">"}`)))
		if !strings.Contains(got, `"n":12345678901234567890`) || !strings.Contains(got, `<p>a & b</p>`) {
			t.Fatalf("value changed beyond the redaction: %s", got)
		}
	})

	t.Run("text after the image is kept", func(t *testing.T) {
		got := string(redactRawBase64Images([]byte(`{"c":"see data:image/png;base64,AAAA and more"}`)))
		if !strings.Contains(got, " and more") {
			t.Fatalf("following text lost: %s", got)
		}
	})

	t.Run("non-JSON body falls back to a raw match", func(t *testing.T) {
		got := string(redactRawBase64Images([]byte(`<html><img src="Data:Image/png;base64,AAAA"></html>`)))
		if strings.Contains(got, "AAAA") {
			t.Fatalf("raw fallback missed: %s", got)
		}
	})
}

func TestRedactInlineImagesWrapper(t *testing.T) {
	g := NewAccessGuard(testAccessConfig())
	const jsonBody = `{"content":"<img src=\"data:image/png;base64,AAAA\">"}`
	jsonHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(jsonBody))
	}
	serve := func(h http.HandlerFunc, roles []string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "jane.doe@example.com", UserID: "u", Roles: roles}))
		w := httptest.NewRecorder()
		RedactInlineImages(g, h)(w, r)
		return w
	}

	t.Run("caller without download permission gets redacted JSON with the handler's status", func(t *testing.T) {
		w := serve(jsonHandler, []string{"test-viewer"})
		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d", w.Code)
		}
		if strings.Contains(w.Body.String(), "AAAA") || !strings.Contains(w.Body.String(), redactedInlineImageSrc) {
			t.Fatalf("not redacted: %s", w.Body.String())
		}
	})

	t.Run("caller with download permission gets the body untouched", func(t *testing.T) {
		w := serve(jsonHandler, []string{"test-attachment-downloader"})
		if w.Body.String() != jsonBody {
			t.Fatalf("holder body changed: %s", w.Body.String())
		}
	})

	t.Run("non-JSON content is passed through byte for byte", func(t *testing.T) {
		raw := []byte("binary data:image/png;base64,AAAA")
		w := serve(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(raw)
		}, []string{"test-viewer"})
		if w.Body.String() != string(raw) || w.Code != http.StatusOK {
			t.Fatalf("non-JSON body changed: %d %q", w.Code, w.Body.String())
		}
	})

	t.Run("error envelopes pass through", func(t *testing.T) {
		w := serve(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
		}, []string{"test-viewer"})
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d", w.Code)
		}
		assertErrorMessage(t, w, ErrMsgNotFound)
	})
}
