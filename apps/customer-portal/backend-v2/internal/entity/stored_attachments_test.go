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

package entity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

func storedTestClient(t *testing.T, routes map[string]http.HandlerFunc) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	})
	for p, h := range routes {
		mux.HandleFunc(p, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
}

func TestStoredAttachments_BodiesAreRelayedVerbatimWithTheCallersHeaders(t *testing.T) {
	var gotBody, gotUser, gotCorr, gotAuth string
	record := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotUser, gotCorr, gotAuth = string(b), r.Header.Get("x-user-id-token"), r.Header.Get("X-CSM-Correlation-ID"), r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"relayed":true}`))
	}
	c := storedTestClient(t, map[string]http.HandlerFunc{
		"POST /attachments/download":            record,
		"POST /attachments/upload":              record,
		"POST /attachments/upload/{id}/confirm": record,
		"POST /attachments/delete":              func(w http.ResponseWriter, r *http.Request) { record(w, r); w.WriteHeader(http.StatusNoContent) },
	})
	ctx := WithCorrelationID(WithUserIDToken(context.Background(), "tok"), "corr-1")
	body := []byte(`{"type":"work_item","ids":["a"]}`)

	out, err := c.ResolveStoredAttachmentLinks(ctx, body)
	if err != nil || string(out) != `{"relayed":true}` || gotBody != string(body) || gotUser != "tok" || gotCorr != "corr-1" || !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("download: err=%v out=%s body=%q user=%q corr=%q auth=%q", err, out, gotBody, gotUser, gotCorr, gotAuth)
	}
	if _, err := c.InitiateStoredAttachmentUpload(ctx, body); err != nil {
		t.Errorf("upload: %v", err)
	}
	if _, err := c.ConfirmStoredAttachmentUpload(ctx, "abc", body); err != nil {
		t.Errorf("confirm: %v", err)
	}
	if err := c.DeleteStoredAttachment(ctx, body); err != nil {
		t.Errorf("delete: %v", err)
	}
}

func TestStoredAttachments_UpstreamStatusIsKept(t *testing.T) {
	c := storedTestClient(t, map[string]http.HandlerFunc{
		"POST /attachments/download": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"code":429,"message":"too many"}`, http.StatusTooManyRequests)
		},
	})
	_, err := c.ResolveStoredAttachmentLinks(context.Background(), []byte(`{}`))
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("err = %v, want an upstream 429", err)
	}
}

// TestStreamInlineImage_IsNotBuffered proves the body is handed over unread: the
// server writes one chunk, then waits for the client to confirm it already has
// that chunk before sending more. A client that buffered the whole response
// (as doBinary does) would deadlock here and fail on the timeout.
func TestStreamInlineImage_IsNotBuffered(t *testing.T) {
	got := make(chan struct{})
	c := storedTestClient(t, map[string]http.HandlerFunc{
		"GET /attachments/inline/{id}/content": func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-user-id-token") != "tok" {
				t.Errorf("x-user-id-token not forwarded")
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("first"))
			w.(http.Flusher).Flush()
			<-got
			_, _ = w.Write([]byte("second"))
		},
	})
	body, ct, size, err := c.StreamInlineImage(WithUserIDToken(context.Background(), "tok"), "img-1")
	if err != nil {
		t.Fatalf("StreamInlineImage: %v", err)
	}
	defer body.Close()
	if ct != "image/png" || size != -1 {
		t.Errorf("content type %q size %d, want image/png and -1 (chunked)", ct, size)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(body, buf); err != nil || string(buf) != "first" {
		t.Fatalf("first chunk = %q, %v", buf, err)
	}
	close(got)
	rest, _ := io.ReadAll(body)
	if string(rest) != "second" {
		t.Errorf("rest = %q", rest)
	}
}

func TestStreamInlineImage_MapsAnErrorStatus(t *testing.T) {
	c := storedTestClient(t, map[string]http.HandlerFunc{
		"GET /attachments/inline/{id}/content": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"code":404,"message":"attachment not found"}`, http.StatusNotFound)
		},
	})
	_, _, _, err := c.StreamInlineImage(context.Background(), "img-1")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("err = %v, want an upstream 404", err)
	}
}
