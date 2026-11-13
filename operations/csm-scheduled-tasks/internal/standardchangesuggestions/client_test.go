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

package standardchangesuggestions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)
	apiSrv := httptest.NewServer(handler)
	t.Cleanup(apiSrv.Close)
	c, err := NewClient(ClientConfig{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClient_RefreshPostsThePage(t *testing.T) {
	var path, auth string
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"runId":"r1","customers":2,"changeRequests":40,"suggestions":3,"skipped":1,"more":true,"nextCursor":"a2"}`))
	})
	resp, err := c.Refresh(context.Background(), Request{RunID: "r1", Cursor: "a1", Limit: PageSize})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/standard-change-suggestions/refresh" || auth != "Bearer test-token" {
		t.Errorf("path=%q auth=%q", path, auth)
	}
	if body["runId"] != "r1" || body["cursor"] != "a1" || body["limit"] != float64(PageSize) || len(body) != 3 {
		t.Errorf("body = %v", body)
	}
	if resp.RunID != "r1" || resp.Customers != 2 || resp.Skipped != 1 || !resp.More || resp.NextCursor != "a2" {
		t.Errorf("resp = %+v", resp)
	}
}

// The first page names no run: entity-service decodes with DisallowUnknownFields,
// and runId and cursor are omitempty, so it sends only the limit.
func TestClient_FirstPageSendsOnlyTheLimit(t *testing.T) {
	var raw []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"runId":"r1","more":false}`))
	})
	if _, err := c.Refresh(context.Background(), Request{Limit: PageSize}); err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"limit":50}` {
		t.Errorf("body = %s", raw)
	}
}

func TestClient_RefreshNon2xxIsAnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"this resource is only available to a service credential, not a user"}`))
	})
	_, err := c.Refresh(context.Background(), Request{})
	var ae *apierror.Error
	if !errors.As(err, &ae) || ae.StatusCode != http.StatusForbidden || !strings.Contains(ae.Body, "service credential") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewClient_RefusesPlainHTTP(t *testing.T) {
	if _, err := NewClient(ClientConfig{BaseURL: "http://entity.example.com", TokenURL: "https://t.example.com"}); err == nil {
		t.Error("a plain-http base URL was accepted")
	}
}

// A misbehaving upstream or proxy cannot make the client hold an unbounded
// body: a success body is refused past maxResponseBytes, and a refusal keeps
// at most maxErrorBodyBytes of its body.
func TestClient_BoundsTheBody(t *testing.T) {
	t.Run("a success body past the limit is an error", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"runId":"r1","nextCursor":"`))
			_, _ = w.Write([]byte(strings.Repeat("a", maxResponseBytes)))
			_, _ = w.Write([]byte(`"}`))
		})
		_, err := c.Refresh(context.Background(), Request{})
		if err == nil || !strings.Contains(err.Error(), "larger than") {
			t.Fatalf("err = %v, want the body refused", err)
		}
	})
	t.Run("a body at the limit is read", func(t *testing.T) {
		body := `{"runId":"r1","nextCursor":"`
		body += strings.Repeat("a", maxResponseBytes-len(body)-2) + `"}`
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		resp, err := c.Refresh(context.Background(), Request{})
		if err != nil || resp.RunID != "r1" {
			t.Fatalf("resp = %+v, %v", resp.RunID, err)
		}
	})
	t.Run("a refusal keeps only the start of its body", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(strings.Repeat("<html>", 10000)))
		})
		_, err := c.Refresh(context.Background(), Request{})
		var apiErr *apierror.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadGateway {
			t.Fatalf("err = %v, want the 502", err)
		}
		if len(apiErr.Body) != maxErrorBodyBytes {
			t.Errorf("kept %d bytes of the body, want %d", len(apiErr.Body), maxErrorBodyBytes)
		}
	})
}
