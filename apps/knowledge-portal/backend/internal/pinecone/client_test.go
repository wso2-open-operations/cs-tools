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

package pinecone

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestNewValidates(t *testing.T) {
	for _, tc := range []struct {
		host, key string
		ok        bool
	}{
		{"https://idx.example.com", "k", true},
		{"https://idx.example.com/", "k", true},
		{"idx.example.com", "k", false},
		{"ftp://idx.example.com", "k", false},
		{"https://idx.example.com", "", false},
	} {
		if _, err := New(tc.host, tc.key); (err == nil) != tc.ok {
			t.Errorf("New(%q,%q) err = %v, ok want %v", tc.host, tc.key, err, tc.ok)
		}
	}
}

func TestSimilarReturnsIDsAndScoresOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Api-Key") != "k" || r.URL.Path != "/query" {
			t.Errorf("bad request: %s key=%q", r.URL.Path, r.Header.Get("Api-Key"))
		}
		_, _ = w.Write([]byte(`{"matches":[{"id":"a","score":0.9,"metadata":{"title":"UNTRUSTED"}}]}`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k")
	got, err := c.Similar(context.Background(), "src", 5)
	if err != nil || len(got) != 1 || got[0] != (Match{ID: "a", Score: 0.9}) {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestSimilarDoesNotFollowRedirectsAndRejectsErrors(t *testing.T) {
	var hit atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit.Store(true) }))
	defer target.Close()
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redir.Close()
	c, _ := New(redir.URL, "k")
	if _, err := c.Similar(context.Background(), "src", 5); err == nil {
		t.Error("redirect treated as success")
	}
	if hit.Load() {
		t.Error("redirect followed (API key would have been replayed)")
	}
}
