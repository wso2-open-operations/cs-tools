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

package cloudstatus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRecordDelivery_EscapesTheID keeps an id with path-significant
// characters inside its own path segment, as the sibling clients do.
func TestRecordDelivery_EscapesTheID(t *testing.T) {
	var gotRawPath, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"Bearer","expires_in":3600}`))
			return
		}
		gotRawPath, gotPath = r.URL.EscapedPath(), r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, err := NewClient(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RecordDelivery(context.Background(), "a/b?c", true, ""); err != nil {
		t.Fatal(err)
	}
	if gotRawPath != "/internal/cloud-status/a%2Fb%3Fc/delivery" {
		t.Fatalf("id must be escaped into one path segment, got raw %q (decoded %q)", gotRawPath, gotPath)
	}
}
