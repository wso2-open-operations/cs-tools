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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

func newStoredTestClient(t *testing.T, h http.HandlerFunc) *CustomerEntityClient {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/", h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return NewCustomerEntityClient(CustomerEntityConfig{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})
}

func TestStoredAttachmentCallsSendTheDocumentedRequests(t *testing.T) {
	t.Parallel()
	var got []string
	c := newStoredTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.Path+" "+string(b))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})
	ctx := context.Background()
	if _, err := c.StoredAttachmentLinks(ctx, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.InitiateStoredAttachmentUpload(ctx, []byte(`{"b":2}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ConfirmStoredAttachmentUpload(ctx, "id-1", []byte(`{"c":3}`)); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteStoredAttachment(ctx, []byte(`{"d":4}`)); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`POST /attachments/download {"a":1}`, `POST /attachments/upload {"b":2}`,
		`POST /attachments/upload/id-1/confirm {"c":3}`, `POST /attachments/delete {"d":4}`,
	}
	if len(got) != len(want) {
		t.Fatalf("calls = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOpenInlineImageStreamsTheBodyAndForwardsIdentity(t *testing.T) {
	t.Parallel()
	var gotToken, gotCorrelation, gotPath string
	c := newStoredTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotToken, gotCorrelation, gotPath = r.Header.Get("x-user-id-token"), r.Header.Get("X-CSM-Correlation-ID"), r.URL.Path
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGDATA"))
	})
	ctx := WithCorrelationID(WithUserIDToken(context.Background(), "tok"), "corr-1")
	img, err := c.OpenInlineImage(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	defer img.Body.Close()
	b, _ := io.ReadAll(img.Body)
	if string(b) != "PNGDATA" || img.ContentType != "image/png" || img.Length != 7 {
		t.Errorf("got %q type=%q len=%d", b, img.ContentType, img.Length)
	}
	if gotToken != "tok" || gotCorrelation != "corr-1" || gotPath != "/attachments/inline/abc/content" {
		t.Errorf("token=%q correlation=%q path=%q", gotToken, gotCorrelation, gotPath)
	}
}

func TestOpenInlineImageReturnsTheUpstreamStatusAsAnError(t *testing.T) {
	t.Parallel()
	c := newStoredTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"attachment not found"}`, http.StatusNotFound)
	})
	_, err := c.OpenInlineImage(context.Background(), "abc")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("err = %v, want an *apierror.Error with status 404", err)
	}
}
