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
package plgsignups

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	c, err := NewClient(Config{BaseURL: apiSrv.URL, TokenURL: tokenSrv.URL, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestIngestPostsTheBatchWithABearerToken(t *testing.T) {
	var path, auth string
	var body map[string][]map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = w.Write([]byte(`{"accepted":1,"failed":0,"results":[{"status":"created"}]}`))
	})
	results, err := c.Ingest(context.Background(), []Registration{{CompanyID: "c-1", InitiatingProduct: "core"}})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/plg/registrations/ingest" || auth != "Bearer test-token" {
		t.Errorf("path = %q, auth = %q", path, auth)
	}
	if len(body["registrations"]) != 1 || body["registrations"][0]["companyId"] != "c-1" {
		t.Errorf("body = %v", body)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Errorf("results = %+v", results)
	}
}

func TestRecordFailurePostsTheFailurePath(t *testing.T) {
	var path string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	})
	if err := c.RecordFailure(context.Background(), Failure{EventID: "e", Payload: json.RawMessage(`{}`), Failure: "x"}); err != nil {
		t.Fatal(err)
	}
	if path != "/plg/ingest-failures" {
		t.Errorf("path = %q", path)
	}
}

func TestARefusalIsAnAPIErrorWithoutTheBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"owner@example.com"}`))
	})
	_, err := c.Ingest(context.Background(), nil)
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || err.Error() != "upstream returned 400" {
		t.Errorf("err = %v", err)
	}
}
