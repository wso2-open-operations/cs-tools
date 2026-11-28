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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package entity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func TestPutStoredAttachmentChunk_SendsTheBytesWithTheCallersToken(t *testing.T) {
	var method, path, query, token, ctype, corr, body string
	var length int64
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
		token, ctype, corr, length = r.Header.Get("x-user-id-token"), r.Header.Get("Content-Type"), r.Header.Get("X-CSM-Correlation-ID"), r.ContentLength
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := WithCorrelationID(WithUserIDToken(context.Background(), "user-jwt"), "corr-1")
	// A reader that is not a strings.Reader: the length has to come from the caller.
	err := client.PutStoredAttachmentChunk(ctx, "abc", "work_item", 3, 5, io.MultiReader(strings.NewReader("hel"), strings.NewReader("lo")))
	if err != nil {
		t.Fatalf("PutStoredAttachmentChunk: %v", err)
	}
	if method != http.MethodPut || path != "/attachments/upload/abc/chunks/3" || query != "type=work_item" {
		t.Errorf("request = %s %s?%s", method, path, query)
	}
	if body != "hello" || length != 5 || ctype != "application/octet-stream" {
		t.Errorf("body = %q, length = %d, type = %q", body, length, ctype)
	}
	if token != "user-jwt" || corr != "corr-1" {
		t.Errorf("token = %q, correlation = %q: the caller's identity must reach entity-service", token, corr)
	}
}

func TestPutStoredAttachmentChunk_FailureKeepsTheStatusAndBody(t *testing.T) {
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"This case is closed."}`))
	})
	err := client.PutStoredAttachmentChunk(context.Background(), "abc", "work_item", 0, 1, strings.NewReader("x"))
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 409 || !strings.Contains(apiErr.Body, "closed") {
		t.Errorf("err = %v, want a 409 apierror carrying the body", err)
	}
}

func TestDownloadStoredAttachment_StreamsAndPassesTheRange(t *testing.T) {
	var path, query, rng string
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, query, rng = r.URL.Path, r.URL.RawQuery, r.Header.Get("Range")
		w.Header().Set("Content-Range", "bytes 2-4/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("234"))
	})
	f, err := client.DownloadStoredAttachment(context.Background(), "work_item", "abc", "bytes=2-4")
	if err != nil {
		t.Fatalf("DownloadStoredAttachment: %v", err)
	}
	defer f.Body.Close()
	if b, _ := io.ReadAll(f.Body); string(b) != "234" || f.StatusCode != 206 || f.Header.Get("Content-Range") != "bytes 2-4/10" {
		t.Errorf("body = %q, status = %d, headers = %v", b, f.StatusCode, f.Header)
	}
	if path != "/attachments/download/abc/content" || query != "type=work_item" || rng != "bytes=2-4" {
		t.Errorf("request = %s?%s Range=%q", path, query, rng)
	}
}

func TestDownloadStoredAttachment_Refusals(t *testing.T) {
	for _, tc := range []struct {
		status int
		check  func(error) bool
	}{
		{http.StatusNotFound, func(err error) bool {
			var e *apierror.Error
			return errors.As(err, &e) && e.StatusCode == 404
		}},
		{http.StatusGone, func(err error) bool {
			var e *apierror.Error
			return errors.As(err, &e) && e.StatusCode == 410
		}},
		{http.StatusRequestedRangeNotSatisfiable, func(err error) bool {
			var e *RangeNotSatisfiableError
			return errors.As(err, &e) && e.ContentRange == "bytes */10"
		}},
	} {
		client := newHealthDetailTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Range", "bytes */10")
			w.WriteHeader(tc.status)
		})
		f, err := client.DownloadStoredAttachment(context.Background(), "work_item", "abc", "")
		if f != nil || !tc.check(err) {
			t.Errorf("status %d: file = %v, err = %v", tc.status, f, err)
		}
	}
}

func TestInlineImageContent_RequestsTheContentRoute(t *testing.T) {
	var path string
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	})
	f, err := client.InlineImageContent(context.Background(), "img-1")
	if err != nil {
		t.Fatalf("InlineImageContent: %v", err)
	}
	defer f.Body.Close()
	if b, _ := io.ReadAll(f.Body); string(b) != "png" || path != "/attachments/inline/img-1/content" {
		t.Errorf("body = %q, path = %q", b, path)
	}
}

func TestListStoredAttachmentChunks_RequestsThePathAndType(t *testing.T) {
	var path, query string
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"chunkBytes":8388608,"chunks":[]}`))
	})
	out, err := client.ListStoredAttachmentChunks(context.Background(), "abc", "comment")
	if err != nil || path != "/attachments/upload/abc/chunks" || query != "type=comment" || !strings.Contains(string(out), "chunkBytes") {
		t.Errorf("out = %s, err = %v, request = %s?%s", out, err, path, query)
	}
}

// The file client has no total timeout: a 100 MiB download on a slow link
// outlasts any fixed one. The ordinary client keeps its own.
func TestFileClientHasNoTotalTimeout(t *testing.T) {
	c := NewCustomerEntityClient(CustomerEntityConfig{BaseURL: "http://x", TokenURL: "http://x/token", ClientID: "c", ClientSecret: "s"})
	if c.fileHTTP.Timeout != 0 {
		t.Errorf("file client timeout = %v, want none (each call is bounded by its context)", c.fileHTTP.Timeout)
	}
	if c.http.Timeout != defaultCustomerEntityTimeout {
		t.Errorf("ordinary client timeout = %v, want %v", c.http.Timeout, defaultCustomerEntityTimeout)
	}
}

func TestStoredAttachmentLimits_RequestsThePathAndReturnsRawJSON(t *testing.T) {
	var method, path string
	client := newHealthDetailTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_, _ = w.Write([]byte(`{"maxFileBytes":1073741824,"maxInlineImagesTotalBytes":20971520}`))
	})
	out, err := client.StoredAttachmentLimits(context.Background())
	if err != nil || method != http.MethodGet || path != "/attachments/limits" || !strings.Contains(string(out), "maxFileBytes") {
		t.Errorf("out = %s, err = %v, request = %s %s", out, err, method, path)
	}
}
