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

package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

type captured struct {
	method, path, query string
	body                map[string]any
}

// ledgerServer serves a token endpoint and answers every other request
// with status and respBody, capturing what it received.
func ledgerServer(t *testing.T, status int, respBody string, got *captured) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
			return
		}
		got.method, got.path, got.query = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &got.body); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing bearer token")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
}

func client(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(Config{BaseURL: srv.URL + "/", TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAttempt_RequestShapeAndDecode(t *testing.T) {
	var got captured
	srv := ledgerServer(t, 200, `{"allowed":true,"run":{"id":"00000000-0000-0000-0000-000000000001","taskName":"t","attemptCount":2}}`, &got)
	defer srv.Close()

	period := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	claim, err := client(t, srv).Attempt(context.Background(), "t", period, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/scheduled-tasks/attempts" {
		t.Errorf("got %s %s", got.method, got.path)
	}
	if got.body["taskName"] != "t" || got.body["periodKey"] != "2026-10-02T03:00:00Z" || got.body["staleClaimAfterSeconds"] != float64(600) {
		t.Errorf("unexpected request body %v", got.body)
	}
	if !claim.Allowed || claim.Run.AttemptCount != 2 {
		t.Errorf("unexpected claim %+v", claim)
	}
}

func TestCompleteAndFail_RequestShape(t *testing.T) {
	var got captured
	srv := ledgerServer(t, 200, `{}`, &got)
	defer srv.Close()
	c := client(t, srv)

	if err := c.Complete(context.Background(), "a/b", 3); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch || got.path != "/scheduled-tasks/attempts/a%2Fb" ||
		got.body["status"] != "succeeded" || got.body["attemptCount"] != float64(3) {
		t.Errorf("Complete sent %s %s %v", got.method, got.path, got.body)
	}

	next := time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)
	if err := c.Fail(context.Background(), "id1", 3, "boom", next); err != nil {
		t.Fatal(err)
	}
	if got.body["status"] != "failed" || got.body["error"] != "boom" || got.body["nextRetryOn"] != "2026-10-02T04:00:00Z" {
		t.Errorf("Fail sent %v", got.body)
	}
}

func TestDeleteResolvedBefore(t *testing.T) {
	var got captured
	srv := ledgerServer(t, 200, `{"deletedCount":4}`, &got)
	defer srv.Close()

	n, err := client(t, srv).DeleteResolvedBefore(context.Background(), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || n != 4 {
		t.Fatalf("got %d, %v", n, err)
	}
	if got.method != http.MethodDelete || !strings.Contains(got.query, "resolvedBefore=2026-09-01T00%3A00%3A00Z") {
		t.Errorf("got %s ?%s", got.method, got.query)
	}
}

func TestErrorsPropagateAsAPIErrorWithoutTheBody(t *testing.T) {
	var got captured
	srv := ledgerServer(t, 503, `secret upstream detail`, &got)
	defer srv.Close()
	c := client(t, srv)

	calls := map[string]func() error{
		"Attempt":  func() error { _, err := c.Attempt(context.Background(), "t", time.Now(), 0); return err },
		"Complete": func() error { return c.Complete(context.Background(), "id", 1) },
		"Fail":     func() error { return c.Fail(context.Background(), "id", 1, "x", time.Now()) },
		"Delete":   func() error { _, err := c.DeleteResolvedBefore(context.Background(), time.Now()); return err },
	}
	for name, call := range calls {
		err := call()
		var apiErr *apierror.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 503 {
			t.Errorf("%s: want *apierror.Error 503, got %v", name, err)
			continue
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: the response body must not be in the error text: %v", name, err)
		}
	}
}

func TestTransportFailurePropagates(t *testing.T) {
	var got captured
	srv := ledgerServer(t, 200, `{}`, &got)
	c := client(t, srv)
	srv.Close() // nothing listening any more

	if err := c.Complete(context.Background(), "id", 1); err == nil {
		t.Fatal("a transport failure must be returned")
	}
}

func TestMalformedResponseIsAnError(t *testing.T) {
	var got captured
	srv := ledgerServer(t, 200, `not json`, &got)
	defer srv.Close()

	if _, err := client(t, srv).Attempt(context.Background(), "t", time.Now(), 0); err == nil {
		t.Fatal("an undecodable claim response must be an error")
	}
}
