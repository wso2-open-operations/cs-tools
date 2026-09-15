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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// fakeSNServer is a minimal in-memory stand-in for a ServiceNow instance's
// oauth_token.do and /api/now/table/{table} endpoints, used to test snClient
// without any real SN credentials (none are available in this sandbox --
// see the accompanying return notes on what's unverified).
type fakeSNServer struct {
	tokenCalls int
	rowsByType map[string][]map[string]string // table -> all rows, in sys_id order
	pageSize   int
}

func newFakeSNServer(rowsByType map[string][]map[string]string) *fakeSNServer {
	return &fakeSNServer{rowsByType: rowsByType, pageSize: tableAPIPageSize}
}

func (f *fakeSNServer) Do(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/oauth_token.do") {
		f.tokenCalls++
		body, _ := json.Marshal(tokenResponse{AccessToken: "fake-token", ExpiresIn: 1800})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	}

	if req.Header.Get("Authorization") != "Bearer fake-token" {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"error":"unauthorized"}`))}, nil
	}

	// Parse /api/now/table/{table}
	parts := strings.Split(req.URL.Path, "/")
	table := parts[len(parts)-1]
	all := f.rowsByType[table]

	q := req.URL.Query()
	afterID := ""
	if query := q.Get("sysparm_query"); strings.Contains(query, "sys_id>") {
		afterID = query[strings.Index(query, "sys_id>")+len("sys_id>"):]
	}
	limit, _ := strconv.Atoi(q.Get("sysparm_limit"))
	if limit == 0 {
		limit = f.pageSize
	}

	startIdx := 0
	if afterID != "" {
		for i, r := range all {
			if r["sys_id"] == afterID {
				startIdx = i + 1
				break
			}
		}
	}
	endIdx := startIdx + limit
	if endIdx > len(all) {
		endIdx = len(all)
	}
	var page []map[string]string
	if startIdx < len(all) {
		page = all[startIdx:endIdx]
	}

	respBody, _ := json.Marshal(tableQueryResponse{Result: page})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(respBody)))}, nil
}

func sysIDForTest(n int) string {
	return fmt.Sprintf("%032x", n)
}

func TestSNClient_Authenticate_CachesToken(t *testing.T) {
	fake := newFakeSNServer(nil)
	c := newSNClient("https://wso2sndev.example.invalid", "client-id", "secret", fake)

	tok1, err := c.authenticate(context.Background())
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	tok2, err := c.authenticate(context.Background())
	if err != nil {
		t.Fatalf("authenticate (2nd call): %v", err)
	}
	if tok1 != tok2 {
		t.Errorf("expected the cached token to be reused, got %q then %q", tok1, tok2)
	}
	if fake.tokenCalls != 1 {
		t.Errorf("expected exactly 1 token request, got %d", fake.tokenCalls)
	}
}

func TestSNClient_FetchAllRows_PaginatesByKeyset(t *testing.T) {
	// Seed more rows than one page so pagination must kick in, and confirm
	// sysparm_offset is never used -- only ORDERBYsys_id^sys_id>{lastID}.
	const total = tableAPIPageSize + 37
	rows := make([]map[string]string, total)
	for i := 0; i < total; i++ {
		rows[i] = map[string]string{"sys_id": sysIDForTest(i + 1), "title": fmt.Sprintf("kb-%d", i)}
	}
	fake := newFakeSNServer(map[string][]map[string]string{"kb_knowledge_base": rows})
	c := newSNClient("https://wso2sndev.example.invalid", "id", "secret", fake)

	got, err := c.fetchAllRows(context.Background(), "kb_knowledge_base", []string{"sys_id", "title"})
	if err != nil {
		t.Fatalf("fetchAllRows: %v", err)
	}
	if len(got) != total {
		t.Fatalf("got %d rows, want %d", len(got), total)
	}
	for i, r := range got {
		if r["sys_id"] != rows[i]["sys_id"] {
			t.Fatalf("row %d: sys_id = %q, want %q (pagination order broken)", i, r["sys_id"], rows[i]["sys_id"])
		}
	}
}

func TestSNClient_FetchPage_NeverRequestsDisplayValue(t *testing.T) {
	var capturedQuery url.Values
	capture := doerFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/oauth_token.do") {
			body, _ := json.Marshal(tokenResponse{AccessToken: "fake-token", ExpiresIn: 1800})
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
		}
		capturedQuery = req.URL.Query()
		body, _ := json.Marshal(tableQueryResponse{Result: nil})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})
	c := newSNClient("https://wso2sndev.example.invalid", "id", "secret", capture)

	if _, err := c.fetchAllRows(context.Background(), "kb_knowledge", kbKnowledgeFields); err != nil {
		t.Fatalf("fetchAllRows: %v", err)
	}
	if capturedQuery.Has("sysparm_display_value") {
		t.Fatalf("sysparm_display_value must never be requested, got query %v", capturedQuery)
	}
	if capturedQuery.Has("sysparm_offset") {
		t.Fatalf("sysparm_offset must never be used (keyset pagination only), got query %v", capturedQuery)
	}
}

// doerFunc adapts a plain function to the httpDoer interface.
type doerFunc func(req *http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }
