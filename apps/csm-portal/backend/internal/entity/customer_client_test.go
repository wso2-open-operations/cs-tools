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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestTokenFetchTimeout verifies that a stalled token endpoint fails requests
// within the configured timeout rather than blocking indefinitely.
func TestTokenFetchTimeout(t *testing.T) {
	t.Parallel()

	// Token server that stalls longer than tokenFetchTimeout but returns eventually
	// so httptest.Server.Close() can drain cleanly.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer tokenSrv.Close()

	// Use a short timeout so the test runs quickly.
	tokenFetchTimeout = 100 * time.Millisecond
	t.Cleanup(func() { tokenFetchTimeout = 10 * time.Second })

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      tokenSrv.URL,
		TokenURL:     tokenSrv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	start := time.Now()
	_, err := client.GetCase(context.Background(), "11111111-1111-1111-1111-111111111111")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from hung token server, got nil")
	}
	// Should fail well within 1 s (configured at 100 ms); allow 2 s for CI jitter.
	if elapsed > 2*time.Second {
		t.Errorf("token fetch took %v; expected <2s with 100ms timeout", elapsed)
	}
}

// TestSearchTagsSendsPostWithBody pins the entity-service call shape for tag
// search: a POST to /tags/search carrying the caller's body byte-for-byte. The
// old contract was a GET with `q`/`limit` query params and is gone; asserting
// on the raw bytes (rather than decoding through a struct) is what makes this
// test able to catch a silent key rename.
func TestSearchTagsSendsPostWithBody(t *testing.T) {
	t.Parallel()

	const reqBody = `{"filters":{"searchQuery":"micro"},"limit":20}`

	var gotMethod, gotPath, gotRawQuery string
	var gotBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/tags/search", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tags":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	if _, err := client.SearchTags(context.Background(), []byte(reqBody)); err != nil {
		t.Fatalf("SearchTags: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/tags/search" {
		t.Errorf("path = %q, want /tags/search", gotPath)
	}
	if gotRawQuery != "" {
		t.Errorf("query string = %q, want empty (the query moved into the body)", gotRawQuery)
	}
	if string(gotBody) != reqBody {
		t.Errorf("body = %s, want %s", gotBody, reqBody)
	}
}

// TestGetGroupSendsGetToGroupsID pins the entity-service call shape for the
// group page opened from an approval stage: GET /groups/{id} (a "group" id,
// not the team lookup) with no body and the id escaped into the path.
func TestGetGroupSendsGetToGroupsID(t *testing.T) {
	t.Parallel()

	const id = "22222222-2222-4222-8222-222222222222"
	var gotMethod, gotPath string
	var gotBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/groups/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `","name":"CAB Approval","members":[],"total":0}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	raw, err := client.GetGroup(context.Background(), id)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/groups/"+id {
		t.Errorf("path = %q, want /groups/%s", gotPath, id)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q, want none", gotBody)
	}
	if string(raw) == "" {
		t.Error("empty response")
	}
}

// TestGetCommentSendsGetToCommentsID pins the entity-service call shape
// CommentHandler relies on to learn a comment's author before deciding
// whether to allow an edit/delete (see comments.go's authorizeCommentActor):
// a GET to /comments/{id} with no body and the id escaped into the path.
func TestGetCommentSendsGetToCommentsID(t *testing.T) {
	t.Parallel()

	const id = "33333333-3333-4333-8333-333333333333"
	var gotMethod, gotPath string
	var gotBody []byte

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/comments/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + id + `","createdBy":{"email":"jane.doe@example.com"}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/token",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	})

	raw, err := client.GetComment(context.Background(), id)
	if err != nil {
		t.Fatalf("GetComment: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/comments/"+id {
		t.Errorf("path = %q, want /comments/%s", gotPath, id)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q, want none", gotBody)
	}
	if string(raw) == "" {
		t.Error("empty response")
	}
}

func TestNewCustomerEntityClientTimeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		cfg  time.Duration
		want time.Duration
	}{
		{"unset uses default", 0, 60 * time.Second},
		{"configured value applied", 90 * time.Second, 90 * time.Second},
	} {
		c := NewCustomerEntityClient(CustomerEntityConfig{Timeout: tc.cfg})
		if c.http.Timeout != tc.want {
			t.Errorf("%s: client timeout = %v, want %v", tc.name, c.http.Timeout, tc.want)
		}
	}
}

// TestAnnouncementRequestAttachmentClientCalls pins the five calls that carry
// the files sent with an announcement: method, path (ids escaped into it), what
// travels in the body or the query string, and that the content call returns the
// raw bytes with the upstream Content-Type.
func TestAnnouncementRequestAttachmentClientCalls(t *testing.T) {
	t.Parallel()

	const (
		id    = "11111111-1111-4111-8111-111111111111"
		att   = "22222222-2222-4222-8222-222222222222"
		caseI = "33333333-3333-4333-8333-333333333333"
	)
	type seen struct {
		method, path, query string
		body                []byte
	}
	var got seen

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/announcement-requests/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = seen{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body}
		if strings.HasSuffix(r.URL.Path, "/content") {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-bytes"))
			return
		}
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewCustomerEntityClient(CustomerEntityConfig{
		BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s",
	})
	ctx := context.Background()

	if _, err := client.CreateAnnouncementRequestAttachment(ctx, id, []byte(`{"name":"a.pdf"}`)); err != nil {
		t.Fatalf("CreateAnnouncementRequestAttachment: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/announcement-requests/"+id+"/attachments" || string(got.body) != `{"name":"a.pdf"}` {
		t.Errorf("create: %+v", got)
	}

	if _, err := client.ListAnnouncementRequestAttachments(ctx, id); err != nil {
		t.Fatalf("ListAnnouncementRequestAttachments: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/announcement-requests/"+id+"/attachments" || len(got.body) != 0 {
		t.Errorf("list: %+v", got)
	}

	content, ct, err := client.GetAnnouncementRequestAttachmentContent(ctx, id, att)
	if err != nil {
		t.Fatalf("GetAnnouncementRequestAttachmentContent: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/announcement-requests/"+id+"/attachments/"+att+"/content" {
		t.Errorf("content: %+v", got)
	}
	if string(content) != "%PDF-bytes" || ct != "application/pdf" {
		t.Errorf("content = %q (%s), want the raw bytes and the upstream type", content, ct)
	}

	if _, err := client.DeleteAnnouncementRequestAttachment(ctx, id, att, "user 1&x=y"); err != nil {
		t.Fatalf("DeleteAnnouncementRequestAttachment: %v", err)
	}
	if got.method != http.MethodDelete || got.path != "/announcement-requests/"+id+"/attachments/"+att {
		t.Errorf("delete: %+v", got)
	}
	// The acting user is one query parameter and nothing a caller supplies can add another.
	if got.query != "actorId=user+1%26x%3Dy" {
		t.Errorf("delete query = %q, want the actor URL-encoded as a single parameter", got.query)
	}

	if _, err := client.AttachAnnouncementRequestAttachmentsToCase(ctx, id, caseI, []byte(`{"actorId":"u"}`)); err != nil {
		t.Fatalf("AttachAnnouncementRequestAttachmentsToCase: %v", err)
	}
	if got.method != http.MethodPost || got.path != "/announcement-requests/"+id+"/cases/"+caseI+"/attachments" || string(got.body) != `{"actorId":"u"}` {
		t.Errorf("attach to case: %+v", got)
	}
}
